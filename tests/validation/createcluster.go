package validation

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/gruntwork-io/terratest/modules/terraform"
)

func BuildCluster(ctype string, t *testing.T, destroy bool) (string, string, string) {
	var tDir string
	var vDir string
	tDir = "./terraform/modules/rke2cluster"
	vDir = "./terraform/modules/rke2cluster/" + ctype + ".tfvars"

	tfDir, _ := filepath.Abs(tDir)
	varDir, _ := filepath.Abs(vDir)

	TerraformOptions := &terraform.Options{
		TerraformDir: tfDir,
		VarFiles:     []string{varDir},
	}
	if destroy {
		fmt.Printf("Destroying Cluster")
		terraform.Destroy(t, TerraformOptions)
		return "", "", ""
	}

	fmt.Printf("Creating Cluster")
	terraform.InitAndApply(t, TerraformOptions)
	kubeconfig := terraform.Output(t, TerraformOptions, "kubeconfig")
	master_ips := terraform.Output(t, TerraformOptions, "master_ips")
	worker_ips := terraform.Output(t, TerraformOptions, "worker_ips")

	kubeconfigfile := "/tmp" + kubeconfig
	return kubeconfigfile, master_ips, worker_ips
}
