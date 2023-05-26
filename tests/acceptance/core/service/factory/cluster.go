package factory

import (
	"fmt"
	"path/filepath"
	"strconv"

	"github.com/gruntwork-io/terratest/modules/terraform"
	"github.com/rancher/rke2/tests/acceptance/shared/util"

	. "github.com/onsi/ginkgo/v2"
)

func BuildCluster(g GinkgoTInterface, destroy bool) (string, error) {
	tfDir, err := filepath.Abs(util.BasePath() + "/modules")
	if err != nil {
		return "", err
	}

	varDir, err := filepath.Abs(util.BasePath() + "/modules/config/local.tfvars")
	if err != nil {
		return "", err
	}

	terraformOptions := terraform.Options{
		TerraformDir: tfDir,
		VarFiles:     []string{varDir},
	}

	util.NumServers, err = strconv.Atoi(terraform.GetVariableAsStringFromVarFile(g, varDir,
		"no_of_server_nodes"))
	if err != nil {
		return "", err
	}
	util.NumAgents, err = strconv.Atoi(terraform.GetVariableAsStringFromVarFile(g, varDir,
		"no_of_worker_nodes"))
	if err != nil {
		return "", err
	}

	splitRoles := terraform.GetVariableAsStringFromVarFile(g, varDir, "split_roles")
	if splitRoles == "true" {
		etcdNodes, err := strconv.Atoi(terraform.GetVariableAsStringFromVarFile(g, varDir,
			"etcd_only_nodes"))
		if err != nil {
			return "", err
		}
		etcdCpNodes, err := strconv.Atoi(terraform.GetVariableAsStringFromVarFile(g, varDir,
			"etcd_cp_nodes"))
		if err != nil {
			return "", err
		}
		etcdWorkerNodes, err := strconv.Atoi(terraform.GetVariableAsStringFromVarFile(g, varDir,
			"etcd_worker_nodes"))
		if err != nil {
			return "", err
		}
		cpNodes, err := strconv.Atoi(terraform.GetVariableAsStringFromVarFile(g, varDir,
			"cp_only_nodes"))
		if err != nil {
			return "", err
		}
		cpWorkerNodes, err := strconv.Atoi(terraform.GetVariableAsStringFromVarFile(g, varDir,
			"cp_worker_nodes"))
		if err != nil {
			return "", err
		}
		util.NumServers = util.NumServers + etcdNodes + etcdCpNodes + etcdWorkerNodes +
			+cpNodes + cpWorkerNodes
	}

	util.AwsUser = terraform.GetVariableAsStringFromVarFile(g, varDir, "aws_user")
	util.AccessKey = terraform.GetVariableAsStringFromVarFile(g, varDir, "access_key")
	fmt.Printf("\nCreating Cluster")

	if destroy {
		fmt.Printf("Cluster is being deleted")
		terraform.Destroy(g, &terraformOptions)
		return "cluster destroyed", err
	}

	terraform.InitAndApply(g, &terraformOptions)
	util.KubeConfigFile = terraform.Output(g, &terraformOptions, "kubeconfig")
	util.ServerIPs = terraform.Output(g, &terraformOptions, "master_ips")
	util.AgentIPs = terraform.Output(g, &terraformOptions, "worker_ips")

	return "cluster created", nil
}
