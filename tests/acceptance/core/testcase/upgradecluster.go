package testcase

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/rancher/rke2/tests/acceptance/core/service/assert"
	"github.com/rancher/rke2/tests/acceptance/core/service/customflag"
	"github.com/rancher/rke2/tests/acceptance/shared/util"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// TestUpgradeClusterSUC upgrades cluster using the system-upgrade-controller.
func TestUpgradeClusterSUC(version string) error {
	fmt.Printf("\nUpgrading cluster to version: %s\n", version)

	_, err := util.ManageWorkload("create", "suc.yaml")
	Expect(err).NotTo(HaveOccurred(),
		"system-upgrade-controller manifest did not deploy successfully")

	getPodsSystemUpgrade := "kubectl get pods -n system-upgrade --kubeconfig="
	assert.CheckComponentCmdHost(
		getPodsSystemUpgrade+util.KubeConfigFile,
		"system-upgrade-controller",
		util.Running,
	)
	Expect(err).NotTo(HaveOccurred())

	originalFilePath := util.BasePath() + "/fixtures/workloads" + "/upgrade-plan.yaml"
	newFilePath := util.BasePath() + "/fixtures/workloads" + "/plan.yaml"

	content, err := os.ReadFile(originalFilePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %s", err)
	}

	newContent := strings.ReplaceAll(string(content), "$UPGRADEVERSION", version)
	err = os.WriteFile(newFilePath, []byte(newContent), 0644)
	if err != nil {
		return fmt.Errorf("failed to write file: %s", err)
	}

	_, err = util.ManageWorkload("create", "plan.yaml")
	Expect(err).NotTo(HaveOccurred(), "failed to upgrade cluster.")

	return nil
}

// TestUpgradeClusterManually upgrades cluster "manually"
func TestUpgradeClusterManually(version string) error {
	if version == "" {
		return fmt.Errorf("please provide a non-empty rke2 version to upgrade to")
	}

	serverIPs := strings.Split(util.ServerIPs, ",")
	agentIPs := strings.Split(util.AgentIPs, ",")

	if util.NumServers == 0 && util.NumAgents == 0 {
		return fmt.Errorf("no nodes found to upgrade")
	}

	if util.NumServers > 0 {
		if err := upgradeServer(version, serverIPs); err != nil {
			return err
		}
	}

	if util.NumAgents > 0 {
		if err := upgradeAgent(version, agentIPs); err != nil {
			return err
		}
	}

	return nil
}

// upgradeServer upgrades servers in cluster,it will spawn a go routine per server ip.
func upgradeServer(installType string, serverIPs []string) error {
	var wg sync.WaitGroup
	errCh := make(chan error, len(serverIPs))

	for _, ip := range serverIPs {
		switch {
		case customflag.InstallType.Version != "":
			installType = fmt.Sprintf("INSTALL_RKE2_VERSION=%s", customflag.InstallType.Version)
		case customflag.InstallType.Commit != "":
			installType = fmt.Sprintf("INSTALL_RKE2_COMMIT=%s", customflag.InstallType.Commit)
		}

		installRke2Server := "sudo curl -sfL https://get.rke2.io | sudo %s INSTALL_RKE2_TYPE=server sh - "
		upgradeCommand := fmt.Sprintf(installRke2Server, installType)
		wg.Add(1)
		go func(ip, installFlagServer string) {
			defer wg.Done()
			defer GinkgoRecover()

			fmt.Println("Upgrading server to: " + upgradeCommand)
			if _, err := util.RunCommandOnNode(upgradeCommand, ip); err != nil {
				fmt.Printf("\nError upgrading server %s: %v\n\n", ip, err)
				errCh <- err
				close(errCh)
				return
			}

			fmt.Println("Restarting server: " + ip)
			if _, err := util.RestartCluster(ip); err != nil {
				fmt.Printf("\nError restarting server %s: %v\n\n", ip, err)
				errCh <- err
				close(errCh)
				return
			}
			time.Sleep(30 * time.Second)
		}(ip, installType)
	}
	wg.Wait()
	close(errCh)

	return nil
}

// upgradeAgent upgrades agents in cluster, it will spawn a go routine per agent ip.
func upgradeAgent(installType string, agentIPs []string) error {
	var wg sync.WaitGroup
	errCh := make(chan error, len(agentIPs))

	for _, ip := range agentIPs {
		switch {
		case customflag.InstallType.Version != "":
			installType = fmt.Sprintf("INSTALL_RKE2_VERSION=%s", customflag.InstallType.Version)
		case customflag.InstallType.Commit != "":
			installType = fmt.Sprintf("INSTALL_RKE2_COMMIT=%s", customflag.InstallType.Commit)
		}

		installRke2Agent := "sudo curl -sfL https://get.rke2.io | sudo %s INSTALL_RKE2_TYPE=agent sh - "
		upgradeCommand := fmt.Sprintf(installRke2Agent, installType)
		wg.Add(1)
		go func(ip, installFlagAgent string) {
			defer wg.Done()
			defer GinkgoRecover()

			fmt.Println("Upgrading agent to: " + upgradeCommand)
			if _, err := util.RunCommandOnNode(upgradeCommand, ip); err != nil {
				fmt.Printf("\nError upgrading agent %s: %v\n\n", ip, err)
				errCh <- err
				close(errCh)
				return
			}

			fmt.Println("Restarting agent: " + ip)
			if _, err := util.RestartCluster(ip); err != nil {
				fmt.Printf("\nError restarting agent %s: %v\n\n", ip, err)
				errCh <- err
				close(errCh)
				return
			}
			time.Sleep(10 * time.Second)
		}(ip, installType)
	}
	wg.Wait()
	close(errCh)

	return nil
}
