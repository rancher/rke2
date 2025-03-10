//go:build linux
// +build linux

package defaults

import (
	"os"

	pkgerrors "github.com/pkg/errors"
)

func createDataDir(dataDir string, perm os.FileMode) error {
	if dataDir == "" {
		return nil
	}

	if err := os.MkdirAll(dataDir, perm); err != nil {
		return pkgerrors.WithMessagef(err, "failed to create directory %s", dataDir)
	}
	return nil
}
