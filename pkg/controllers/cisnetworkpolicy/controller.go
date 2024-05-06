package cisnetworkpolicy

import (
	"context"
	"net"
	"sort"

	"github.com/k3s-io/k3s/pkg/server"
	"github.com/pkg/errors"
	coreclient "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	core "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
)

const (
	flannelPresenceLabel         = "flannel.alpha.coreos.com/public-ip"
	flannelHostNetworkPolicyName = "rke2-flannel-host-networking"
)

func Controller(ctx context.Context, sc *server.Context) error {
	return register(ctx, sc.Core.Core().V1().Node(), sc.K8s)
}

func register(ctx context.Context,
	nodes coreclient.NodeController,
	k8s kubernetes.Interface,
) error {
	h := &handler{
		ctx: ctx,
		k8s: k8s,
	}
	logrus.Debugf("CISNetworkPolicyController: Registering controller hooks for NetworkPolicy %s", flannelHostNetworkPolicyName)
	nodes.OnChange(ctx, "cisnetworkpolicy-node", h.handle)
	nodes.OnRemove(ctx, "cisnetworkpolicy-node", h.handle)
	return nil
}

type handler struct {
	ctx context.Context
	k8s kubernetes.Interface
}

func (h *handler) handle(key string, node *core.Node) (*core.Node, error) {
	if node == nil {
		return nil, nil
	}

	return h.reconcileFlannelHostNetworkPolicy(key, node)
}

func (h *handler) reconcileFlannelHostNetworkPolicy(_ string, _ *core.Node) (*core.Node, error) {
	var np *netv1.NetworkPolicy
	var npIR *netv1.NetworkPolicyIngressRule

	npIR, err := h.generateHostNetworkPolicyIngressRule()
	if err != nil {
		return nil, err
	}

	np = h.generateHostNetworkingNetworkPolicy(npIR)

	var namespaces = []string{
		metav1.NamespaceSystem,
		metav1.NamespaceDefault,
		metav1.NamespacePublic,
	}

	for _, namespace := range namespaces {
		err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			if npGet, err := h.k8s.NetworkingV1().NetworkPolicies(namespace).Get(h.ctx, flannelHostNetworkPolicyName, metav1.GetOptions{}); err != nil {
				if apierrors.IsNotFound(err) {
					if _, err := h.k8s.NetworkingV1().NetworkPolicies(namespace).Create(h.ctx, np, metav1.CreateOptions{}); err != nil {
						if !apierrors.IsAlreadyExists(err) {
							logrus.Errorf("CISNetworkPolicyController: Error creating network policy in namespace %s: %v", namespace, err)
							return err
						}
					}
				} else {
					logrus.Errorf("CISNetworkPolicyController: Error getting network policy network policy in namespace %s: %v", namespace, err)
					return err
				}
			} else {
				npGet = npGet.DeepCopy()
				npGet.Spec.Ingress[0] = *npIR
				_, err := h.k8s.NetworkingV1().NetworkPolicies(namespace).Update(h.ctx, npGet, metav1.UpdateOptions{})
				if err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return nil, errors.Wrapf(err, "CISNetworkPolicyController: error working on network policy in namespace %s", namespace)
		}
	}
	logrus.Debugf("CISNetworkPolicyController: Handled node change")
	return nil, nil
}

func (h *handler) generateHostNetworkingNetworkPolicy(networkPolicyIngressRule *netv1.NetworkPolicyIngressRule) *netv1.NetworkPolicy {
	np := &netv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: flannelHostNetworkPolicyName,
		},
		Spec: netv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{},
			Ingress: []netv1.NetworkPolicyIngressRule{
				*networkPolicyIngressRule,
			},
			PolicyTypes: []netv1.PolicyType{
				netv1.PolicyTypeIngress,
			},
		},
	}

	return np
}

func (h *handler) generateHostNetworkPolicyIngressRule() (*netv1.NetworkPolicyIngressRule, error) {
	var npIR netv1.NetworkPolicyIngressRule

	nodes, err := h.k8s.CoreV1().Nodes().List(h.ctx, metav1.ListOptions{})
	if err != nil {
		return &npIR, errors.Wrap(err, "CISNetworkPolicyController: problem listing nodes")
	}

	for _, node := range nodes.Items {
		if _, ok := node.Annotations[flannelPresenceLabel]; !ok {
			logrus.Debugf("CISNetworkPolicyController: node=%v doesn't have flannel label, skipping", node.Name)
			continue
		}
		podCIDRFirstIP, _, err := net.ParseCIDR(node.Spec.PodCIDR)
		if err != nil {
			logrus.Debugf("CISNetworkPolicyController: node=%+v", node)
			logrus.Errorf("CISNetworkPolicyController: couldn't parse PodCIDR(%v) for node %v err=%v", node.Spec.PodCIDR, node.Name, err)
			continue
		}
		ipBlock := netv1.IPBlock{
			CIDR: podCIDRFirstIP.String() + "/32",
		}
		npIR.From = append(npIR.From, netv1.NetworkPolicyPeer{IPBlock: &ipBlock})
	}

	// sort ipblocks so it always appears in a certain order
	sort.Slice(npIR.From, func(i, j int) bool {
		return npIR.From[i].IPBlock.CIDR < npIR.From[j].IPBlock.CIDR
	})

	return &npIR, nil
}
