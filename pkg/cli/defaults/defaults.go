package defaults

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/k3s-io/k3s/pkg/cli/cmds"
	pkgerrors "github.com/pkg/errors"
	"github.com/urfave/cli/v2"
	"google.golang.org/grpc/grpclog"
)

func Set(_ *cli.Context, dataDir string) error {
	if err := createDataDir(dataDir, 0755); err != nil {
		return pkgerrors.WithMessagef(err, "failed to create directory %s", dataDir)
	}

	logsDir := filepath.Join(dataDir, "agent", "logs")
	if err := os.MkdirAll(logsDir, 0750); err != nil {
		return pkgerrors.WithMessagef(err, "failed to create directory %s", logsDir)
	}

	cmds.ServerConfig.ClusterInit = true
	cmds.ServerConfig.DisableNPC = true
	cmds.ServerConfig.FlannelBackend = "none"
	cmds.ServerConfig.AdvertisePort = 6443
	cmds.ServerConfig.SupervisorPort = 9345
	cmds.ServerConfig.HTTPSPort = 6443
	cmds.ServerConfig.APIServerPort = 6443
	cmds.ServerConfig.APIServerBindAddress = "0.0.0.0"
	if err := AppendToStringSlice(&cmds.ServerConfig.ExtraAPIArgs, []string{
		"enable-admission-plugins=NodeRestriction",
	}); err != nil {
		return err
	}
	if err := AppendToStringSlice(&cmds.AgentConfig.ExtraKubeletArgs, []string{
		"stderrthreshold=FATAL",
		"log-file-max-size=50",
		"alsologtostderr=false",
		"logtostderr=false",
		"log-file=" + filepath.Join(logsDir, "kubelet.log"),
	}); err != nil {
		return err
	}
	if !cmds.Debug {
		l := grpclog.NewLoggerV2(ioutil.Discard, ioutil.Discard, os.Stderr)
		grpclog.SetLoggerV2(l)
	}

	return nil
}

// With urfaveCLI/v2, we cannot directly access the []string value of a cli.StringSlice
// so we need to individually append each value to the slice using the Set method
func AppendToStringSlice(ss *cli.StringSlice, values []string) error {
	for _, v := range values {
		if err := ss.Set(v); err != nil {
			return err
		}
	}
	return nil
}
