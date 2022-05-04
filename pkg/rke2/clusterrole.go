package rke2

import (
	"context"
	"sync"

	"github.com/k3s-io/k3s/pkg/cli/cmds"
	"github.com/sirupsen/logrus"
	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/client-go/tools/clientcmd"
	rbacrest "k8s.io/kubernetes/pkg/registry/rbac/rest"
)

// setClusterRoles applies common clusterroles and clusterrolebindings that are critical
// to the function of internal controllers.
func setClusterRoles() cmds.StartupHook {
	return func(ctx context.Context, wg *sync.WaitGroup, args cmds.StartupHookArgs) error {
		go func() {
			defer wg.Done()
			<-args.APIServerReady
			logrus.Info("Applying Cluster Role Bindings")

			config, err := clientcmd.BuildConfigFromFlags("", args.KubeConfigAdmin)
			if err != nil {
				logrus.Fatalf("clusterrole: new k8s client: %v", err)
			}

			stopChan := make(chan struct{})
			defer close(stopChan)

			// kube-apiserver has a post-start hook that reconciles the built-in cluster RBAC on every startup.
			// We're reusing that here to bootstrap our own roles and bindings.
			hookContext := genericapiserver.PostStartHookContext{
				LoopbackClientConfig: config,
				StopCh:               stopChan,
			}

			policy := rbacrest.PolicyData{
				ClusterRoles:        clusterRoles(),
				ClusterRoleBindings: clusterRoleBindings(),
				Roles:               roles(),
				RoleBindings:        roleBindings(),
			}
			if err := policy.EnsureRBACPolicy()(hookContext); err != nil {
				logrus.Fatalf("clusterrole: EnsureRBACPolicy failed: %v", err)
			}

			logrus.Info("Cluster Role Bindings applied successfully")
		}()
		return nil
	}
}
