package terraform

import (
	"fmt"

	"path/filepath"
	"strconv"
	"testing"

	"github.com/gruntwork-io/terratest/modules/terraform"
)

var (
	KubeConfigFile string
	MasterIPs      string
	WorkerIPs      string
	NumServers     int
	NumWorkers     int
	AwsUser        string
	AccessKey      string
)

func BuildCluster(t *testing.T, tfVarsPath string, destroy bool) (string, error) {
	basepath := GetBasepath()
	tfDir, err := filepath.Abs(basepath + "/tests/terraform/modules")
	if err != nil {
		return "", err
	}
	varDir, err := filepath.Abs(basepath + tfVarsPath)
	if err != nil {
		return "", err
	}
	TerraformOptions := &terraform.Options{
		TerraformDir: tfDir,
		VarFiles:     []string{varDir},
	}
	NumServers, _ = strconv.Atoi(terraform.GetVariableAsStringFromVarFile(t, varDir, "no_of_server_nodes"))
	NumWorkers, _ = strconv.Atoi(terraform.GetVariableAsStringFromVarFile(t, varDir, "no_of_worker_nodes"))
	splitRoles := terraform.GetVariableAsStringFromVarFile(t, varDir, "split_roles")
	if splitRoles == "true" {
		etcdNodes, _ := strconv.Atoi(terraform.GetVariableAsStringFromVarFile(t, varDir, "etcd_only_nodes"))
		etcdCpNodes, _ := strconv.Atoi(terraform.GetVariableAsStringFromVarFile(t, varDir, "etcd_cp_nodes"))
		etcdWorkerNodes, _ := strconv.Atoi(terraform.GetVariableAsStringFromVarFile(t, varDir, "etcd_worker_nodes"))
		cpNodes, _ := strconv.Atoi(terraform.GetVariableAsStringFromVarFile(t, varDir, "cp_only_nodes"))
		cpWorkerNodes, _ := strconv.Atoi(terraform.GetVariableAsStringFromVarFile(t, varDir, "cp_worker_nodes"))
		NumServers = NumServers + etcdNodes + etcdCpNodes + etcdWorkerNodes + cpNodes + cpWorkerNodes
	}
	AwsUser = terraform.GetVariableAsStringFromVarFile(t, varDir, "aws_user")
	AccessKey = terraform.GetVariableAsStringFromVarFile(t, varDir, "access_key")

	if destroy {
		fmt.Printf("Cluster is being deleted")
		terraform.Destroy(t, TerraformOptions)
		return "cluster destroyed", err
	}

	fmt.Printf("Creating Cluster")
	terraform.InitAndApply(t, TerraformOptions)
	KubeConfigFile = terraform.Output(t, TerraformOptions, "kubeconfig")
	MasterIPs = terraform.Output(t, TerraformOptions, "master_ips")
	WorkerIPs = terraform.Output(t, TerraformOptions, "worker_ips")
	return "cluster created", err
}
