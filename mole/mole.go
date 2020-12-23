package mole

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/davrodpin/mole/alias"
	"github.com/davrodpin/mole/fsutils"
	"github.com/davrodpin/mole/rpc"
	"github.com/davrodpin/mole/tunnel"

	"github.com/awnumar/memguard"
	"github.com/gofrs/uuid"
	"github.com/sevlyar/go-daemon"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh/terminal"
)

// cli keeps a reference to the latest Client object created.
// This is mostly needed to introspect client states during runtime (e.g. a
// remote procedure call that needs to check certain configuration fields)
var cli *Client

func init() {
	rpc.Register("show-instance", ShowRpc)
}

type Configuration struct {
	Id                string                 `json:"id"`
	TunnelType        string                 `json:"tunnel-type"`
	Verbose           bool                   `json:"verbose"`
	Insecure          bool                   `json:"insecure"`
	Detach            bool                   `json:"detach"`
	Source            alias.AddressInputList `json:"source"`
	Destination       alias.AddressInputList `json:"destination"`
	Server            alias.AddressInput     `json:"server"`
	Key               string                 `json:"key"`
	KeepAliveInterval time.Duration          `json:"keep-alive-interval"`
	ConnectionRetries int                    `json:"connection-retries"`
	WaitAndRetry      time.Duration          `json:"wait-and-retry"`
	SshAgent          string                 `json:"ssh-agent"`
	Timeout           time.Duration          `json:"timeout"`
	SshConfig         string                 `json:"ssh-config"`
	Rpc               bool                   `json:"rpc"`
	RpcAddress        string                 `json:"rpc-address"`
}

// ParseAlias translates a Configuration object to an Alias object.
func (c Configuration) ParseAlias(name string) *alias.Alias {
	return &alias.Alias{
		Name:              name,
		TunnelType:        c.TunnelType,
		Verbose:           c.Verbose,
		Insecure:          c.Insecure,
		Detach:            c.Detach,
		Source:            c.Source.List(),
		Destination:       c.Destination.List(),
		Server:            c.Server.String(),
		Key:               c.Key,
		KeepAliveInterval: c.KeepAliveInterval.String(),
		ConnectionRetries: c.ConnectionRetries,
		WaitAndRetry:      c.WaitAndRetry.String(),
		SshAgent:          c.SshAgent,
		Timeout:           c.Timeout.String(),
		SshConfig:         c.SshConfig,
		Rpc:               c.Rpc,
		RpcAddress:        c.RpcAddress,
	}
}

// Client manages the overall state of the application based on its configuration.
type Client struct {
	Conf *Configuration
}

// New initializes a new mole's client.
func New(conf *Configuration) *Client {
	cli = &Client{Conf: conf}

	return cli
}

// Start kicks off mole's client, establishing the tunnel and its channels
// based on the client configuration attributes.
func (c *Client) Start() error {
	// memguard is used to securely keep sensitive information in memory.
	// This call makes sure all data will be destroy when the program exits.
	defer memguard.Purge()

	if c.Conf.Detach {
		var err error

		if c.Conf.Id == "" {
			u, err := uuid.NewV4()
			if err != nil {
				return fmt.Errorf("could not auto generate app instance id: %v", err)
			}
			c.Conf.Id = u.String()[:8]
		}

		ic, err := NewDetachedInstance(c.Conf.Id)
		if err != nil {
			log.WithError(err).Errorf("error while creating directory to store mole instance related files")
			return err
		}

		err = startDaemonProcess(ic)
		if err != nil {
			log.WithFields(log.Fields{
				"id": c.Conf.Id,
			}).WithError(err).Error("error starting ssh tunnel")

			return err
		}
	}

	if c.Conf.Id == "" {
		c.Conf.Id = strconv.Itoa(os.Getpid())
	}

	if c.Conf.Verbose {
		log.SetLevel(log.DebugLevel)
	}

	d, err := fsutils.CreateInstanceDir(c.Conf.Id)
	if err != nil {
		log.WithFields(log.Fields{
			"id": c.Conf.Id,
		}).WithError(err).Error("error creating directory for mole instance")

		return err
	}

	if c.Conf.Rpc {
		addr, err := rpc.Start(c.Conf.RpcAddress)
		if err != nil {
			return err
		}

		rd := filepath.Join(d, "rpc")

		err = ioutil.WriteFile(rd, []byte(addr.String()), 0644)
		if err != nil {
			log.WithFields(log.Fields{
				"id": c.Conf.Id,
			}).WithError(err).Error("error creating file with rpc address")

			return err
		}

		c.Conf.RpcAddress = addr.String()

		log.Infof("rpc server address saved on %s", rd)
	}

	s, err := tunnel.NewServer(c.Conf.Server.User, c.Conf.Server.Address(), c.Conf.Key, c.Conf.SshAgent, c.Conf.SshConfig)
	if err != nil {
		log.Errorf("error processing server options: %v\n", err)
		return err
	}

	s.Insecure = c.Conf.Insecure
	s.Timeout = c.Conf.Timeout

	err = s.Key.HandlePassphrase(func() ([]byte, error) {
		fmt.Printf("The key provided is secured by a password. Please provide it below:\n")
		fmt.Printf("Password: ")
		p, err := terminal.ReadPassword(int(syscall.Stdin))
		fmt.Printf("\n")
		return p, err
	})

	if err != nil {
		log.WithError(err).Error("error setting up password handling function")
		return err
	}

	log.Debugf("server: %s", s)

	source := make([]string, len(c.Conf.Source))
	for i, r := range c.Conf.Source {
		source[i] = r.String()
	}

	destination := make([]string, len(c.Conf.Destination))
	for i, r := range c.Conf.Destination {
		if r.Port == "" {
			log.WithError(err).Errorf("missing port in destination address: %s", r.String())
			return err
		}

		destination[i] = r.String()
	}

	t, err := tunnel.New(c.Conf.TunnelType, s, source, destination, c.Conf.SshConfig)
	if err != nil {
		log.Error(err)
		return err
	}

	//TODO need to find a way to require the attributes below to be always set
	// since they are not optional (functionality will break if they are not
	// set and CLI parsing is the one setting the default values).
	// That could be done by make them required in the constructor's signature or
	// by creating a configuration struct for a tunnel object.
	t.ConnectionRetries = c.Conf.ConnectionRetries
	t.WaitAndRetry = c.Conf.WaitAndRetry
	t.KeepAliveInterval = c.Conf.KeepAliveInterval

	if err = t.Start(); err != nil {
		log.WithFields(log.Fields{
			"tunnel": t.String(),
		}).WithError(err).Error("error while starting tunnel")

		return err
	}

	return nil
}

// Stop shuts down a detached mole's application instance.
func (c *Client) Stop() error {
	pfp, err := GetPidFileLocation(c.Conf.Id)
	if err != nil {
		return fmt.Errorf("error getting information about aliases directory: %v", err)
	}

	if _, err := os.Stat(pfp); os.IsNotExist(err) {
		return fmt.Errorf("no instance of mole with id %s is running", c.Conf.Id)
	}

	cntxt := &daemon.Context{
		PidFileName: pfp,
	}

	d, err := cntxt.Search()
	if err != nil {
		return err
	}

	err = d.Kill()
	if err != nil {
		return err
	}

	err = os.RemoveAll(pfp)
	if err != nil {
		return err
	}

	return nil
}

// Merge overwrites Configuration from the given Alias.
//
// Certain attributes like Verbose, Insecure and Detach will be overwritten
// only if they are found on the givenFlags which should contain the name of
// all flags given by the user through UI (e.g. CLI).
func (c *Configuration) Merge(al *alias.Alias, givenFlags []string) error {
	var fl flags

	fl = givenFlags

	if !fl.lookup("verbose") {
		c.Verbose = al.Verbose
	}

	if !fl.lookup("insecure") {
		c.Insecure = al.Insecure
	}

	if !fl.lookup("detach") {
		c.Detach = al.Detach
	}

	c.Id = al.Name
	c.TunnelType = al.TunnelType

	srcl := alias.AddressInputList{}
	for _, src := range al.Source {
		err := srcl.Set(src)
		if err != nil {
			return err
		}
	}
	c.Source = srcl

	dstl := alias.AddressInputList{}
	for _, dst := range al.Destination {
		err := dstl.Set(dst)
		if err != nil {
			return err
		}
	}
	c.Destination = dstl

	srv := alias.AddressInput{}
	err := srv.Set(al.Server)
	if err != nil {
		return err
	}
	c.Server = srv

	c.Key = al.Key

	kai, err := time.ParseDuration(al.KeepAliveInterval)
	if err != nil {
		return err
	}
	c.KeepAliveInterval = kai

	c.ConnectionRetries = al.ConnectionRetries

	war, err := time.ParseDuration(al.WaitAndRetry)
	if err != nil {
		return err
	}
	c.WaitAndRetry = war

	c.SshAgent = al.SshAgent

	tim, err := time.ParseDuration(al.Timeout)
	if err != nil {
		return err
	}
	c.Timeout = tim

	c.SshConfig = al.SshConfig

	c.Rpc = al.Rpc

	c.RpcAddress = al.RpcAddress

	return nil
}

// ShowRpc returns runtime information about the mole client.
func ShowRpc(params interface{}) (json.RawMessage, error) {
	if cli == nil {
		return nil, fmt.Errorf("client configuration could not be found.")
	}

	conf := cli.Conf

	cj, err := json.Marshal(conf)
	if err != nil {
		return nil, err
	}

	return json.RawMessage(cj), nil
}

func startDaemonProcess(instanceConf *DetachedInstance) error {
	cntxt := &daemon.Context{
		PidFileName: InstancePidFile,
		PidFilePerm: 0644,
		LogFileName: InstanceLogFile,
		LogFilePerm: 0640,
		Umask:       027,
		Args:        os.Args,
	}

	d, err := cntxt.Reborn()
	if err != nil {
		return err
	}

	if d != nil {
		err = os.Rename(InstancePidFile, instanceConf.PidFile)
		if err != nil {
			return err
		}

		err = os.Rename(InstanceLogFile, instanceConf.LogFile)
		if err != nil {
			return err
		}

		log.Infof("execute \"mole stop %s\" if you like to stop it at any time", instanceConf.Id)

		os.Exit(0)
	}

	defer cntxt.Release()

	return nil
}

type flags []string

func (fs flags) lookup(flag string) bool {
	for _, f := range fs {
		if flag == f {
			return true
		}
	}

	return false
}
