//go:build windows
// +build windows

package defaults

import (
	"fmt"
	"os"
	"path/filepath"

	k3swindows "github.com/k3s-io/k3s/pkg/agent/util/acl"
	"github.com/pkg/errors"
	rke2windows "github.com/rancher/rke2/pkg/windows"
	"golang.org/x/sys/windows"
)

func createDataDir(dataDir string, perm os.FileMode) error {
	_, err := os.Stat(dataDir)
	doesNotExist := errors.Is(err, os.ErrNotExist)
	if err != nil && !doesNotExist {
		return fmt.Errorf("failed to create data directory %s: %v", dataDir, err)
	}

	if !doesNotExist {
		return nil
	}

	// only set restrictive ACLs the dataDir, not the full path
	path, _ := filepath.Split(dataDir)
	if os.MkdirAll(path, perm) != nil {
		return fmt.Errorf("failed  to create data directory %s: %v", dataDir, err)
	}

	if err = rke2windows.Mkdir(dataDir, []windows.EXPLICIT_ACCESS{
		k3swindows.GrantSid(windows.GENERIC_ALL, k3swindows.LocalSystemSID()),
		k3swindows.GrantSid(windows.GENERIC_ALL, k3swindows.BuiltinAdministratorsSID()),
	}...); err != nil {
		return fmt.Errorf("failed to create data directory %s: %v", dataDir, err)
	}

	return nil
}
