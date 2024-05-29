//go:build windows
// +build windows

package defaults

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/rancher/permissions/pkg/access"
	"github.com/rancher/permissions/pkg/acl"
	"github.com/rancher/permissions/pkg/sid"
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

	if err = acl.Mkdir(dataDir, []windows.EXPLICIT_ACCESS{
		access.GrantSid(windows.GENERIC_ALL, sid.LocalSystem()),
		access.GrantSid(windows.GENERIC_ALL, sid.BuiltinAdministrators()),
	}...); err != nil {
		return fmt.Errorf("failed to create data directory %s: %v", dataDir, err)
	}

	return nil
}
