package hosted

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/k3s-io/k3s/pkg/clientaccess"
	"github.com/k3s-io/k3s/pkg/cluster/managed"
	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/etcd"
	pkgerrors "github.com/pkg/errors"
)

func (h *HostedConfig) SetControlConfig(config *config.Control) error {
	nameFile := filepath.Join(h.DataDir, "server", "db", "etcd", "name")
	if err := os.MkdirAll(filepath.Dir(nameFile), 0700); err != nil {
		return nil
	}
	if err := os.WriteFile(nameFile, []byte(h.Name+"-etcd-0"), 0600); err != nil {
		return err
	}

	h.config = config
	h.config.ClusterInit = true
	h.config.PrivateIP = h.Name + "-etcd-0." + h.Name + "-etcd"
	if h.Domain != "" {
		h.config.PrivateIP = fmt.Sprintf("%s.%s.svc.%s", h.config.PrivateIP, h.namespace, h.Domain)
	}

	h.etcd = etcd.NewETCD()
	return h.etcd.SetControlConfig(h.config)
}

func (h *HostedConfig) IsInitialized() (bool, error) {
	return false, nil
}

func (h *HostedConfig) Register(handler http.Handler) (http.Handler, error) {
	h.config.Datastore.Endpoint = "https://" + h.Name + "-etcd:2379"
	h.config.Runtime.EtcdConfig.Endpoints = []string{h.config.Datastore.Endpoint}
	return h.etcd.Register(handler)
}

func (h *HostedConfig) Reset(ctx context.Context, wg *sync.WaitGroup, reboostrap func() error) error {
	return pkgerrors.WithMessage(errNotImplemented, "Reset")
}

func (h *HostedConfig) IsReset() (bool, error) {
	return false, nil
}

func (h *HostedConfig) ResetFile() string {
	return ""
}

func (h *HostedConfig) Start(ctx context.Context, wg *sync.WaitGroup, clientAccessInfo *clientaccess.Info) error {
	return h.etcd.Start(ctx, wg, clientAccessInfo)
}

func (h *HostedConfig) Restore(ctx context.Context) error {
	return pkgerrors.WithMessage(errNotImplemented, "Restore")
}

func (h *HostedConfig) EndpointName() string {
	return h.Name + "-etcd"
}

func (h *HostedConfig) Snapshot(ctx context.Context) (*managed.SnapshotResult, error) {
	return nil, pkgerrors.WithMessage(errNotImplemented, "Snapshot")
}

func (h *HostedConfig) ReconcileSnapshotData(ctx context.Context) error {
	return h.etcd.ReconcileSnapshotData(ctx)
}

func (h *HostedConfig) GetMembersClientURLs(ctx context.Context) ([]string, error) {
	return h.etcd.GetMembersClientURLs(ctx)
}

func (h *HostedConfig) RemoveSelf(ctx context.Context) error {
	return pkgerrors.WithMessage(errNotImplemented, "RemoveSelf")
}
