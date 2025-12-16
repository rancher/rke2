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
	cmds.ServerConfig.ExtraAPIArgs = PrependToStringSlice(cmds.ServerConfig.ExtraAPIArgs, []string{
		"enable-admission-plugins=NodeRestriction",
	})
	cmds.AgentConfig.ExtraKubeletArgs = PrependToStringSlice(cmds.AgentConfig.ExtraKubeletArgs, []string{
		"stderrthreshold=FATAL",
		"log-file-max-size=50",
		"alsologtostderr=false",
		"logtostderr=false",
		"log-file=" + filepath.Join(logsDir, "kubelet.log"),
	})
	if !cmds.Debug {
		l := grpclog.NewLoggerV2(ioutil.Discard, ioutil.Discard, os.Stderr)
		grpclog.SetLoggerV2(l)
	}

	return nil
}

// With urfaveCLI/v2, we cannot directly access the internal []string of a cli.StringSlice
// so we create a new []string with the values we want to prepend
// and replace the original StringSlice with the new one.
func PrependToStringSlice(ss cli.StringSlice, values []string) cli.StringSlice {
	values = append(values, ss.Value()...)
	return *cli.NewStringSlice(values...)
}
