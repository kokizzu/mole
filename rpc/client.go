package rpc

import (
	"context"
	"net"
	"os"
	"path/filepath"

	"github.com/davrodpin/mole/fsutils"

	ps "github.com/mitchellh/go-ps"
	log "github.com/sirupsen/logrus"
	"github.com/sourcegraph/jsonrpc2"
)

var (
	InstancePidFile = "pid"
)

type Client struct {
	Id string
}

// Show returns runtime information about another mole instance, given its
// alias or id.
func (c *Client) Show(context context.Context) (map[string]interface{}, error) {
	resp, err := CallById(context, c.Id, "show-instance", nil)
	if err != nil {
		return nil, nil
	}

	return resp, nil
}

// CallById returns the response of a remote procedure call made against
// another mole instance, given its id or alias.
func CallById(context context.Context, id, method string, params interface{}) (map[string]interface{}, error) {
	addr, err := fsutils.RpcAddress(id)
	if err != nil {
		return nil, err
	}

	resp, err := Call(context, addr, method, params)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

// Call initiates a JSON-RPC call to a given rpc server address, using the
// specified method and waits for the response.
func Call(ctx context.Context, addr, method string, params interface{}) (map[string]interface{}, error) {
	tc, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}

	stream := jsonrpc2.NewBufferedStream(tc, jsonrpc2.VarintObjectCodec{})
	h := &Handler{}
	conn := jsonrpc2.NewConn(ctx, stream, h)

	var r map[string]interface{}
	err = conn.Call(ctx, method, params, &r)
	if err != nil {
		return nil, err
	}

	return r, nil
}

// ShowAll returns runtime information about all instaces of mole running on
// the system.
func ShowAll(context context.Context) ([]map[string]interface{}, error) {
	var instances []map[string]interface{}

	home, err := fsutils.Dir()
	if err != nil {
		return nil, err
	}

	err = filepath.Walk(home, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			id := filepath.Base(path)

			if id == "." {
				return nil
			}

			if !processRunning(id) {
				log.WithFields(log.Fields{
					"rpc": "enabled",
					"id":  id,
				}).Debugf("rpc failed: process not running")

				return nil
			}

			addr, err := fsutils.RpcAddress(id)
			if err != nil {
				log.WithFields(log.Fields{
					"rpc": "enabled",
					"id":  id,
				}).WithError(err).Debugf("rpc failed")

				return nil
			}

			resp, err := Call(context, addr, "show-instance", nil)
			if err != nil {
				log.WithFields(log.Fields{
					"rpc": "enabled",
					"id":  id,
				}).WithError(err).Debugf("rpc failed")

				return nil
			}

			instances = append(instances, resp)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return instances, nil
}

func processRunning(id string) bool {
	pid, err := fsutils.Pid(id)
	if err != nil {
		return false
	}

	proc, err := ps.FindProcess(pid)
	if err != nil {
		return false
	}

	if proc == nil {
		return false
	}

	return true
}
