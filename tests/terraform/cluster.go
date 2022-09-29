package terraform

import (
	"fmt"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/gruntwork-io/terratest/modules/terraform"
)

var (
	kubeConfigFile string
	masterIPs      string
	workerIPs      string
	numServers     int
	numWorkers     int
	awsUser        string
	accessKey      string
)

func buildCluster(t *testing.T, tfVarsPath string, destroy bool) (string, error) {
	tfDir, err := filepath.Abs(basepath() + "/tests/terraform/modules")
	if err != nil {
		return "", err
	}
	varDir, err := filepath.Abs(basepath() + tfVarsPath)
	if err != nil {
		return "", err
	}
	terraformOptions := &terraform.Options{
		TerraformDir: tfDir,
		VarFiles:     []string{varDir},
	}
	numServers, err = strconv.Atoi(terraform.GetVariableAsStringFromVarFile(t, varDir, "no_of_server_nodes"))
	if err != nil {
		return "", err
	}
	numWorkers, err = strconv.Atoi(terraform.GetVariableAsStringFromVarFile(t, varDir, "no_of_worker_nodes"))
	if err != nil {
		return "", err
	}
	splitRoles := terraform.GetVariableAsStringFromVarFile(t, varDir, "split_roles")
	if splitRoles == "true" {
		etcdNodes, err := strconv.Atoi(terraform.GetVariableAsStringFromVarFile(t, varDir, "etcd_only_nodes"))
		if err != nil {
			return "", err
		}
		etcdCpNodes, err := strconv.Atoi(terraform.GetVariableAsStringFromVarFile(t, varDir, "etcd_cp_nodes"))
		if err != nil {
			return "", err
		}
		etcdWorkerNodes, err := strconv.Atoi(terraform.GetVariableAsStringFromVarFile(t, varDir, "etcd_worker_nodes"))
		if err != nil {
			return "", err
		}
		cpNodes, err := strconv.Atoi(terraform.GetVariableAsStringFromVarFile(t, varDir, "cp_only_nodes"))
		if err != nil {
			return "", err
		}
		cpWorkerNodes, err := strconv.Atoi(terraform.GetVariableAsStringFromVarFile(t, varDir, "cp_worker_nodes"))
		if err != nil {
			return "", err
		}
		numServers = numServers + etcdNodes + etcdCpNodes + etcdWorkerNodes + cpNodes + cpWorkerNodes
	}
	awsUser = terraform.GetVariableAsStringFromVarFile(t, varDir, "aws_user")
	accessKey = terraform.GetVariableAsStringFromVarFile(t, varDir, "access_key")

	if destroy {
		fmt.Printf("Cluster is being deleted")
		terraform.Destroy(t, terraformOptions)
		return "cluster destroyed", nil
	}

	fmt.Printf("Creating Cluster")
	terraform.InitAndApply(t, terraformOptions)
	kubeConfigFile = terraform.Output(t, terraformOptions, "kubeconfig")
	masterIPs = terraform.Output(t, terraformOptions, "master_ips")
	workerIPs = terraform.Output(t, terraformOptions, "worker_ips")
	return "cluster created", nil
}
