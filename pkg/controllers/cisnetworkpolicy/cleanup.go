package cisnetworkpolicy

import (
	"context"
	"encoding/json"
	"time"

	"github.com/k3s-io/k3s/pkg/server"
	coreclient "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
)

var (
	finalizerKey = "wrangler.cattle.io/cisnetworkpolicy-node"
)

// Cleanup removes the OnRemove finalizer from any nodes.
// This must be done to clean up from any previously registered OnRemove handlers that are currently disabled.
func Cleanup(ctx context.Context, sc *server.Context) error {
	return unregister(ctx, sc.Core.Core().V1().Node())
}

func unregister(ctx context.Context, nodes coreclient.NodeController) error {
	logrus.Debugf("CISNetworkPolicyController: Removing controller hooks for NetworkPolicy %s", flannelHostNetworkPolicyName)
	go wait.PollImmediateUntilWithContext(ctx, time.Second*30, func(_ context.Context) (bool, error) {
		nodesList, err := nodes.List(metav1.ListOptions{})
		if err != nil {
			logrus.Warnf("CISNetworkPolicyController: failed to list nodes: %v", err)
			return false, nil
		}
		for _, node := range nodesList.Items {
			for _, finalizer := range node.ObjectMeta.Finalizers {
				if finalizer == finalizerKey {
					if err := removeFinalizer(nodes, node); err != nil {
						logrus.Warnf("CISNetworkPolicyController: failed to remove finalizer from node %s: %v", node.Name, err)
						return false, nil
					}
					break
				}
			}
		}
		return true, nil
	})
	return nil
}

func removeFinalizer(nodes coreclient.NodeController, node core.Node) error {
	newFinalizers := []string{}
	finalizers := node.ObjectMeta.Finalizers
	for k, v := range finalizers {
		if v != finalizerKey {
			continue
		}
		newFinalizers = append(finalizers[:k], finalizers[k+1:]...)
	}
	patch := []map[string]interface{}{
		{
			"op":    "replace",
			"value": newFinalizers,
			"path":  "/metadata/finalizers",
		},
	}
	b, err := json.Marshal(patch)
	if err != nil {
		return err
	}
	_, err = nodes.Patch(node.Name, types.JSONPatchType, b)
	return err
}
