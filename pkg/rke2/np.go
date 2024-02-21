package rke2

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/k3s-io/k3s/pkg/cli/cmds"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/transport"
	"k8s.io/client-go/util/retry"
)

const (
	namespaceAnnotationNetworkPolicy        = "np.rke2.io"
	namespaceAnnotationNetworkDNSPolicy     = "np.rke2.io/dns"
	namespaceAnnotationNetworkIngressPolicy = "np.rke2.io/ingress"
	namespaceAnnotationNetworkWebhookPolicy = "np.rke2.io/ingress-webhook"

	defaultNetworkPolicyName        = "default-network-policy"
	defaultNetworkDNSPolicyName     = "default-network-dns-policy"
	defaultNetworkIngressPolicyName = "default-network-ingress-policy"
	defaultNetworkWebhookPolicyName = "default-network-ingress-webhook-policy"

	defaultTimeout     = 30
	cisAnnotationValue = "resolved"
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
				},
			},
		},
		Egress: []v1.NetworkPolicyEgressRule{},
	},
}

// networkIngressPolicy allows for all http and https traffic
// into the kube-system namespace to the ingress controller pods.
var networkIngressPolicy = v1.NetworkPolicy{
	ObjectMeta: metav1.ObjectMeta{
		Name: defaultNetworkIngressPolicyName,
	},
	Spec: v1.NetworkPolicySpec{
		PodSelector: metav1.LabelSelector{
			MatchLabels: map[string]string{
				"app.kubernetes.io/name": "rke2-ingress-nginx",
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
							Type:   intstr.String,
							StrVal: "http",
						},
					},
					{
						Protocol: &tcp,
						Port: &intstr.IntOrString{
							Type:   intstr.String,
							StrVal: "https",
						},
					},
				},
			},
		},
		Egress: []v1.NetworkPolicyEgressRule{},
	},
}

// networkWebhookPolicy allows for https traffic
// into the kube-system namespace to the ingress controller webhook.
var networkWebhookPolicy = v1.NetworkPolicy{
	ObjectMeta: metav1.ObjectMeta{
		Name: defaultNetworkWebhookPolicyName,
	},
	Spec: v1.NetworkPolicySpec{
		PodSelector: metav1.LabelSelector{
			MatchLabels: map[string]string{
				"app.kubernetes.io/name": "rke2-ingress-nginx",
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
							Type:   intstr.String,
							StrVal: "webhook",
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

// setNetworkIngressPolicy sets the default Ingress policy allowing the
// required HTTP/HTTPS traffic to ingress nginx pods.
func setNetworkIngressPolicy(ctx context.Context, cs *kubernetes.Clientset) error {
	ns, err := cs.CoreV1().Namespaces().Get(ctx, metav1.NamespaceSystem, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("networkPolicy: get %s - %w", metav1.NamespaceSystem, err)
	}
	if ns.Annotations == nil {
		ns.Annotations = make(map[string]string)
	}
	if _, ok := ns.Annotations[namespaceAnnotationNetworkIngressPolicy]; !ok {
		if _, err := cs.NetworkingV1().NetworkPolicies(metav1.NamespaceSystem).Get(ctx, defaultNetworkIngressPolicyName, metav1.GetOptions{}); err == nil {
			if err := cs.NetworkingV1().NetworkPolicies(metav1.NamespaceSystem).Delete(ctx, defaultNetworkIngressPolicyName, metav1.DeleteOptions{}); err != nil {
				if !apierrors.IsNotFound(err) {
					return err
				}
			}
		}
		if _, err := cs.NetworkingV1().NetworkPolicies(metav1.NamespaceSystem).Create(ctx, &networkIngressPolicy, metav1.CreateOptions{}); err != nil {
			if !apierrors.IsAlreadyExists(err) {
				return err
			}
		}
		ns.Annotations[namespaceAnnotationNetworkIngressPolicy] = cisAnnotationValue

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

// setNetworkWebhookPolicy sets the default Ingress Webhook policy allowing the
// required webhook traffic to ingress nginx pods.
func setNetworkWebhookPolicy(ctx context.Context, cs *kubernetes.Clientset) error {
	ns, err := cs.CoreV1().Namespaces().Get(ctx, metav1.NamespaceSystem, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("networkPolicy: get %s - %w", metav1.NamespaceSystem, err)
	}
	if ns.Annotations == nil {
		ns.Annotations = make(map[string]string)
	}
	if _, ok := ns.Annotations[namespaceAnnotationNetworkWebhookPolicy]; !ok {
		if _, err := cs.NetworkingV1().NetworkPolicies(metav1.NamespaceSystem).Get(ctx, defaultNetworkWebhookPolicyName, metav1.GetOptions{}); err == nil {
			if err := cs.NetworkingV1().NetworkPolicies(metav1.NamespaceSystem).Delete(ctx, defaultNetworkWebhookPolicyName, metav1.DeleteOptions{}); err != nil {
				if !apierrors.IsNotFound(err) {
					return err
				}
			}
		}
		if _, err := cs.NetworkingV1().NetworkPolicies(metav1.NamespaceSystem).Create(ctx, &networkWebhookPolicy, metav1.CreateOptions{}); err != nil {
			if !apierrors.IsAlreadyExists(err) {
				return err
			}
		}
		ns.Annotations[namespaceAnnotationNetworkWebhookPolicy] = cisAnnotationValue

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
func setNetworkPolicies(cisMode bool, namespaces []string) cmds.StartupHook {
	return func(ctx context.Context, wg *sync.WaitGroup, args cmds.StartupHookArgs) error {
		// check if we're running in CIS mode and if so,
		// apply the network policy.
		if !cisMode {
			wg.Done()
			return nil
		}

		logrus.Info("Applying network policies...")
		go func() {
			defer wg.Done()
			<-args.APIServerReady
			cs, err := newClient(args.KubeConfigSupervisor, nil)
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

			if err := setNetworkIngressPolicy(ctx, cs); err != nil {
				logrus.Fatal(err)
			}

			if err := setNetworkWebhookPolicy(ctx, cs); err != nil {
				logrus.Fatal(err)
			}

			logrus.Info("Applying network policies complete")
		}()

		return nil
	}
}

// updateNamespaceRef retrieves the most recent revision of Namespace ns, copies over any annotations from
// the passed revision of the Namespace to the most recent revision, and updates the pointer to refer to the
// most recent revision. This get/change/update pattern is required to alter an object
// that may have changed since it was retrieved.
func updateNamespaceRef(ctx context.Context, cs *kubernetes.Clientset, ns *corev1.Namespace) error {
	logrus.Info("updating namespace: " + ns.Name)
	newNS, err := cs.CoreV1().Namespaces().Get(ctx, ns.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	if newNS.Annotations == nil {
		newNS.Annotations = make(map[string]string, len(ns.Annotations))
	}
	// copy annotations, since we may have changed them
	for k, v := range ns.Annotations {
		newNS.Annotations[k] = v
	}
	*ns = *newNS
	return nil
}

// newClient create a new Kubernetes client from configuration.
func newClient(kubeConfigPath string, k8sWrapTransport transport.WrapperFunc) (*kubernetes.Clientset, error) {
	config, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	if err != nil {
		return nil, err
	}
	if k8sWrapTransport != nil {
		config.WrapTransport = k8sWrapTransport
	}
	config.Timeout = time.Second * defaultTimeout
	return kubernetes.NewForConfig(config)
}
