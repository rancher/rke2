package createcluster

import (
	"fmt"

	"path/filepath"
	"strconv"
	"testing"

	"github.com/gruntwork-io/terratest/modules/terraform"
	tf "github.com/rancher/rke2/tests/terraform"
)

var (
	KubeConfigFile string
	MasterIPs      string
	WorkerIPs      string
	NumServers     int
	NumWorkers     int
	AwsUser        string
	AccessKey      string
	modulesPath    = "/tests/terraform/modules"
	tfVarsPath     = "/tests/terraform/modules/config/local.tfvars"
)

func BuildCluster(t *testing.T, destroy bool) (string, error) {
	tfDir, err := filepath.Abs(tf.Basepath() + modulesPath)
	if err != nil {
		return "", err
	}

	varDir, err := filepath.Abs(tf.Basepath() + tfVarsPath)
	if err != nil {
		return "", err
	}
	terraformOptions := terraform.Options{
		TerraformDir: tfDir,
		VarFiles:     []string{varDir},
	}

	NumServers, err = strconv.Atoi(terraform.GetVariableAsStringFromVarFile(t, varDir, "no_of_server_nodes"))
	if err != nil {
		return "", err
	}
	NumWorkers, err = strconv.Atoi(terraform.GetVariableAsStringFromVarFile(t, varDir, "no_of_worker_nodes"))
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
		NumServers = NumServers + etcdNodes + etcdCpNodes + etcdWorkerNodes + cpNodes + cpWorkerNodes
	}
	AwsUser = terraform.GetVariableAsStringFromVarFile(t, varDir, "aws_user")
	AccessKey = terraform.GetVariableAsStringFromVarFile(t, varDir, "access_key")

	if destroy {
		fmt.Printf("Cluster is being deleted")
		terraform.Destroy(t, &terraformOptions)
		return "cluster destroyed", nil
	}

	fmt.Printf("Creating Cluster")

	terraform.InitAndApply(t, &terraformOptions)
	KubeConfigFile = terraform.Output(t, &terraformOptions, "kubeconfig")
	MasterIPs = terraform.Output(t, &terraformOptions, "master_ips")
	WorkerIPs = terraform.Output(t, &terraformOptions, "worker_ips")

	return "cluster created", nil
}
