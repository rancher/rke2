package rke2

import (
	"context"
	"sync"
	"time"

	"github.com/k3s-io/k3s/pkg/cli/cmds"
	"github.com/k3s-io/k3s/pkg/util"

	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
	"k8s.io/kubernetes/plugin/pkg/admission/serviceaccount"
)

func retryError(err error) bool {
	return apierrors.IsNotFound(err)
}

// updateServiceAccountRef retrieves the most recent revision of Service Account sa
// and updates the pointer to refer to the most recent revision. This get/change/update pattern
// is required to alter an object that may have changed since it was retrieved.
func updateServiceAccountRef(ctx context.Context, namespace string, cs kubernetes.Interface, sa *v1.ServiceAccount) error {
	logrus.Info("updating service account: " + sa.Name)
	newSA, err := cs.CoreV1().ServiceAccounts(namespace).Get(ctx, sa.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	*sa = *newSA
	return nil
}

func restrictServiceAccount(ctx context.Context, namespace string, cs kubernetes.Interface) error {
	var backoff = wait.Backoff{
		Steps:    10,
		Duration: 5 * time.Second,
	}
	// There are two race conditions this function avoids, a race on getting the initial sa because it does not yet exist,
	// and a race between nodes to update the same sa, resulting in a conflict error
	return retry.OnError(backoff, retryError, func() error {
		sa, err := cs.CoreV1().ServiceAccounts(namespace).Get(ctx, serviceaccount.DefaultServiceAccountName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			var automount bool
			sa.AutomountServiceAccountToken = &automount
			if _, err = cs.CoreV1().ServiceAccounts(namespace).Update(ctx, sa, metav1.UpdateOptions{}); err != nil {
				if apierrors.IsConflict(err) {
					if getErr := updateServiceAccountRef(ctx, namespace, cs, sa); getErr != nil {
						return getErr
					}
				}
				return err
			}
			return nil
		})
	})
}

// restrictServiceAccounts disables automount across the 3 primary namespaces.
func restrictServiceAccounts(cisMode bool, namespaces []string) cmds.StartupHook {
	return func(ctx context.Context, wg *sync.WaitGroup, args cmds.StartupHookArgs) error {
		if !cisMode {
			wg.Done()
			return nil
		}

		logrus.Info("Restricting automount...")
		go func() {
			defer wg.Done()
			<-args.APIServerReady
			cs, err := util.GetClientSet(args.KubeConfigSupervisor)
			if err != nil {
				logrus.Fatalf("serviceAccount: new k8s client: %v", err)
			}
			nps := append(namespaces, "kube-node-lease")
			for _, namespace := range nps {
				if err := restrictServiceAccount(ctx, namespace, cs); err != nil {
					logrus.Fatalf("serviceAccount: namespace %s %s", namespace, err.Error())
				}
			}
			logrus.Info("Restricting automount for default serviceAccounts complete")
		}()

		return nil
	}
}
