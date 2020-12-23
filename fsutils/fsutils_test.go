package fsutils_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/davrodpin/mole/fsutils"
)

var home string

func TestPid(t *testing.T) {
	expectedPid := 1234
	id := strconv.Itoa(expectedPid)

	pid, err := fsutils.Pid(id)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if expectedPid != pid {
		t.Errorf("pid does not match: want %d, got %d", expectedPid, pid)
	}
}

func TestPidAlias(t *testing.T) {
	expectedPid := 1234
	id := "test-env"

	err := createPidFile(id, expectedPid)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	pid, err := fsutils.Pid(id)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if expectedPid != pid {
		t.Errorf("pid does not match: want %d, got %d", expectedPid, pid)
	}
}

func TestMain(m *testing.M) {
	home, err := setup()
	if err != nil {
		fmt.Printf("error while loading data for TestShow: %v", err)
		os.RemoveAll(home)
		os.Exit(1)
	}

	code := m.Run()

	os.RemoveAll(home)

	os.Exit(code)

}

//setup prepares the system environment to run the tests by:
// 1. Create temp dir and <dir>/.mole
// 2. Copy fixtures to <dir>/.mole
// 3. Set temp dir as the user testDir dir
func setup() (string, error) {
	testDir, err := ioutil.TempDir("", "mole-fsutils")
	if err != nil {
		return "", fmt.Errorf("error while setting up tests: %v", err)
	}

	moleAliasDir := filepath.Join(testDir, ".mole")
	err = os.Mkdir(moleAliasDir, 0755)
	if err != nil {
		return "", fmt.Errorf("error while setting up tests: %v", err)
	}

	err = os.Setenv("HOME", testDir)
	if err != nil {
		return "", fmt.Errorf("error while setting up tests: %v", err)
	}

	err = os.Setenv("USERPROFILE", testDir)
	if err != nil {
		return "", fmt.Errorf("error while setting up tests: %v", err)
	}

	home = testDir

	return moleAliasDir, nil
}

func createPidFile(id string, pid int) error {
	dir := filepath.Join(home, ".mole", id)

	err := os.Mkdir(dir, 0755)
	if err != nil {
		return err
	}

	d := []byte(strconv.Itoa(pid))
	err = ioutil.WriteFile(filepath.Join(dir, fsutils.PidFile), d, 0644)
	if err != nil {
		return err
	}

	if err != nil {
		return err
	}

	return nil
}
