package factory

import (
	"fmt"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/gruntwork-io/terratest/modules/terraform"
	"github.com/rancher/rke2/tests/acceptance/shared"

	. "github.com/onsi/ginkgo/v2"
)

type Cluster struct {
	Status     		string
	ServerIPs  		string
	AgentIPs   		string
	WinAgentIPs		string
	NumServers 		int
	NumAgents  		int
	NumWinAgents  	int
}

var (
	once    sync.Once
	cluster *Cluster
)

// NewCluster creates a new cluster and returns his values from terraform config and vars
func NewCluster(g GinkgoTInterface) (*Cluster, error) {
	tfDir, err := filepath.Abs(shared.BasePath() + "/acceptance/modules")
	if err != nil {
		return nil, err
	}

	varDir, err := filepath.Abs(shared.BasePath() + "/acceptance/modules/config/local.tfvars")
	if err != nil {
		return nil, err
	}

	terraformOptions := &terraform.Options{
		TerraformDir: tfDir,
		VarFiles:     []string{varDir},
	}

	NumServers, err := strconv.Atoi(terraform.GetVariableAsStringFromVarFile(g, varDir, "no_of_server_nodes"))
	if err != nil {
		return nil, err
	}

	NumAgents, err := strconv.Atoi(terraform.GetVariableAsStringFromVarFile(g, varDir, "no_of_worker_nodes"))
	if err != nil {
		return nil, err
	}

	NumWinAgents, err := strconv.Atoi(terraform.GetVariableAsStringFromVarFile(g, varDir, "no_of_windows_worker_nodes"))
	if err != nil {
		return nil, err
	}

	splitRoles := terraform.GetVariableAsStringFromVarFile(g, varDir, "split_roles")
	if splitRoles == "true" {
		etcdNodes, err := strconv.Atoi(terraform.GetVariableAsStringFromVarFile(g, varDir,
			"etcd_only_nodes"))
		if err != nil {
			return nil, err
		}
		etcdCpNodes, err := strconv.Atoi(terraform.GetVariableAsStringFromVarFile(g, varDir,
			"etcd_cp_nodes"))
		if err != nil {
			return nil, err
		}
		etcdWorkerNodes, err := strconv.Atoi(terraform.GetVariableAsStringFromVarFile(g, varDir,
			"etcd_worker_nodes"))
		if err != nil {
			return nil, err
		}
		cpNodes, err := strconv.Atoi(terraform.GetVariableAsStringFromVarFile(g, varDir,
			"cp_only_nodes"))
		if err != nil {
			return nil, err
		}
		cpWorkerNodes, err := strconv.Atoi(terraform.GetVariableAsStringFromVarFile(g, varDir,
			"cp_worker_nodes"))
		if err != nil {
			return nil, err
		}
		NumServers = NumServers + etcdNodes + etcdCpNodes + etcdWorkerNodes +
			+cpNodes + cpWorkerNodes
	}

	fmt.Println("Creating Cluster")

	terraform.InitAndApply(g, terraformOptions)

	ServerIPs := terraform.Output(g, terraformOptions, "master_ips")
	AgentIPs := terraform.Output(g, terraformOptions, "worker_ips")
	WinAgentIPs := terraform.Output(g, terraformOptions, "windows_worker_ips")

	shared.AwsUser = terraform.GetVariableAsStringFromVarFile(g, varDir, "aws_user")
	shared.AccessKey = terraform.GetVariableAsStringFromVarFile(g, varDir, "access_key")
	shared.KubeConfigFile = terraform.Output(g, terraformOptions, "kubeconfig")
	return &Cluster{
		Status:     "cluster created",
		ServerIPs:  ServerIPs,
		AgentIPs:   AgentIPs,
		WinAgentIPs: WinAgentIPs,
		NumServers: NumServers,
		NumAgents:  NumAgents,
		NumWinAgents: NumWinAgents,
	}, nil
}

// GetCluster returns a singleton cluster
func GetCluster(g GinkgoTInterface) *Cluster {
	var err error

	once.Do(func() {
		cluster, err = NewCluster(g)
		if err != nil {
			g.Errorf("error getting cluster: %v", err)
		}
	})
	return cluster
}

// DestroyCluster destroys the cluster and returns a message
func DestroyCluster(g GinkgoTInterface) (string, error) {
	basepath := shared.BasePath()
	tfDir, err := filepath.Abs(basepath + "/modules")
	if err != nil {
		return "", err
	}
	varDir, err := filepath.Abs(basepath + "/modules/config/local.tfvars")
	if err != nil {
		return "", err
	}

	terraformOptions := terraform.Options{
		TerraformDir: tfDir,
		VarFiles:     []string{varDir},
	}
	terraform.Destroy(g, &terraformOptions)

	return "cluster destroyed", nil
}
