package psp

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/rancher/spur/cli"
	"github.com/sirupsen/logrus"
	"k8s.io/api/policy/v1beta1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	yamlutil "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/transport"
)

const (
	defaultRetries          = 5
	defaultWaitSeconds      = 5
	defaultTimeout          = 30
	k8sWrapTransportTimeout = 30
)

const (
	kubeSystemNamespace = "kube-system"
	cisAnnotationValue  = "resolved"

	namespaceAnnotationBase               = "psp.rke2.io/"
	namespaceAnnotationGlobalRestricted   = namespaceAnnotationBase + "global-restricted"
	namespaceAnnotationGlobalUnrestricted = namespaceAnnotationBase + "global-unrestricted"
	namespaceAnnotationSystemUnrestricted = namespaceAnnotationBase + "system-unrestricted"
)

// SetPSPs
// If we are in CIS mode:
//
// 	- Check for all three annotations exist.. Bye!
//
// 	- Check if the globalUnrestricted annotation does not exist, create PSP, role, role binding.
// 	- Check if the systemUnrestructed annotation does not exist, create the PSP, role, role binding.
//
// 	- Check if the globalRestructed annotation does not exist, check if the PSP exists, if not, create it. And
// 	  check if role and role bindings exist, delete them.
//
// If we are NOT in CIS mode:
//
// check for all three annotations exist.. Bye!
//
// Check if the globalRestricted annotation does not exist, create PSP, role, role binding.
// Check if the systemUnrestricted annotation does not exist, create the PSP, role, role binding.
//
// Check if the globalUnrestricted annotation does not exist, check if PSP exists, and if so,
// delete it, the role, and the bindings. Create annotation at completion.
func SetPSPs(ctx *cli.Context, k8sWrapTransport transport.WrapperFunc) error {
	const (
		kubeConfigPath = "/etc/rancher/rke2/rke2.yaml"
		apiWaitDelay   = time.Second * 1
	)

	cs, err := newClient(kubeConfigPath, k8sWrapTransport)
	if err != nil {
		return err
	}

	for {
		if _, err := cs.Discovery().ServerVersion(); err != nil {
			logrus.Info("waiting on API to become available")
			time.Sleep(apiWaitDelay)
			continue
		}

		ns, err := cs.CoreV1().Namespaces().Get(context.TODO(), kubeSystemNamespace, metav1.GetOptions{})
		if err != nil {
			return err
		}

		if ctx.String("profile") == "" { // CIS mode
			if _, ok := ns.Annotations[namespaceAnnotationGlobalUnrestricted]; !ok {
				tmpl := fmt.Sprintf(globalUnrestrictedPSPTemplate, globalUnrestrictedPSPName)
				if err := deployPodSecurityPolicyFromYaml(cs, tmpl); err != nil {
					return err
				}
				tmpl = fmt.Sprintf(roleTemplate, globalUnrestrictedRoleName, globalUnrestrictedPSPName)
				if err := deployRoleFromYaml(cs, tmpl, kubeSystemNamespace); err != nil {
					return err
				}
				tmpl = fmt.Sprintf(globalUnrestrictedRoleBindingTemplate, globalUnrestrictedRoleBindingName, globalUnrestrictedRoleName)
				if err := deployRoleBindingFromYaml(cs, tmpl); err != nil {
					return err
				}
				ns.SetAnnotations(map[string]string{namespaceAnnotationGlobalUnrestricted: cisAnnotationValue})
			}

			if _, ok := ns.Annotations[namespaceAnnotationSystemUnrestricted]; !ok {
				tmpl := fmt.Sprintf(systemUnrestrictedPSPTemplate, systemUnrestrictedPSPName)
				if err := deployPodSecurityPolicyFromYaml(cs, tmpl); err != nil {
					return err
				}
				tmpl = fmt.Sprintf(roleTemplate, globalUnrestrictedRoleName, globalUnrestrictedPSPName)
				if err := deployRoleFromYaml(cs, tmpl, kubeSystemNamespace); err != nil {
					return err
				}
				tmpl = fmt.Sprintf(globalUnrestrictedRoleBindingTemplate, globalUnrestrictedRoleBindingName, globalUnrestrictedRoleName)
				if err := deployRoleBindingFromYaml(cs, tmpl); err != nil {
					return err
				}
				ns.SetAnnotations(map[string]string{namespaceAnnotationSystemUnrestricted: cisAnnotationValue})
			}

			if _, ok := ns.Annotations[namespaceAnnotationGlobalRestricted]; !ok {
				if _, err := cs.PolicyV1beta1().PodSecurityPolicies().Get(context.TODO(), globalRestrictedPSPName, metav1.GetOptions{}); err != nil {
					if apierrors.IsAlreadyExists(err) {
						tmpl := fmt.Sprintf(globalRestrictedPSPTemplate, globalRestrictedPSPName)
						if err := deployPodSecurityPolicyFromYaml(cs, tmpl); err != nil {
							return err
						}
						ns.SetAnnotations(map[string]string{namespaceAnnotationGlobalRestricted: cisAnnotationValue})
					}
				}
				// check if role exists and delete it
				role, err := cs.RbacV1().ClusterRoles().Get(context.TODO(), globalRestrictedRoleName, metav1.GetOptions{})
				if err != nil {
					logrus.Warn(err)
				}
				if role != nil {
					if err := cs.RbacV1().ClusterRoles().Delete(context.TODO(), role.Name, metav1.DeleteOptions{}); err != nil {
						return err
					}
				}

				// check if role binding exists and delete it
				roleBinding, err := cs.RbacV1().ClusterRoleBindings().Get(context.TODO(), globalRestrictedRoleName, metav1.GetOptions{})
				if err != nil {
					logrus.Warn(err)
				}
				if roleBinding != nil {
					if err := cs.RbacV1().ClusterRoleBindings().Delete(context.TODO(), roleBinding.Name, metav1.DeleteOptions{}); err != nil {
						return err
					}
				}
				ns.SetAnnotations(map[string]string{namespaceAnnotationGlobalRestricted: cisAnnotationValue})
			}
		} else { // non-CIS mode

		}
		// node policy
		if err := deployClusterRoleBindingFromYaml(cs, nodeClusterRoleBindingTemplate); err != nil {
			return err
		}
	}
}

// deployFn
type deployFn func(*kubernetes.Clientset, interface{}) error

// NewClient
func newClient(kubeConfigPath string, k8sWrapTransport transport.WrapperFunc) (*kubernetes.Clientset, error) {
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

// decodeYamlResource
func decodeYamlResource(data interface{}, yaml string) error {
	decoder := yamlutil.NewYAMLToJSONDecoder(bytes.NewReader([]byte(yaml)))
	return decoder.Decode(data)
}

// retryTo
func retryTo(runFunc deployFn, cs *kubernetes.Clientset, resource interface{}, retries, wait int) error {
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
func retryToWithTimeout(runFunc deployFn, cs *kubernetes.Clientset, resource interface{}, timeout int) error {
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

// deployPodSecurityPolicyFromYaml
func deployPodSecurityPolicyFromYaml(cs *kubernetes.Clientset, pspYaml string) error {
	var psp v1beta1.PodSecurityPolicy
	if err := decodeYamlResource(&psp, pspYaml); err != nil {
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

// deployClusterRoleBindingFromYaml
func deployClusterRoleBindingFromYaml(cs *kubernetes.Clientset, clusterRoleBindingYaml string) error {
	var clusterRoleBinding rbacv1.ClusterRoleBinding
	if err := decodeYamlResource(&clusterRoleBinding, clusterRoleBindingYaml); err != nil {
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

// deployClusterRoleFromYaml
func deployClusterRoleFromYaml(cs *kubernetes.Clientset, clusterRoleYaml string) error {
	var clusterRole rbacv1.ClusterRole
	if err := decodeYamlResource(&clusterRole, clusterRoleYaml); err != nil {
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

// deployRoleBindingFromYaml
func deployRoleBindingFromYaml(cs *kubernetes.Clientset, roleBindingYaml string) error {
	var roleBinding rbacv1.RoleBinding
	if err := decodeYamlResource(&roleBinding, roleBindingYaml); err != nil {
		return err
	}
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

// deployRoleFromYaml
func deployRoleFromYaml(cs *kubernetes.Clientset, roleYaml, namespace string) error {
	var role rbacv1.Role
	if err := decodeYamlResource(&role, roleYaml); err != nil {
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
