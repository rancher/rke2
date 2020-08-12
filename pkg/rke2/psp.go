package rke2

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"k8s.io/api/policy/v1beta1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	yamlutil "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/transport"
)

// globalUnrestrictedPSP
const globalUnrestrictedPSP = `apiVersion: policy/v1beta1
kind: PodSecurityPolicy
metadata:
  name: default-psp
  annotations:
    seccomp.security.alpha.kubernetes.io/allowedProfileNames: '*'
spec:
  privileged: true
  allowPrivilegeEscalation: true
  allowedCapabilities:
  - '*'
  volumes:
  - '*'
  hostNetwork: true
  hostPorts:
  - min: 0
    max: 65535
  hostIPC: true
  hostPID: true
  runAsUser:
    rule: 'RunAsAny'
  seLinux:
    rule: 'RunAsAny'
  supplementalGroups:
    rule: 'RunAsAny'
  fsGroup:
    rule: 'RunAsAny'
`

// defaultPSPClusterRole
const defaultPSPClusterRole = `kind: ClusterRole
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: default-psp-role
rules:
- apiGroups: ['policy']
  resources: ['podsecuritypolicies']
  verbs:     ['use']
  resourceNames:
  - default-psp
`

// globalUnrestrictedRoleBinding
const globalUnrestrictedRoleBinding = `apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: default-psp-rolebinding
  namespace: kube-system
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: default-psp-role
subjects:
- kind: Group
  apiGroup: rbac.authorization.k8s.io
  name: system:serviceaccounts
`

// clusterRoleBinding
const clusterRoleBinding = `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: system-node-default-psp-rolebinding
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: default-psp-role
subjects:
- kind: Group
  apiGroup: rbac.authorization.k8s.io
  name: system:nodes
`

const (
	defaultRetries          = 5
	defaultWaitSeconds      = 5
	defaultTimeout          = 30
	k8sWrapTransportTimeout = 30
)

// k8sCall
type k8sCall func(*kubernetes.Clientset, interface{}) error

// NewClient
func NewClient(kubeConfigPath string, k8sWrapTransport transport.WrapperFunc) (*kubernetes.Clientset, error) {
	config, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	if err != nil {
		return nil, err
	}
	if k8sWrapTransport != nil {
		config.WrapTransport = k8sWrapTransport
	}
	config.Timeout = time.Second * defaultTimeout
	return kubernetes.NewForConfig(config)
}

// DecodeYamlResource
func DecodeYamlResource(data interface{}, yaml string) error {
	decoder := yamlutil.NewYAMLToJSONDecoder(bytes.NewReader([]byte(yaml)))
	return decoder.Decode(data)
}

// retryTo
func retryTo(runFunc k8sCall, cs *kubernetes.Clientset, resource interface{}, retries, wait int) error {
	var err error
	if retries <= 0 {
		retries = defaultRetries
	}
	if wait <= 0 {
		wait = defaultWaitSeconds
	}
	for i := 0; i < retries; i++ {
		if err = runFunc(cs, resource); err != nil {
			time.Sleep(time.Second * time.Duration(wait))
			continue
		}
		return nil
	}
	return err
}

// retryToWithTimeout
func retryToWithTimeout(runFunc k8sCall, cs *kubernetes.Clientset, resource interface{}, timeout int) error {
	var err error
	var timePassed int
	for timePassed < timeout {
		if err = runFunc(cs, resource); err != nil {
			time.Sleep(time.Second * time.Duration(defaultWaitSeconds))
			timePassed += defaultWaitSeconds
			continue
		}
		return nil
	}
	return err
}

// DeployPodSecurityPolicyFromYaml
func DeployPodSecurityPolicyFromYaml(cs *kubernetes.Clientset, pspYaml string) error {
	var psp v1beta1.PodSecurityPolicy
	if err := DecodeYamlResource(&psp, pspYaml); err != nil {
		return err
	}
	return retryTo(deployPodSecurityPolicy, cs, psp, defaultRetries, defaultWaitSeconds)
}

// deployPodSecurityPolicy
func deployPodSecurityPolicy(cs *kubernetes.Clientset, p interface{}) error {
	psp, ok := p.(v1beta1.PodSecurityPolicy)
	if !ok {
		return fmt.Errorf("invalid type provided: %T, expected: PodSecurityPolicy", p)
	}
	if _, err := cs.PolicyV1beta1().PodSecurityPolicies().Create(context.TODO(), &psp, metav1.CreateOptions{}); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return err
		}
		if _, err := cs.PolicyV1beta1().PodSecurityPolicies().Update(context.TODO(), &psp, metav1.UpdateOptions{}); err != nil {
			return err
		}
	}
	return nil
}

// DeployClusterRoleBindingFromYaml
func DeployClusterRoleBindingFromYaml(cs *kubernetes.Clientset, clusterRoleBindingYaml string) error {
	var clusterRoleBinding rbacv1.ClusterRoleBinding
	if err := DecodeYamlResource(&clusterRoleBinding, clusterRoleBindingYaml); err != nil {
		return err
	}
	return retryTo(deployClusterRoleBinding, cs, clusterRoleBinding, defaultRetries, defaultWaitSeconds)
}

// deployClusterRoleBinding
func deployClusterRoleBinding(cs *kubernetes.Clientset, crb interface{}) error {
	clusterRoleBinding, ok := crb.(rbacv1.ClusterRoleBinding)
	if !ok {
		return fmt.Errorf("invalid type provided: %T, expected: ClusterRoleBinding", crb)
	}
	if _, err := cs.RbacV1().ClusterRoleBindings().Create(context.TODO(), &clusterRoleBinding, metav1.CreateOptions{}); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return err
		}
		if _, err := cs.RbacV1().ClusterRoleBindings().Update(context.TODO(), &clusterRoleBinding, metav1.UpdateOptions{}); err != nil {
			return err
		}
	}
	return nil
}

// DeployClusterRoleFromYaml
func DeployClusterRoleFromYaml(cs *kubernetes.Clientset, clusterRoleYaml string) error {
	var clusterRole rbacv1.ClusterRole
	if err := DecodeYamlResource(&clusterRole, clusterRoleYaml); err != nil {
		return err
	}

	return retryTo(deployClusterRole, cs, clusterRole, defaultRetries, defaultWaitSeconds)
}

// deployClusterRole
func deployClusterRole(cs *kubernetes.Clientset, cr interface{}) error {
	clusterRole, ok := cr.(rbacv1.ClusterRole)
	if !ok {
		return fmt.Errorf("invalid type provided: %T, expected: ClusterRole", cr)
	}
	if _, err := cs.RbacV1().ClusterRoles().Create(context.TODO(), &clusterRole, metav1.CreateOptions{}); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return err
		}
		if _, err := cs.RbacV1().ClusterRoles().Update(context.TODO(), &clusterRole, metav1.UpdateOptions{}); err != nil {
			return err
		}
	}
	return nil
}

// DeployRoleBindingFromYaml
func DeployRoleBindingFromYaml(cs *kubernetes.Clientset, roleBindingYaml, namespace string) error {
	var roleBinding rbacv1.RoleBinding
	if err := DecodeYamlResource(&roleBinding, roleBindingYaml); err != nil {
		return err
	}
	roleBinding.Namespace = namespace
	return retryTo(deployRoleBinding, cs, roleBinding, defaultRetries, defaultWaitSeconds)
}

// deployRoleBinding
func deployRoleBinding(cs *kubernetes.Clientset, rb interface{}) error {
	roleBinding, ok := rb.(rbacv1.RoleBinding)
	if !ok {
		return fmt.Errorf("invalid type provided: %T, expected: RoleBinding", rb)
	}
	if _, err := cs.RbacV1().RoleBindings(roleBinding.Namespace).Create(context.TODO(), &roleBinding, metav1.CreateOptions{}); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return err
		}
		if _, err := cs.RbacV1().RoleBindings(roleBinding.Namespace).Update(context.TODO(), &roleBinding, metav1.UpdateOptions{}); err != nil {
			return err
		}
	}
	return nil
}

// DeployRoleFromYaml
func DeployRoleFromYaml(cs *kubernetes.Clientset, roleYaml, namespace string) error {
	var role rbacv1.Role
	if err := DecodeYamlResource(&role, roleYaml); err != nil {
		return err
	}
	role.Namespace = namespace
	return retryTo(deployRole, cs, role, defaultRetries, defaultWaitSeconds)
}

// deployRole
func deployRole(cs *kubernetes.Clientset, r interface{}) error {
	role, ok := r.(rbacv1.Role)
	if !ok {
		return fmt.Errorf("invalid type provided: %T, expected: Role", r)
	}
	if _, err := cs.RbacV1().Roles(role.Namespace).Create(context.TODO(), &role, metav1.CreateOptions{}); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return err
		}
		if _, err := cs.RbacV1().Roles(role.Namespace).Update(context.TODO(), &role, metav1.UpdateOptions{}); err != nil {
			return err
		}
	}
	return nil
}
