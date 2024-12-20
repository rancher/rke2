package rke2

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/k3s-io/k3s/pkg/cli/cmds"
	"github.com/sirupsen/logrus"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/client-go/kubernetes"
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

			config, err := clientcmd.BuildConfigFromFlags("", args.KubeConfigSupervisor)
			if err != nil {
				logrus.Fatalf("clusterrole: new k8s restConfig: %v", err)
			}
			client, err := kubernetes.NewForConfig(config)
			if err != nil {
				logrus.Fatalf("clusterrole: new k8s client: %v", err)
			}

			// kube-apiserver has a post-start hook that reconciles the built-in cluster RBAC on every startup.
			// We're reusing that here to bootstrap our own roles and bindings.
			hookContext := genericapiserver.PostStartHookContext{
				LoopbackClientConfig: config,
				Context:              ctx,
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

			// Begin remediation for https://github.com/rancher/rke2/issues/6272
			// This can be removed after ~1 year of shipping releases not affected by this issue.

			// stub binding/clusterrolebinding for marshalling the patch json
			type binding struct {
				Subjects []rbacv1.Subject `json:"subjects"`
			}

			// It is not critical if these fail, the excess subjects just need to be cleaned up eventually
			for ns, rbs := range policy.RoleBindings {
				for _, rb := range rbs {
					b, _ := json.Marshal(binding{Subjects: rb.Subjects})
					if _, err := client.RbacV1().RoleBindings(ns).Patch(ctx, rb.Name, types.MergePatchType, b, metav1.PatchOptions{}); err != nil {
						logrus.Debugf("Failed to patch RoleBinding %s/%s subjects: %v", ns, rb.Name, err)
					}
				}
			}
			for _, crb := range policy.ClusterRoleBindings {
				b, _ := json.Marshal(binding{Subjects: crb.Subjects})
				if _, err := client.RbacV1().ClusterRoleBindings().Patch(ctx, crb.Name, types.MergePatchType, b, metav1.PatchOptions{}); err != nil {
					logrus.Debugf("Failed to patch ClusterRoleBinding %s subjects: %v", crb.Name, err)
				}
			}

			// End remediation for https://github.com/rancher/rke2/issues/6272

			logrus.Info("Cluster Role Bindings applied successfully")
		}()
		return nil
	}
}
