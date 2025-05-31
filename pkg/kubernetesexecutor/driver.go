package kubernetesexecutor

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/k3s-io/k3s/pkg/clientaccess"
	"github.com/k3s-io/k3s/pkg/cluster/managed"
	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/etcd"
	pkgerrors "github.com/pkg/errors"
)

func (k *KubernetesConfig) SetControlConfig(config *config.Control) error {
	nameFile := filepath.Join(k.DataDir, "server", "db", "etcd", "name")
	if err := os.MkdirAll(filepath.Dir(nameFile), 0700); err != nil {
		return nil
	}
	if err := os.WriteFile(nameFile, []byte(k.Name+"-etcd-0"), 0600); err != nil {
		return err
	}

	k.config = config
	k.config.ClusterInit = true
	k.config.PrivateIP = k.Name + "-etcd-0." + k.Name + "-etcd"
	if k.Domain != "" {
		k.config.PrivateIP = fmt.Sprintf("%s.%s.svc.%s", k.config.PrivateIP, k.namespace, k.Domain)
	}

	k.etcd = etcd.NewETCD()
	return k.etcd.SetControlConfig(k.config)
}

func (k *KubernetesConfig) IsInitialized() (bool, error) {
	return false, nil
}

func (k *KubernetesConfig) Register(handler http.Handler) (http.Handler, error) {
	k.config.Datastore.Endpoint = "https://" + k.Name + "-etcd:2379"
	k.config.Runtime.EtcdConfig.Endpoints = []string{k.config.Datastore.Endpoint}
	return k.etcd.Register(handler)
}

func (k *KubernetesConfig) Reset(ctx context.Context, reboostrap func() error) error {
	return pkgerrors.WithMessage(errNotImplemented, "Reset")
}

func (k *KubernetesConfig) IsReset() (bool, error) {
	return false, nil
}

func (k *KubernetesConfig) ResetFile() string {
	return ""
}

func (k *KubernetesConfig) Start(ctx context.Context, clientAccessInfo *clientaccess.Info) error {
	return k.etcd.Start(ctx, clientAccessInfo)
}

func (k *KubernetesConfig) Restore(ctx context.Context) error {
	return pkgerrors.WithMessage(errNotImplemented, "Restore")
}

func (k *KubernetesConfig) EndpointName() string {
	return k.Name + "-etcd"
}

func (k *KubernetesConfig) Snapshot(ctx context.Context) (*managed.SnapshotResult, error) {
	return nil, pkgerrors.WithMessage(errNotImplemented, "Snapshot")
}

func (k *KubernetesConfig) ReconcileSnapshotData(ctx context.Context) error {
	return k.etcd.ReconcileSnapshotData(ctx)
}

func (k *KubernetesConfig) GetMembersClientURLs(ctx context.Context) ([]string, error) {
	return k.etcd.GetMembersClientURLs(ctx)
}

func (k *KubernetesConfig) RemoveSelf(ctx context.Context) error {
	return pkgerrors.WithMessage(errNotImplemented, "RemoveSelf")
}
