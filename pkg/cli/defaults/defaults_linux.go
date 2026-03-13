//go:build linux
// +build linux

package defaults

import (
	"os"

	"github.com/k3s-io/k3s/pkg/util/errors"
)

func createDataDir(dataDir string, perm os.FileMode) error {
	if dataDir == "" {
		return nil
	}

	if err := os.MkdirAll(dataDir, perm); err != nil {
		return errors.WithMessagef(err, "failed to create directory %s", dataDir)
	}
	return nil
}
