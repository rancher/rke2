package rke2

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
)

const (
	namespaceAnnotationNetworkPolicy    = "np.rke2.io"
	namespaceAnnotationNetworkDNSPolicy = "np.rke2.io/dns"

	defaultNetworkPolicyName    = "default-network-policy"
	defaultNetworkDNSPolicyName = "default-network-dns-policy"
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
		PolicyTypes: []v1.PolicyType{
			v1.PolicyTypeIngress,
		},
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

var (
	tcp = corev1.ProtocolTCP
	udp = corev1.ProtocolUDP
)

// networkDNSPolicy allows for all DNS traffic
// into the kube-system namespace.
var networkDNSPolicy = v1.NetworkPolicy{
	ObjectMeta: metav1.ObjectMeta{
		Name: defaultNetworkDNSPolicyName,
	},
	Spec: v1.NetworkPolicySpec{
		PodSelector: metav1.LabelSelector{
			MatchLabels: map[string]string{
				"k8s-app": "kube-dns",
			},
		},
		PolicyTypes: []v1.PolicyType{
			v1.PolicyTypeIngress,
		},
		Ingress: []v1.NetworkPolicyIngressRule{
			{
				Ports: []v1.NetworkPolicyPort{
					{
						Protocol: &tcp,
						Port: &intstr.IntOrString{
							IntVal: int32(53),
						},
					},
					{
						Protocol: &udp,
						Port: &intstr.IntOrString{
							IntVal: int32(53),
						},
					},
					{
						Protocol: &tcp,
						Port: &intstr.IntOrString{
							IntVal: int32(9153),
						},
					},
				},
			},
		},
		Egress: []v1.NetworkPolicyEgressRule{},
	},
}

// setNetworkPolicy applies the default network policy for the given namespace and updates
// the given namespaces' annotation. First, the namespaces' annotation is checked for existence.
// If the annotation exists, we move on. If the annotation doesn't exist, we check to see if the
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
		if _, err := cs.NetworkingV1().NetworkPolicies(namespace).Get(ctx, defaultNetworkPolicyName, metav1.GetOptions{}); err == nil {
			if err := cs.NetworkingV1().NetworkPolicies(namespace).Delete(ctx, defaultNetworkPolicyName, metav1.DeleteOptions{}); err != nil {
				if !apierrors.IsNotFound(err) {
					return err
				}
			}
		}
		if _, err := cs.NetworkingV1().NetworkPolicies(namespace).Create(ctx, &networkPolicy, metav1.CreateOptions{}); err != nil {
			if !apierrors.IsAlreadyExists(err) {
				return err
			}
		}
		ns.Annotations[namespaceAnnotationNetworkPolicy] = cisAnnotationValue

		if err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			if _, err := cs.CoreV1().Namespaces().Update(ctx, ns, metav1.UpdateOptions{}); err != nil {
				if apierrors.IsConflict(err) {
					return updateNamespaceRef(ctx, cs, ns)
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

// setNetworkDNSPolicy sets the default DNS policy allowing the
// required DNS traffic.
func setNetworkDNSPolicy(ctx context.Context, cs *kubernetes.Clientset) error {
	ns, err := cs.CoreV1().Namespaces().Get(ctx, metav1.NamespaceSystem, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("networkPolicy: get %s - %w", metav1.NamespaceSystem, err)
	}
	if ns.Annotations == nil {
		ns.Annotations = make(map[string]string)
	}
	if _, ok := ns.Annotations[namespaceAnnotationNetworkDNSPolicy]; !ok {
		if _, err := cs.NetworkingV1().NetworkPolicies(metav1.NamespaceSystem).Get(ctx, defaultNetworkDNSPolicyName, metav1.GetOptions{}); err == nil {
			if err := cs.NetworkingV1().NetworkPolicies(metav1.NamespaceSystem).Delete(ctx, defaultNetworkDNSPolicyName, metav1.DeleteOptions{}); err != nil {
				if !apierrors.IsNotFound(err) {
					return err
				}
			}
		}
		if _, err := cs.NetworkingV1().NetworkPolicies(metav1.NamespaceSystem).Create(ctx, &networkDNSPolicy, metav1.CreateOptions{}); err != nil {
			if !apierrors.IsAlreadyExists(err) {
				return err
			}
		}
		ns.Annotations[namespaceAnnotationNetworkDNSPolicy] = cisAnnotationValue

		if err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			if _, err := cs.CoreV1().Namespaces().Update(ctx, ns, metav1.UpdateOptions{}); err != nil {
				if apierrors.IsConflict(err) {
					return updateNamespaceRef(ctx, cs, ns)
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
func setNetworkPolicies(cisMode bool, namespaces []string) func(context.Context, <-chan struct{}, string) error {
	return func(ctx context.Context, apiServerReady <-chan struct{}, kubeConfigAdmin string) error {
		// check if we're running in CIS mode and if so,
		// apply the network policy.
		if cisMode {
			logrus.Info("Applying network policies...")
			go func() {
				<-apiServerReady

				cs, err := newClient(kubeConfigAdmin, nil)
				if err != nil {
					logrus.Fatalf("networkPolicy: new k8s client: %s", err.Error())
				}
				for _, namespace := range namespaces {
					if err := setNetworkPolicy(ctx, namespace, cs); err != nil {
						logrus.Fatal(err)
					}
				}
				if err := setNetworkDNSPolicy(ctx, cs); err != nil {
					logrus.Fatal(err)
				}
				logrus.Info("Applying network policies complete")
			}()
		}
		return nil
	}
}
