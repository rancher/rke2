package testcase

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/rancher/rke2/tests/acceptance/core/service/assert"
	"github.com/rancher/rke2/tests/acceptance/core/service/customflag"
	"github.com/rancher/rke2/tests/acceptance/core/service/factory"
	"github.com/rancher/rke2/tests/acceptance/shared"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// TestUpgradeClusterSUC upgrades cluster using the system-upgrade-controller.
func TestUpgradeClusterSUC(version string) error {
	fmt.Printf("\nUpgrading cluster to version: %s\n", version)

	_, err := shared.ManageWorkload("create", "suc.yaml")
	Expect(err).NotTo(HaveOccurred(),
		"system-upgrade-controller manifest did not deploy successfully")

	getPodsSystemUpgrade := "kubectl get pods -n system-upgrade --kubeconfig="
	assert.CheckComponentCmdHost(
		getPodsSystemUpgrade+shared.KubeConfigFile,
		"system-upgrade-controller",
		Running,
	)
	Expect(err).NotTo(HaveOccurred())

	originalFilePath := shared.BasePath() + "/acceptance/workloads" + "/upgrade-plan.yaml"
	newFilePath := shared.BasePath() + "/acceptance/workloads" + "/plan.yaml"

	content, err := os.ReadFile(originalFilePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %s", err)
	}

	newContent := strings.ReplaceAll(string(content), "$UPGRADEVERSION", version)
	err = os.WriteFile(newFilePath, []byte(newContent), 0644)
	if err != nil {
		return fmt.Errorf("failed to write file: %s", err)
	}

	_, err = shared.ManageWorkload("create", "plan.yaml")
	Expect(err).NotTo(HaveOccurred(), "failed to upgrade cluster.")

	return nil
}

// TestUpgradeClusterManually upgrades cluster "manually"
func TestUpgradeClusterManually(version string) error {
	if version == "" {
		return fmt.Errorf("please provide a non-empty rke2 version to upgrade to")
	}
	cluster := factory.GetCluster(GinkgoT())

	serverIPs := strings.Split(cluster.ServerIPs, ",")
	agentIPs := strings.Split(cluster.AgentIPs, ",")

	if cluster.NumServers == 0 && cluster.NumAgents == 0 {
		return fmt.Errorf("no nodes found to upgrade")
	}

	if cluster.NumServers > 0 {
		if err := upgradeServer(version, serverIPs); err != nil {
			return err
		}
	}

	if cluster.NumAgents > 0 {
		if err := upgradeAgent(version, agentIPs); err != nil {
			return err
		}
	}

	return nil
}

// upgradeNode upgrades a node server or agent type to the specified version
func upgradeNode(nodeType string, installType string, ips []string) error {
	var wg sync.WaitGroup
	var channel string
	errCh := make(chan error, len(ips))

	switch {
	case customflag.ServiceFlag.InstallType.Version != nil:
		installType = fmt.Sprintf("INSTALL_RKE2_VERSION=%s", customflag.ServiceFlag.InstallType.Version)
	case customflag.ServiceFlag.InstallType.Commit != nil:
		installType = fmt.Sprintf("INSTALL_RKE2_COMMIT=%s", customflag.ServiceFlag.InstallType.Commit)
	case customflag.ServiceFlag.InstallType.Channel != "":
		channel = fmt.Sprintf("INSTALL_RKE2_CHANNEL=%s", customflag.ServiceFlag.InstallType.Channel)
	case customflag.ServiceFlag.InstallType.Channel == "":
		channel = fmt.Sprintf("INSTALL_RKE2_CHANNEL=%s", "stable")
	}

	installRke2Command := "sudo curl -sfL https://get.rke2.io | sudo %s %s INSTALL_RKE2_TYPE=" + nodeType + " sh - "
	for _, ip := range ips {
		upgradeCommand := fmt.Sprintf(installRke2Command, installType, channel)
		wg.Add(1)
		go func(ip, installFlag string) {
			defer wg.Done()
			defer GinkgoRecover()

			fmt.Println("Upgrading " + nodeType + " to: " + upgradeCommand)
			if _, err := shared.RunCommandOnNode(upgradeCommand, ip); err != nil {
				fmt.Printf("\nError upgrading %s %s: %v\n\n", nodeType, ip, err)
				errCh <- err
				close(errCh)
				return
			}

			fmt.Println("Restarting " + nodeType + ": " + ip)
			if _, err := shared.RestartCluster(ip); err != nil {
				fmt.Printf("\nError restarting %s %s: %v\n\n", nodeType, ip, err)
				errCh <- err
				close(errCh)
				return
			}
			time.Sleep(20 * time.Second)
		}(ip, installType)
	}
	wg.Wait()
	close(errCh)

	return nil
}

func upgradeServer(installType string, serverIPs []string) error {
	return upgradeNode("server", installType, serverIPs)
}

func upgradeAgent(installType string, agentIPs []string) error {
	return upgradeNode("agent", installType, agentIPs)
}
