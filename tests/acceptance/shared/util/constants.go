package util

import (
	"flag"
)

// global configurations
var (
	Destroy        = flag.Bool("destroy", false, "a bool")
	KubeConfigFile string
	ServerIPs      string
	AgentIPs       string
	NumServers     int
	NumAgents      int
	AwsUser        string
	AccessKey      string

	ExecDnsUtils          = "kubectl exec -n auto-dns -t dnsutils --kubeconfig="
	GetAll                = "kubectl get all -A --kubeconfig="
	GetNodesWide          = "kubectl get nodes --no-headers -o wide --kubeconfig="
	GetPodsWide           = "kubectl get pods -o wide --no-headers -A --kubeconfig="
	GetNodesExternalIp    = "kubectl get nodes --output=jsonpath='{.items[*].status.addresses[?(@.type==\"ExternalIP\")].address}' --kubeconfig="
	GetCoreDNSdeployImage = "kubectl get deploy rke2-coredns-rke2-coredns -n kube-system -o jsonpath='{.spec.template.spec.containers[?(@.name==\"coredns\")].image}'"
	GetWorkerNodes        = "kubectl get node -o jsonpath='{range .items[*]}{@.metadata.name} " +
		"{@.status.conditions[-1].type} <not retrieved> <not retrieved> " +
		"{@.status.nodeInfo.kubeletVersion} " +
		"{@.status.addresses[?(@.type==\"InternalIP\")].address} " +
		"{@.status.addresses[?(@.type==\"ExternalIP\")].address} " +
		"{@.spec.taints[*].effect}{\"\\n\"}{end}' " +
		"--kubeconfig="
	Running  = "Running"
	Nslookup = "kubernetes.default.svc.cluster.local"
)
