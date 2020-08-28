package rke2

import (
	"context"
	"fmt"

	daemonsConfig "github.com/rancher/k3s/pkg/daemons/config"
	"github.com/rancher/spur/cli"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
)

const (
	namespaceAnnotationNetworkPolicy = "np.rke2.io"

	defaultNetworkPolicyName = "default-network-policy"
)

// networkPolicy specifies a base level network policy applied
// to the 3 primary namespace: kube-system, kube-public, and default.
// This policy only allows for intra-namespace traffic.
var networkPolicy = v1.NetworkPolicy{
	ObjectMeta: metav1.ObjectMeta{
		Name: defaultNetworkPolicyName,
	},
	Spec: v1.NetworkPolicySpec{
		PodSelector: metav1.LabelSelector{}, // empty to match all pods
		PolicyTypes: []v1.PolicyType{v1.PolicyTypeIngress},
		Ingress: []v1.NetworkPolicyIngressRule{
			{
				From: []v1.NetworkPolicyPeer{
					{
						PodSelector: &metav1.LabelSelector{}, // empty to match all pods
					},
				},
			},
		},
	},
}

// setNetworkPolicy applies the default network policy for the given namespace and updates
// the given namespaces' annotation. First, the namespaces' annotation is checked for existence.
// If the annotation exists, we move on. If the annoation doesnt' exist, we check to see if the
// policy exists. If it does, we delete it, and create the new default policy.
func setNetworkPolicy(ctx context.Context, namespace string, cs *kubernetes.Clientset) error {
	ns, err := cs.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("networkPolicy: get %s - %w", namespace, err)
	}
	if ns.Annotations == nil {
		ns.Annotations = make(map[string]string)
	}
	if _, ok := ns.Annotations[namespaceAnnotationNetworkPolicy]; !ok {
		if _, err := cs.NetworkingV1().NetworkPolicies(namespace).Get(ctx, defaultNetworkPolicyName, metav1.GetOptions{}); err != nil {
			if !apierrors.IsNotFound(err) {
				return err
			}
		}
		if err := cs.NetworkingV1().NetworkPolicies(namespace).Delete(ctx, defaultNetworkPolicyName, metav1.DeleteOptions{}); err != nil {
			return err
		}
		if _, err := cs.NetworkingV1().NetworkPolicies(namespace).Create(ctx, &networkPolicy, metav1.CreateOptions{}); err != nil {
			return err
		}
		ns.Annotations[namespaceAnnotationNetworkPolicy] = cisAnnotationValue

		if err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			if _, err := cs.CoreV1().Namespaces().Update(ctx, ns, metav1.UpdateOptions{}); err != nil {
				if apierrors.IsConflict(err) {
					if err := updateNamespaceRef(ctx, cs, ns); err != nil {
						return err
					}
				}
				return err
			}
			return nil
		}); err != nil {
			logrus.Fatalf("networkPolicy: update namespace: %s - %s", ns.Name, err.Error())
		}
	}
	return nil
}

// setNetworkPolicies applies a default network policy across the 3 primary namespaces.
func setNetworkPolicies(clx *cli.Context) func(context.Context, daemonsConfig.Control) error {
	return func(ctx context.Context, cfg daemonsConfig.Control) error {
		logrus.Info("Applying network policies...")
		go func() {
			<-cfg.Runtime.APIServerReady

			cs, err := newClient(cfg.Runtime.KubeConfigAdmin, nil)
			if err != nil {
				logrus.Fatalf("networkPolicy: new k8s client: %s", err.Error())
			}
			var namespaces = []string{
				metav1.NamespaceSystem,
				metav1.NamespaceDefault,
				metav1.NamespacePublic,
			}
			for _, namespace := range namespaces {
				if err := setNetworkPolicy(ctx, namespace, cs); err != nil {
					logrus.Fatal(err)
				}
			}
			logrus.Info("Applying network policies complete")
		}()
		return nil
	}
}
