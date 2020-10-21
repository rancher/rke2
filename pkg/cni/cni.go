package cni

import (
	"strings"

	"github.com/pkg/errors"
)

const (
	Canal  string = "canal"
	Cilium string = "cilium"
	None   string = "none"
)

var (
	All = []string{Canal, Cilium, None}
)

// ValidateCNIPlugin checks whether the provided cniPlugin is a valid
// CNI plugin
func ValidateCNIPlugin(cniPlugin string) error {
	for _, currentCNIPlugin := range All {
		if cniPlugin == currentCNIPlugin {
			return nil
		}
	}
	return errors.Errorf("unknown CNI plugin: %s (valid values: %s)", cniPlugin, strings.Join(All, ", "))
}
