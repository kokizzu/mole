package fsutils

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
)

const (
	PidFile = "pid"
)

// Dir returns the location where all mole related files are persisted,
// including alias configuration and log files.
func Dir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	mp := filepath.Join(home, ".mole")

	return mp, nil
}

// CreateHomeDir creates then returns the location where all mole related files
// are persisted, including alias configuration and log files.
func CreateHomeDir() (string, error) {

	home, err := Dir()
	if err != nil {
		return "", err
	}

	if _, err := os.Stat(home); os.IsNotExist(err) {
		err := os.MkdirAll(home, 0755)
		if err != nil {
			return "", err
		}
	}

	return home, err
}

// CreateInstanceDir creates and then returns the location where all files
// related to a specific mole instance are persisted.
func CreateInstanceDir(appId string) (string, error) {
	home, err := Dir()
	if err != nil {
		return "", err
	}

	d := filepath.Join(home, appId)

	if _, err := os.Stat(d); os.IsNotExist(err) {
		err := os.MkdirAll(d, 0755)
		if err != nil {
			return "", err
		}
	}

	return d, nil
}

// InstanceDir returns the location where all files related to a specific mole
// instance are persisted.
func InstanceDir(id string) (string, error) {
	home, err := Dir()
	if err != nil {
		return "", err
	}

	return filepath.Join(home, id), nil
}

// RpcAddress returns the network address of the rpc server for a given
// application instance id or alias.
func RpcAddress(id string) (string, error) {
	d, err := InstanceDir(id)
	if err != nil {
		return "", err
	}

	rf := filepath.Join(d, "rpc")

	if _, err := os.Stat(rf); os.IsNotExist(err) {
		return "", nil
	}

	data, err := ioutil.ReadFile(rf)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

// PidFileLocation returns the location of the pid file associated with a mole
// instance.
//
// Only detached instances keep a pid file so, if an alias is given to
// this function, a path to a non-existent file will be returned.
func PidFileLocation(id string) (string, error) {
	d, err := InstanceDir(id)
	if err != nil {
		return "", err
	}

	return filepath.Join(d, PidFile), nil
}

// Pid returns the process id associated with the given alias or id.
func Pid(id string) (int, error) {
	if pid, err := strconv.Atoi(id); err == nil {
		return pid, nil
	}

	pfl, err := PidFileLocation(id)
	if err != nil {
		return -1, err
	}

	ps, err := ioutil.ReadFile(pfl)
	if err != nil {
		return -1, err
	}

	pid, err := strconv.Atoi(string(ps))
	if err != nil {
		return -1, err
	}

	return pid, nil
}
