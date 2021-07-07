package rke2

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func restrictServiceAccount(ctx context.Context, namespace string, cs *kubernetes.Clientset) error {
	var sa *v1.ServiceAccount
	var err error
	for {
		sa, err = cs.CoreV1().ServiceAccounts(namespace).Get(ctx, "default", metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				logrus.Infof("namespace: %s: serviceaccount: default does not yet exist", namespace)
				time.Sleep(5 * time.Second)
				continue
			}
			return err
		}
		break
	}
	if sa.AutomountServiceAccountToken == nil {
		sa.AutomountServiceAccountToken = new(bool)
	}
	*sa.AutomountServiceAccountToken = false
	_, err = cs.CoreV1().ServiceAccounts(namespace).Update(ctx, sa, metav1.UpdateOptions{})
	return err
}

// restrictServiceAccounts disables automount across the 3 primary namespaces.
func restrictServiceAccounts(cisMode bool) func(context.Context, <-chan struct{}, string) error {
	return func(ctx context.Context, apiServerReady <-chan struct{}, kubeConfigAdmin string) error {
		if cisMode {
			logrus.Info("Restricting automount...")
			go func() {
				<-apiServerReady
				cs, err := newClient(kubeConfigAdmin, nil)
				if err != nil {
					logrus.Fatalf("serviceAccount: new k8s client: %s", err.Error())
				}
				var namespaces = []string{
					metav1.NamespaceDefault,
					metav1.NamespaceSystem,
					metav1.NamespacePublic,
					"kube-node-lease",
				}
				for _, namespace := range namespaces {
					if err := restrictServiceAccount(ctx, namespace, cs); err != nil {
						logrus.Fatalf("serviceAccount: namespace %s %s", namespace, err.Error())
					}
				}
				logrus.Info("Restricting automount for serviceAccounts complete")
			}()
		}
		return nil
	}
}
