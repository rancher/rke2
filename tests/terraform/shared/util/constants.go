package util

import "flag"

var (
	Destroy        = flag.Bool("destroy", false, "a bool")
	UpgradeVersion = flag.String("upgradeVersion", "", "Version to upgrade the cluster to")

	KubeConfigFile string
	ServerIPs      string
	AgentIPs       string
	NumServers     int
	NumAgents      int
	AwsUser        string
	AccessKey      string
	ModulesPath    = "/modules"
	TfVarsPath     = "/modules/config/local.tfvars"
)
