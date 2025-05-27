package tests

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/utils/set"
)

// This file consolidates functions that are used across multiple testing frameworks.
// Most of it relates to interacting with the Kubernetes API and checking the status of resources.

// CheckDefaultDeployments checks if the standard array of RKE2 deployments are ready, otherwise returns an error
func CheckDefaultDeployments(kubeconfigFile string) error {
	return CheckDeployments([]string{"rke2-coredns-rke2-coredns", "rke2-coredns-rke2-coredns-autoscaler", "rke2-metrics-server", "rke2-snapshot-controller"}, kubeconfigFile)
}

// CheckDeployments checks if the provided list of deployments are ready, otherwise returns an error
func CheckDeployments(deployments []string, kubeconfigFile string) error {

	deploymentSet := make(map[string]bool)
	for _, d := range deployments {
		deploymentSet[d] = false
	}

	client, err := K8sClient(kubeconfigFile)
	if err != nil {
		return err
	}
	deploymentList, err := client.AppsV1().Deployments("").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return err
	}
	for _, deployment := range deploymentList.Items {
		if _, ok := deploymentSet[deployment.Name]; ok && deployment.Status.ReadyReplicas == deployment.Status.Replicas {
			deploymentSet[deployment.Name] = true
		}
	}
	for d, found := range deploymentSet {
		if !found {
			return fmt.Errorf("failed to deploy %s", d)
		}
	}

	return nil
}

// CheckDefaultDaemonSets checks if the standard array of RKE2 DaemonSets are ready, otherwise returns an error
func CheckDefaultDaemonSets(kubeconfigFile string) error {
	return CheckDaemonSets([]string{"rke2-canal", "rke2-ingress-nginx-controller"}, kubeconfigFile)
}

// CheckDaemonSets checks if the provided list of DaemonSets are ready, otherwise returns an error
func CheckDaemonSets(daemonsets []string, kubeconfigFile string) error {

	daemonsetSet := make(map[string]bool)
	for _, d := range daemonsets {
		daemonsetSet[d] = false
	}

	client, err := K8sClient(kubeconfigFile)
	if err != nil {
		return err
	}

	daemonsetList, err := client.AppsV1().DaemonSets("").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, daemonset := range daemonsetList.Items {
		if _, ok := daemonsetSet[daemonset.Name]; ok && daemonset.Status.NumberReady == daemonset.Status.DesiredNumberScheduled {
			daemonsetSet[daemonset.Name] = true
		}
	}

	for d, found := range daemonsetSet {
		if !found {
			return fmt.Errorf("failed to deploy %s", d)
		}
	}

	return nil
}

func ParseServices(kubeconfigFile string) ([]corev1.Service, error) {
	clientSet, err := K8sClient(kubeconfigFile)
	if err != nil {
		return nil, err
	}

	ser, err := clientSet.CoreV1().Services("").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return ser.Items, nil
}

func ParseNodes(kubeconfigFile string) ([]corev1.Node, error) {
	clientSet, err := K8sClient(kubeconfigFile)
	if err != nil {
		return nil, err
	}
	nodes, err := clientSet.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	return nodes.Items, nil
}

func ParsePods(kubeconfigFile string) ([]corev1.Pod, error) {
	clientSet, err := K8sClient(kubeconfigFile)
	if err != nil {
		return nil, err
	}
	pods, err := clientSet.CoreV1().Pods("").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	return pods.Items, nil
}

// AllPodsUp checks if pods on the cluster are Running or Succeeded, otherwise returns an error
func AllPodsUp(kubeconfigFile string) error {
	clientSet, err := K8sClient(kubeconfigFile)
	if err != nil {
		return err
	}
	pods, err := clientSet.CoreV1().Pods("").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return err
	}
	for _, pod := range pods.Items {
		// Check if the pod is running
		if pod.Status.Phase != corev1.PodRunning && pod.Status.Phase != corev1.PodSucceeded {
			return fmt.Errorf("pod %s is %s", pod.Name, pod.Status.Phase)
		}
	}
	return nil
}

// PodReady checks if a pod is ready by querying its status
func PodReady(podName, namespace, kubeconfigFile string) (bool, error) {
	clientSet, err := K8sClient(kubeconfigFile)
	if err != nil {
		return false, err
	}
	pod, err := clientSet.CoreV1().Pods(namespace).Get(context.TODO(), podName, metav1.GetOptions{})
	if err != nil {
		return false, fmt.Errorf("failed to get pod: %v", err)
	}
	// Check if the pod is running
	for _, containerStatus := range pod.Status.ContainerStatuses {
		if containerStatus.Name == podName && containerStatus.Ready {
			return true, nil
		}
	}
	return false, nil
}

// GetPodIPs returns all IP addresses attached to a pod
func GetPodIPs(podName, namespace, kubeconfigFile string) ([]string, error) {
	clientSet, err := K8sClient(kubeconfigFile)
	if err != nil {
		return nil, err
	}

	pod, err := clientSet.CoreV1().Pods(namespace).Get(context.TODO(), podName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get pod %s/%s: %w", namespace, podName, err)
	}

	var ips []string

	// Get the primary pod IP
	if pod.Status.PodIP != "" {
		ips = append(ips, pod.Status.PodIP)
	}

	// Get additional pod IPs (for dual-stack scenarios)
	for _, podIP := range pod.Status.PodIPs {
		// Avoid duplicating the primary IP
		if podIP.IP != pod.Status.PodIP {
			ips = append(ips, podIP.IP)
		}
	}

	return ips, nil
}

// Checks if provided nodes are ready, otherwise returns an error
func NodesReady(kubeconfigFile string, nodeNames []string) error {
	nodes, err := ParseNodes(kubeconfigFile)
	if err != nil {
		return err
	}
	nodesToCheck := set.New(nodeNames...)
	readyNodes := make(set.Set[string], 0)
	for _, node := range nodes {
		for _, condition := range node.Status.Conditions {
			if condition.Type == corev1.NodeReady && condition.Status != corev1.ConditionTrue {
				return fmt.Errorf("node %s is not ready", node.Name)
			}
			readyNodes.Insert(node.Name)
		}
	}
	// Check if all nodes are ready
	if !nodesToCheck.Equal(readyNodes) {
		return fmt.Errorf("expected nodes %v, found %v", nodesToCheck, readyNodes)
	}
	return nil
}

// GetNodeIPs returns all IP addresses attached to a node
func GetNodeIPs(nodeName, kubeconfigFile string) ([]string, error) {
	clientSet, err := K8sClient(kubeconfigFile)
	if err != nil {
		return nil, err
	}

	node, err := clientSet.CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get node %s: %w", nodeName, err)
	}

	var ips []string

	// Get the primary node IP
	if node.Status.Addresses != nil {
		for _, address := range node.Status.Addresses {
			if address.Type == corev1.NodeInternalIP || address.Type == corev1.NodeExternalIP {
				ips = append(ips, address.Address)
			}
		}
	}

	return ips, nil
}

func K8sClient(kubeconfigFile string) (*kubernetes.Clientset, error) {
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigFile)
	if err != nil {
		return nil, err
	}
	clientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return clientSet, nil
}
