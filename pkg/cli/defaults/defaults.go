package defaults

import (
	"github.com/rancher/k3s/pkg/cli/cmds"
	"github.com/rancher/rke2/pkg/images"
)

func Set(images images.Images) {
	cmds.ServerConfig.DatastoreEndpoint = "http://localhost:2379"
	cmds.ServerConfig.DisableCCM = true
	cmds.ServerConfig.DisableNPC = true
	cmds.ServerConfig.FlannelBackend = "none"
	cmds.ServerConfig.AdvertisePort = 6443
	cmds.ServerConfig.SupervisorPort = 9345
	cmds.ServerConfig.HTTPSPort = 6443
	cmds.ServerConfig.APIServerPort = 6443
	cmds.ServerConfig.DisableKubeProxy = true
	cmds.AgentConfig.PauseImage = images.Pause
	cmds.AgentConfig.NoFlannel = true
}
