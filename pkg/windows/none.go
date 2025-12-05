//go:build windows
// +build windows

package windows

import (
	"context"

	daemonconfig "github.com/k3s-io/k3s/pkg/daemons/config"
	"k8s.io/client-go/rest"
)

// explicit interface check
var _ CNIPlugin = &None{}

// None is a stub implementation for clusters not using a packaged CNI
type None struct{}

func (_ *None) Setup(ctx context.Context, nodeConfig *daemonconfig.Node, restConfig *rest.Config, dataDir string) error {
	return nil
}

func (_ *None) Start(ctx context.Context) error {
	return nil
}

func (_ *None) GetConfig() *CNICommonConfig {
	return &CNICommonConfig{}
}
