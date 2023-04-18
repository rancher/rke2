package factory

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/gruntwork-io/terratest/modules/terraform"
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/rancher/rke2/tests/terraform/core/assert"
	"github.com/rancher/rke2/tests/terraform/shared/util"
)

func BuildCluster(t ginkgo.GinkgoTInterface, destroy bool) (string, error) {
	tfDir, err := filepath.Abs(util.Basepath() + util.ModulesPath)
	if err != nil {
		return "", err
	}

	varDir, err := filepath.Abs(util.Basepath() + util.TfVarsPath)
	if err != nil {
		return "", err
	}
	terraformOptions := terraform.Options{
		TerraformDir: tfDir,
		VarFiles:     []string{varDir},
	}

	util.NumServers, err = strconv.Atoi(terraform.GetVariableAsStringFromVarFile(t, varDir,
		"no_of_server_nodes"))
	if err != nil {
		return "", err
	}
	util.NumAgents, err = strconv.Atoi(terraform.GetVariableAsStringFromVarFile(t, varDir,
		"no_of_worker_nodes"))
	if err != nil {
		return "", err
	}

	splitRoles := terraform.GetVariableAsStringFromVarFile(t, varDir, "split_roles")
	if splitRoles == "true" {
		etcdNodes, err := strconv.Atoi(terraform.GetVariableAsStringFromVarFile(t, varDir,
			"etcd_only_nodes"))
		if err != nil {
			return "", err
		}
		etcdCpNodes, err := strconv.Atoi(terraform.GetVariableAsStringFromVarFile(t, varDir,
			"etcd_cp_nodes"))
		if err != nil {
			return "", err
		}
		etcdWorkerNodes, err := strconv.Atoi(terraform.GetVariableAsStringFromVarFile(t, varDir,
			"etcd_worker_nodes"))
		if err != nil {
			return "", err
		}
		cpNodes, err := strconv.Atoi(terraform.GetVariableAsStringFromVarFile(t, varDir,
			"cp_only_nodes"))
		if err != nil {
			return "", err
		}
		cpWorkerNodes, err := strconv.Atoi(terraform.GetVariableAsStringFromVarFile(t, varDir,
			"cp_worker_nodes"))
		if err != nil {
			return "", err
		}
		util.NumServers = util.NumServers + etcdNodes + etcdCpNodes + etcdWorkerNodes +
			+cpNodes + cpWorkerNodes
	}

	util.AwsUser = terraform.GetVariableAsStringFromVarFile(t, varDir, "aws_user")
	util.AccessKey = terraform.GetVariableAsStringFromVarFile(t, varDir, "access_key")

	if destroy {
		fmt.Printf("Cluster is being deleted")
		terraform.Destroy(t, &terraformOptions)
		return "cluster destroyed", nil
	}

	fmt.Printf("Creating Cluster")

	terraform.InitAndApply(t, &terraformOptions)
	util.KubeConfigFile = terraform.Output(t, &terraformOptions, "kubeconfig")
	util.ServerIPs = terraform.Output(t, &terraformOptions, "master_ips")
	util.AgentIPs = terraform.Output(t, &terraformOptions, "worker_ips")

	return "cluster created", nil
}

func UpgradeClusterSUC(version string) error {
	if strings.TrimSpace(version) == "" {
		return fmt.Errorf("please provide a non-empty rke2 version to upgrade to")
	}

	_, err := util.ManageWorkload("create", "suc.yaml")
	gomega.Expect(err).NotTo(gomega.HaveOccurred(),
		"system-upgrade-controller manifest did not deploy successfully")

	assert.CheckComponentCmdHost("kubectl get pods "+"-n system-upgrade --kubeconfig="+
		util.KubeConfigFile, "system-upgrade-controller", "Running")

	regex := regexp.MustCompile(`\+`)
	sucVersion := regex.ReplaceAllString(version, "-")

	originalFilePath := util.Basepath() + "/shared/workloads" + "/upgrade-plan.yaml"
	newFilePath := util.Basepath() + "/shared/workloads" + "/plan.yaml"

	content, err := os.ReadFile(originalFilePath)
	if err != nil {
		return err
	}
	newContent := strings.ReplaceAll(string(content), "$UPGRADEVERSION", sucVersion)
	err = os.WriteFile(newFilePath, []byte(newContent), 0644)
	if err != nil {
		return err
	}

	_, err = util.ManageWorkload("create", "plan.yaml")
	gomega.Expect(err).NotTo(gomega.HaveOccurred(), "failed to upgrade cluster.")

	return err
}
