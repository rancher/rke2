package defaults

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"google.golang.org/grpc/grpclog"

	"github.com/pkg/errors"

	"github.com/rancher/k3s/pkg/cli/cmds"
	"github.com/rancher/rke2/pkg/images"
)

func Set(images images.Images, dataDir string) error {
	logsDir := filepath.Join(dataDir, "agent", "logs")
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		return errors.Wrapf(err, "failed to create directory %s", logsDir)
	}

	cmds.ServerConfig.DatastoreEndpoint = "etcd"
	cmds.ServerConfig.DisableCCM = true
	cmds.ServerConfig.DisableNPC = true
	cmds.ServerConfig.FlannelBackend = "none"
	cmds.ServerConfig.AdvertisePort = 6443
	cmds.ServerConfig.SupervisorPort = 9345
	cmds.ServerConfig.HTTPSPort = 6443
	cmds.ServerConfig.APIServerPort = 6443
	cmds.ServerConfig.APIServerBindAddress = "0.0.0.0"
	cmds.ServerConfig.DisableKubeProxy = true
	cmds.AgentConfig.PauseImage = images.Pause
	cmds.AgentConfig.NoFlannel = true
	cmds.AgentConfig.ExtraKubeletArgs = append(cmds.AgentConfig.ExtraKubeletArgs,
		"stderrthreshold=FATAL",
		"log-file-max-size=50",
		"alsologtostderr=false",
		"logtostderr=false", "log-file="+filepath.Join(logsDir, "kubelet.log"))

	if !cmds.Debug {
		l := grpclog.NewLoggerV2(ioutil.Discard, ioutil.Discard, os.Stderr)
		grpclog.SetLoggerV2(l)
	}

	return nil
}
