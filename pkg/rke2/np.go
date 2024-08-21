package rke2

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/k3s-io/k3s/pkg/cli/cmds"
	"github.com/k3s-io/k3s/pkg/util"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
)

const (
	cisAnnotationValue = "resolved"
)

var (
	tcp = corev1.ProtocolTCP
	udp = corev1.ProtocolUDP
)

type policyTemplate struct {
	name          string
	annotationKey string
	podSelector   metav1.LabelSelector
	ingress       []v1.NetworkPolicyIngressRule
}

// defaultNamespacePolicies contains a list of policies that are applied to all core namespaces.
var defaultNamespacePolicies = []policyTemplate{
	{
		// default-network-policy is a base level network policy applied
		// to the 3 primary namespaces: kube-system, kube-public, and default.
		// This policy only allows for intra-namespace traffic.
		name:          "default-network-policy",
		annotationKey: "np.rke2.io",
		podSelector:   metav1.LabelSelector{}, // empty to match all pods
		ingress: []v1.NetworkPolicyIngressRule{
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

// defaultKubeSystemPolicies is a list of policies that are applied to the kube-system namespace.
var defaultKubeSystemPolicies = []policyTemplate{
	{
		// allows DNS traffic into the coredns pods
		name:          "default-network-dns-policy",
		annotationKey: "np.rke2.io/dns",
		podSelector:   metav1.LabelSelector{MatchLabels: labels.Set{"k8s-app": "kube-dns"}},
		ingress: []v1.NetworkPolicyIngressRule{
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
	},
	{
		// allows for all http and https traffic into the kube-system namespace to the ingress-nginx controller pods
		name:          "default-network-ingress-policy",
		annotationKey: "np.rke2.io/ingress",
		podSelector:   metav1.LabelSelector{MatchLabels: labels.Set{"app.kubernetes.io/name": "rke2-ingress-nginx"}},
		ingress: []v1.NetworkPolicyIngressRule{
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
	},
	{
		// allows for https traffic into the to the ingress-nginx controller webhook
		name:          "default-network-ingress-webhook-policy",
		annotationKey: "np.rke2.io/ingress-webhook",
		podSelector:   metav1.LabelSelector{MatchLabels: labels.Set{"app.kubernetes.io/name": "rke2-ingress-nginx"}},
		ingress: []v1.NetworkPolicyIngressRule{
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
	},
	{
		// allows for all http and https traffic into the kube-system namespace to the traefik ingress controller pods
		name:          "default-network-traefik-policy",
		annotationKey: "np.rke2.io/traefik",
		podSelector:   metav1.LabelSelector{MatchLabels: labels.Set{"app.kubernetes.io/name": "rke2-traefik"}},
		ingress: []v1.NetworkPolicyIngressRule{
			{
				Ports: []v1.NetworkPolicyPort{
					{
						Protocol: &tcp,
						Port: &intstr.IntOrString{
							Type:   intstr.String,
							StrVal: "web",
						},
					},
					{
						Protocol: &tcp,
						Port: &intstr.IntOrString{
							Type:   intstr.String,
							StrVal: "websecure",
						},
					},
				},
			},
		},
	},
	{
		// allows for https traffic into the CSI snapshot validation webhook
		name:          "default-network-snapshot-validation-webhook-policy",
		annotationKey: "np.rke2.io/snapshot-validation-webhook",
		podSelector:   metav1.LabelSelector{MatchLabels: labels.Set{"app.kubernetes.io/name": "rke2-snapshot-validation-webhook"}},
		ingress: []v1.NetworkPolicyIngressRule{
			{
				Ports: []v1.NetworkPolicyPort{
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
	},
	{
		// allows for https traffic into the to the metrics server
		name:          "default-network-metrics-server-policy",
		annotationKey: "np.rke2.io/metrics-server",
		podSelector:   metav1.LabelSelector{MatchLabels: labels.Set{"app": "rke2-metrics-server"}},
		ingress: []v1.NetworkPolicyIngressRule{
			{
				Ports: []v1.NetworkPolicyPort{
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
	},
}

// policyFromTemplate returns a full v1.NetworkPolicy for the provided policy template
func policyFromTemplate(template policyTemplate) *v1.NetworkPolicy {
	return &v1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:        template.name,
			Annotations: map[string]string{template.annotationKey: cisAnnotationValue},
		},
		Spec: v1.NetworkPolicySpec{
			PodSelector: template.podSelector,
			PolicyTypes: []v1.PolicyType{
				v1.PolicyTypeIngress,
			},
			Ingress: template.ingress,
		},
	}
}

// setNetworkPolicies applies the provided network policy templates to the given namespace and updates
// the given namespaces' annotation. First, the namespaces' annotation is checked for existence.
// If the annotation exists, we move on. If the annotation doesn't exist, we check to see if the
// policy exists. If it does, we delete it, and create the new default policy.
func setPoliciesFromTemplates(ctx context.Context, cs kubernetes.Interface, templates []policyTemplate, namespace string) error {
	ns, err := cs.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("networkPolicy: get %s - %w", namespace, err)
	}
	if ns.Annotations == nil {
		ns.Annotations = make(map[string]string)
	}
	for _, template := range templates {
		if _, ok := ns.Annotations[template.annotationKey]; !ok {
			if _, err := cs.NetworkingV1().NetworkPolicies(namespace).Get(ctx, template.name, metav1.GetOptions{}); err == nil {
				if err := cs.NetworkingV1().NetworkPolicies(namespace).Delete(ctx, template.name, metav1.DeleteOptions{}); err != nil {
					if !apierrors.IsNotFound(err) {
						return err
					}
				}
			}
			if _, err := cs.NetworkingV1().NetworkPolicies(namespace).Create(ctx, policyFromTemplate(template), metav1.CreateOptions{}); err != nil {
				if !apierrors.IsAlreadyExists(err) {
					return err
				}
			}
			ns.Annotations[template.annotationKey] = cisAnnotationValue

			if err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
				if _, err := cs.CoreV1().Namespaces().Update(ctx, ns, metav1.UpdateOptions{}); err != nil {
					if apierrors.IsConflict(err) {
						return updateNamespaceRef(ctx, cs, ns)
					}
					return err
				}
				return nil
			}); err != nil {
				return fmt.Errorf("failed to apply network policy %s to namespace %s: %v", template.name, ns.Name, err)
			}
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

		go func() {
			defer wg.Done()
			<-args.APIServerReady
			cs, err := util.GetClientSet(args.KubeConfigSupervisor)
			if err != nil {
				logrus.Fatalf("np: new k8s client: %v", err)
			}

			go wait.PollImmediateInfiniteWithContext(ctx, 5*time.Second, func(ctx context.Context) (bool, error) {
				logrus.Info("Applying network policies...")
				for _, namespace := range namespaces {
					if err := setPoliciesFromTemplates(ctx, cs, defaultNamespacePolicies, namespace); err != nil {
						logrus.Errorf("Network policy apply failed, will retry: %v", err)
						return false, nil
					}
					if namespace == metav1.NamespaceSystem {
						if err := setPoliciesFromTemplates(ctx, cs, defaultKubeSystemPolicies, namespace); err != nil {
							logrus.Errorf("Network policy apply failed, will retry: %v", err)
							return false, nil
						}
					}
				}
				logrus.Info("Applying network policies complete")
				return true, nil
			})
		}()
		return nil
	}
}

// updateNamespaceRef retrieves the most recent revision of Namespace ns, copies over any annotations from
// the passed revision of the Namespace to the most recent revision, and updates the pointer to refer to the
// most recent revision. This get/change/update pattern is required to alter an object
// that may have changed since it was retrieved.
func updateNamespaceRef(ctx context.Context, cs kubernetes.Interface, ns *corev1.Namespace) error {
	logrus.Infof("Updating namespace %s to apply network policy annotations", ns.Name)
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
