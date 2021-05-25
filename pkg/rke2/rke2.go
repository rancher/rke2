package rke2

import (
	"context"
	"github.com/rancher/rke2/pkg/images"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1alpha2"
	"os/exec"
	"path/filepath"
)

type Config struct {
	AuditPolicyFile     string
	CloudProviderConfig string
	CloudProviderName   string
	Images              images.ImageOverrideConfig
	KubeletPath         string
}

const (
	CISProfile15           = "cis-1.5"
	CISProfile16           = "cis-1.6"
	defaultAuditPolicyFile = "/etc/rancher/rke2/audit-policy.yaml"
)

func startContainerd(dataDir string, errChan chan error, tmpAddress string, cmd *exec.Cmd) {
	args := []string{
		"-a", tmpAddress,
		"--root", filepath.Join(dataDir, "agent", "containerd"),
	}
	cmd.Args = append(cmd.Args, args...)
	cmd.Stdout = nil
	cmd.Stderr = nil

	errChan <- cmd.Run()
}

func isContainerRunning(ctx context.Context, name string, resp *runtimeapi.ListContainersResponse) bool {
	for _, c := range resp.Containers {
		if c.Metadata.Name == name {
			return true
		}
	}
	return false
}
