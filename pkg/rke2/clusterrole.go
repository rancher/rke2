package rke2

import (
	"context"
	"fmt"
	"sync"

	"github.com/rancher/k3s/pkg/cli/cmds"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// setClusterRoles applies common clusterroles and clusterrolebindings that are critical
// to the function of internal controllers.
func setClusterRoles() cmds.StartupHook {
	return func(ctx context.Context, wg *sync.WaitGroup, args cmds.StartupHookArgs) error {
		go func() {
			defer wg.Done()
			<-args.APIServerReady
			logrus.Info("Applying Cluster Role Bindings")

			cs, err := newClient(args.KubeConfigAdmin, nil)
			if err != nil {
				logrus.Fatalf("clusterrole: new k8s client: %s", err.Error())
			}

			if err := setKubeletAPIServerRoleBinding(ctx, cs); err != nil {
				logrus.Fatalf("psp: set kubeletAPIServerRoleBinding: %s", err.Error())
			}

			if err := setKubeProxyServerRoleBinding(ctx, cs); err != nil {
				logrus.Fatalf("psp: set kubeProxyServerRoleBinding: %s", err.Error())
			}

			if err := setTunnelControllerRoleBinding(ctx, cs); err != nil {
				logrus.Fatalf("psp: set tunnelControllerRoleBinding: %s", err.Error())
			}

			if err := setCloudControllerManagerRoleBinding(ctx, cs); err != nil {
				logrus.Fatalf("ccm: set cloudControllerManagerRoleBinding: %s", err.Error())
			}

			logrus.Info("Cluster Role Bindings applied successfully")
		}()
		return nil
	}
}

// setKubeletAPIServerRoleBinding creates the clusterrolebinding that grants the apiserver full access to the kubelet API
func setKubeletAPIServerRoleBinding(ctx context.Context, cs *kubernetes.Clientset) error {
	// check if clusterrolebinding exists
	if _, err := cs.RbacV1().ClusterRoleBindings().Get(ctx, kubeletAPIServerRoleBindingName, metav1.GetOptions{}); err != nil {
		if apierrors.IsNotFound(err) {
			logrus.Infof("Setting Cluster RoleBinding: %s", kubeletAPIServerRoleBindingName)

			tmpl := fmt.Sprintf(kubeletAPIServerRoleBindingTemplate, kubeletAPIServerRoleBindingName)
			if err := deployClusterRoleBindingFromYaml(ctx, cs, tmpl); err != nil {
				return err
			}
		} else {
			return err
		}
	}
	return nil
}

// setKubeProxyServerRoleBinding creates the clusterrole and clusterrolebinding that grants the kube-proxy access to the kubelet API
func setKubeProxyServerRoleBinding(ctx context.Context, cs *kubernetes.Clientset) error {
	// check if clusterrole exists
	if _, err := cs.RbacV1().ClusterRoles().Get(ctx, kubeProxyRoleName, metav1.GetOptions{}); err != nil {
		if apierrors.IsNotFound(err) {
			logrus.Infof("Setting Cluster Role: %s", kubeProxyRoleName)

			tmpl := fmt.Sprintf(kubeProxyRoleTemplate, kubeProxyRoleName)
			if err := deployClusterRoleFromYaml(ctx, cs, tmpl); err != nil {
				return err
			}
		} else {
			return err
		}
	}

	// check if clusterrolebinding exists
	if _, err := cs.RbacV1().ClusterRoleBindings().Get(ctx, kubeProxyRoleName, metav1.GetOptions{}); err != nil {
		if apierrors.IsNotFound(err) {
			logrus.Infof("Setting Cluster RoleBinding: %s", kubeProxyRoleName)

			tmpl := fmt.Sprintf(kubeProxyServerRoleBindingTemplate, kubeProxyRoleName)
			if err := deployClusterRoleBindingFromYaml(ctx, cs, tmpl); err != nil {
				return err
			}
		} else {
			return err
		}
	}

	return nil
}

// setTunnelControllerRoleBinding creates the clusterrole and clusterrolebinding used by internal controllers
// such as the agent tunnel controller
func setTunnelControllerRoleBinding(ctx context.Context, cs *kubernetes.Clientset) error {
	// check if clusterrole exists
	if _, err := cs.RbacV1().ClusterRoles().Get(ctx, tunnelControllerRoleName, metav1.GetOptions{}); err != nil {
		if apierrors.IsNotFound(err) {
			logrus.Infof("Setting Cluster Role: %s", tunnelControllerRoleName)

			tmpl := fmt.Sprintf(tunnelControllerRoleTemplate, tunnelControllerRoleName)
			if err := deployClusterRoleFromYaml(ctx, cs, tmpl); err != nil {
				return err
			}
		} else {
			return err
		}
	}

	// check if clusterrolebinding exists
	if _, err := cs.RbacV1().ClusterRoleBindings().Get(ctx, tunnelControllerRoleName, metav1.GetOptions{}); err != nil {
		if apierrors.IsNotFound(err) {
			logrus.Infof("Setting Cluster RoleBinding: %s", tunnelControllerRoleName)

			tmpl := fmt.Sprintf(tunnelControllerRoleBindingTemplate, tunnelControllerRoleName)
			if err := deployClusterRoleBindingFromYaml(ctx, cs, tmpl); err != nil {
				return err
			}
		} else {
			return err
		}
	}

	return nil
}

func setCloudControllerManagerRoleBinding(ctx context.Context, cs *kubernetes.Clientset) error {
	// check if clusterrole exists
	if _, err := cs.RbacV1().ClusterRoles().Get(ctx, cloudControllerManagerRoleName, metav1.GetOptions{}); err != nil {
		if apierrors.IsNotFound(err) {
			logrus.Infof("Setting Cluster Role: %s", cloudControllerManagerRoleName)

			tmpl := fmt.Sprintf(cloudControllerManagerRoleTemplate, cloudControllerManagerRoleName)
			if err := deployClusterRoleFromYaml(ctx, cs, tmpl); err != nil {
				return err
			}
		} else {
			return err
		}
	}

	// check if clusterrolebinding exists
	if _, err := cs.RbacV1().ClusterRoleBindings().Get(ctx, cloudControllerManagerRoleName, metav1.GetOptions{}); err != nil {
		if apierrors.IsNotFound(err) {
			logrus.Infof("Setting Cluster RoleBinding: %s", cloudControllerManagerRoleName)

			tmpl := fmt.Sprintf(cloudControllerManagerRoleBindingTemplate, cloudControllerManagerRoleName)
			if err := deployClusterRoleBindingFromYaml(ctx, cs, tmpl); err != nil {
				return err
			}
		} else {
			return err
		}
	}

	return nil
}
