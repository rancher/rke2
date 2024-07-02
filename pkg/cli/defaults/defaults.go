package defaults

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/k3s-io/k3s/pkg/cli/cmds"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
	"google.golang.org/grpc/grpclog"
)

func Set(_ *cli.Context, dataDir string) error {
	if err := createDataDir(dataDir, 0755); err != nil {
		return errors.Wrapf(err, "failed to create directory %s", dataDir)
	}

	logsDir := filepath.Join(dataDir, "agent", "logs")
	if err := os.MkdirAll(logsDir, 0750); err != nil {
		return errors.Wrapf(err, "failed to create directory %s", logsDir)
	}

	cmds.ServerConfig.ClusterInit = true
	cmds.ServerConfig.DisableNPC = true
	cmds.ServerConfig.FlannelBackend = "none"
	cmds.ServerConfig.AdvertisePort = 6443
	cmds.ServerConfig.SupervisorPort = 9345
	cmds.ServerConfig.HTTPSPort = 6443
	cmds.ServerConfig.APIServerPort = 6443
	cmds.ServerConfig.APIServerBindAddress = "0.0.0.0"
	cmds.ServerConfig.ExtraAPIArgs = append(
		[]string{
			"enable-admission-plugins=NodeRestriction",
		},
		cmds.ServerConfig.ExtraAPIArgs...)
	cmds.AgentConfig.ExtraKubeletArgs = append(
		[]string{
			"stderrthreshold=FATAL",
			"log-file-max-size=50",
			"alsologtostderr=false",
			"logtostderr=false",
			"log-file=" + filepath.Join(logsDir, "kubelet.log"),
		},
		cmds.AgentConfig.ExtraKubeletArgs...)

	if !cmds.Debug {
		l := grpclog.NewLoggerV2(ioutil.Discard, ioutil.Discard, os.Stderr)
		grpclog.SetLoggerV2(l)
	}

	return nil
}
