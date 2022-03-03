package rke2

import (
	"context"
	"sync"

	"github.com/k3s-io/k3s/pkg/cli/cmds"
)

const kubeProxyChart = "rke2-kube-proxy"

func setKubeProxyDisabled() cmds.StartupHook {
	return func(ctx context.Context, wg *sync.WaitGroup, args cmds.StartupHookArgs) error {
		go func() {
			defer wg.Done()
			<-args.APIServerReady
			args.Skips[kubeProxyChart] = true
			args.Disables[kubeProxyChart] = true
		}()
		return nil
	}
}
