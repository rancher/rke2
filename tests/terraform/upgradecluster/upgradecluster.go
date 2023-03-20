package upgradecluster

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/rancher/rke2/tests/terraform"
)

func upgradeCluster(version string, kubeconfig string) error {
	if strings.TrimSpace(version) == "" {
		return fmt.Errorf("please provide a non-empty rke2 version to upgrade to")
	}
	regex := regexp.MustCompile(`\+`)
	sucVersion := regex.ReplaceAllString(version, "-")
	originalFilePath := terraform.Basepath() + "/tests/terraform/resource_files" + "/upgrade-plan.yaml"
	newFilePath := terraform.Basepath() + "/tests/terraform/resource_files" + "/plan.yaml"
	content, err := os.ReadFile(originalFilePath)
	if err != nil {
		return err
	}
	newContent := strings.ReplaceAll(string(content), "$UPGRADEVERSION", sucVersion)
	os.WriteFile(newFilePath, []byte(newContent), 0777)
	_, err = terraform.DeployWorkload("plan.yaml", kubeconfig)
	return err
}
