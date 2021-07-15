package validation

// Usage:
// ctype: type of cluster- centos, rhel, ubuntu this picks up respective variable file for ami and username
// destroy: true to delete cluster, false to create cluster

//go test -v createcluster_test.go createcluster.go -ctype=centos -destroy=false -timeout=2h
//go test -v createcluster_test.go createcluster.go -ctype=centos -destroy=true -timeout=2h

//NOTE rhel 7.8 needs subscription registration

import (
	"flag"
	"fmt"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/rancher/rke2/tests/validation/testutils"
)

var destroy = flag.Bool("destroy", false, "a bool")
var ctype = flag.String("ctype", "centos", "a string")

func TestCluster(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Cluster Validation Suite")
}

var (
	Kubeconfig string
	MasterIPs  string
	WorkerIPs  string
)

func TestBuildCluster(t *testing.T) {

	Kubeconfig, MasterIPs, WorkerIPs = BuildCluster(*ctype, t, *destroy)
	Kubeconfig = Kubeconfig + "_kubeconfig"
	fmt.Printf("export KUBECONFIG=%s", Kubeconfig)
	fmt.Println(MasterIPs)
	fmt.Println("Worker node IPs")
	fmt.Println(WorkerIPs)

	fmt.Println("Nodes List")
	nodes := ParseNode(Kubeconfig)
	fmt.Println("Pods List")
	for _, config := range nodes {
		Expect(config.Status).Should(Equal("Ready"))
	}
	pods := ParsePod(Kubeconfig)
	for _, pod := range pods {
		if strings.Contains(pod.Name, "helm-install") {
			Expect(pod.Status).Should(Equal("Completed"))
		} else {
			Expect(pod.Status).Should(Equal("Running"))
		}
	}
	fmt.Printf("export KUBECONFIG=%s\n", Kubeconfig)
}
