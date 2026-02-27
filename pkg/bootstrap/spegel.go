package bootstrap

import (
	"context"
	"net/url"
	"time"

	"github.com/containerd/containerd/v2/core/remotes/docker"
	daemonconfig "github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/spegel"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/wait"
)

const registryWaitTime = time.Second * 15

// isEmbeddedRegistryConfigured returns true if the embedded registry is enabled
// and has at least one valid mirror configured.
func isEmbeddedRegistryConfigured(nodeConfig *daemonconfig.Node) bool {
	var hasValidMirrors bool
	for host := range nodeConfig.AgentConfig.Registry.Mirrors {
		if _, err := url.Parse("https://" + host); err == nil && !docker.IsLocalhost(host) {
			hasValidMirrors = true
			break
		}
	}
	return hasValidMirrors && nodeConfig.EmbeddedRegistry && spegel.DefaultRegistry != nil
}

// WaitForEmbeddedRegistry waits for the embedded registry to become ready, if it is enabled.
func WaitForEmbeddedRegistry(ctx context.Context, nodeConfig *daemonconfig.Node) error {
	// if spegel is enabled, wait for it to start up so that we can attempt to pull content through it
	if isEmbeddedRegistryConfigured(nodeConfig) {
		logrus.Infof("Waiting up to %s for embedded registry to find peers", registryWaitTime)
		return wait.PollUntilContextTimeout(ctx, time.Second, registryWaitTime, true, func(ctx context.Context) (bool, error) {
			ready, _ := spegel.DefaultRegistry.Ready(ctx)
			return ready, nil
		})
	}
	return nil
}
