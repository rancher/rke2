package e2e

import (
	"flag"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/gruntwork-io/terratest/modules/terraform"
)

var destroy = flag.Bool("destroy", false, "a bool")
var nodeOs = flag.String("node_os", "centos", "a string")
var installMode = flag.String("install_mode", "rke2", "a string")
var resourceName = flag.String("resource_name", "", "a string")
var sshuser = flag.String("sshuser", "ubuntu", "a string")
var sshkey = flag.String("sshkey", "", "a string")

var (
	kubeconfig string
	masterIPs  string
	workerIPs  string
)

// BuildCluster method is used to create the cluster for various  environments
// nodeOs: examples - centos7, centos8, rhel7, rhel8, sles, ubuntu
// destroy: set to false to create cluster, true to delete cluster
// resourceName name assigned to AWS resources

func BuildCluster(nodeOs, installMode, resourceName string, t *testing.T, destroy bool) (string, string, string) {
	tDir := "./terraform/modules/rke2cluster"
	vDir := "/tmp/config/" + nodeOs + ".tfvars"

	tfDir, _ := filepath.Abs(tDir)
	varDir, _ := filepath.Abs(vDir)

	TerraformOptions := &terraform.Options{
		TerraformDir: tfDir,
		VarFiles:     []string{varDir},
		Vars: map[string]interface{}{
			"install_mode":  installMode,
			"resource_name": resourceName,
		},
	}
	if destroy {
		fmt.Printf("Destroying Cluster")
		terraform.Destroy(t, TerraformOptions)
		return "", "", ""
	}

	fmt.Printf("Creating Cluster")
	terraform.InitAndApply(t, TerraformOptions)
	kubeconfig := terraform.Output(t, TerraformOptions, "kubeconfig")
	masterIPs := terraform.Output(t, TerraformOptions, "master_ips")
	workerIPs := terraform.Output(t, TerraformOptions, "worker_ips")

	kubeconfigfile := "/tmp/" + kubeconfig
	return kubeconfigfile, masterIPs, workerIPs
}
