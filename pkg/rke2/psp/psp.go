package psp

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"time"

	"github.com/rancher/spur/cli"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
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

// setGlobalUnrestricted sets the global unrestricted podSecurityPolicy with
// the associated role and rolebinding.
func setGlobalUnrestricted(cs *kubernetes.Clientset, ns *v1.Namespace) error {
	if _, ok := ns.Annotations[namespaceAnnotationGlobalUnrestricted]; !ok {
		if _, err := cs.RbacV1().ClusterRoles().Get(context.TODO(), globalUnrestrictedPSPName, metav1.GetOptions{}); err != nil {
			if apierrors.IsNotFound(err) {
				tmpl := fmt.Sprintf(globalUnrestrictedPSPTemplate, globalUnrestrictedPSPName)
				if err := deployPodSecurityPolicyFromYaml(cs, tmpl); err != nil {
					return err
				}
			}
		}

		if _, err := cs.RbacV1().ClusterRoles().Get(context.TODO(), globalUnrestrictedRoleName, metav1.GetOptions{}); err != nil {
			if apierrors.IsNotFound(err) {
				tmpl := fmt.Sprintf(roleTemplate, globalUnrestrictedRoleName, globalUnrestrictedPSPName)
				if err := deployClusterRoleFromYaml(cs, tmpl); err != nil {
					return err
				}
			}
		}

		if _, err := cs.RbacV1().ClusterRoleBindings().Get(context.TODO(), globalUnrestrictedRoleBindingName, metav1.GetOptions{}); err != nil {
			if apierrors.IsNotFound(err) {
				tmpl := fmt.Sprintf(globalUnrestrictedRoleBindingTemplate, globalUnrestrictedRoleBindingName, globalUnrestrictedRoleName)
				if err := deployClusterRoleBindingFromYaml(cs, tmpl); err != nil {
					return err
				}
			}
		}

		ns.SetAnnotations(map[string]string{namespaceAnnotationGlobalUnrestricted: cisAnnotationValue})
	}
	return nil
}

// setSystemUnrestricted sets the system unrestricted podSecurityPolicy as
// the associated role and rolebinding.
func setSystemUnrestricted(cs *kubernetes.Clientset, ns *v1.Namespace) error {
	if _, ok := ns.Annotations[namespaceAnnotationSystemUnrestricted]; !ok {
		if _, err := cs.RbacV1().ClusterRoles().Get(context.TODO(), systemUnrestrictedPSPName, metav1.GetOptions{}); err != nil {
			if apierrors.IsNotFound(err) {
				tmpl := fmt.Sprintf(systemUnrestrictedPSPTemplate, systemUnrestrictedPSPName)
				if err := deployPodSecurityPolicyFromYaml(cs, tmpl); err != nil {
					return err
				}
			}
		}
		if _, err := cs.RbacV1().ClusterRoles().Get(context.TODO(), systemUnrestrictedRoleName, metav1.GetOptions{}); err != nil {
			if apierrors.IsNotFound(err) {
				tmpl := fmt.Sprintf(roleTemplate, systemUnrestrictedRoleName, systemUnrestrictedPSPName)
				if err := deployClusterRoleFromYaml(cs, tmpl); err != nil {
					return err
				}
			}
		}
		if _, err := cs.RbacV1().ClusterRoleBindings().Get(context.TODO(), systemUnrestrictedRoleBindingName, metav1.GetOptions{}); err != nil {
			if apierrors.IsNotFound(err) {
				tmpl := fmt.Sprintf(systemUnrestrictedNodesRoleBindingTemplate, systemUnrestrictedRoleBindingName, systemUnrestrictedRoleName)
				if err := deployClusterRoleBindingFromYaml(cs, tmpl); err != nil {
					return err
				}
			}
		}
		if _, err := cs.RbacV1().ClusterRoleBindings().Get(context.TODO(), systemUnrestrictedSvcAcctRoleBindingName, metav1.GetOptions{}); err != nil {
			if apierrors.IsNotFound(err) {
				tmpl := fmt.Sprintf(systemUnrestrictedServiceAcctRoleBindingTemplate, systemUnrestrictedSvcAcctRoleBindingName, systemUnrestrictedRoleName)
				if err := deployRoleBindingFromYaml(cs, tmpl); err != nil {
					return err
				}
			}
		}
		ns.SetAnnotations(map[string]string{namespaceAnnotationSystemUnrestricted: cisAnnotationValue})
	}
	return nil
}

// kubeConfigExists checks if the kubeconfig file
// has been written.
func kubeConfigExists(kc string) bool {
	info, err := os.Stat(kc)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

// SetPSPs sets the default PSP's based on the mode that RKE2 is running in. There is either CIS or non
// CIS mode. For CIS mode, the globalRestricted and systemUnrestricted polices and associated roles and
// bindings are created. For non CIS mode, globalRestricted, system-unrestricted, and global-unrestricted
// policies and associated roles and bindings are loaded, if applicable.
//
// Each PSP has an annotation associated with it that we check to see exists before performing any
// write operations. The annotation is created upon successful setting of PSPs.
//
// Load logic
//
// CIS:
// - Check if the globalRestricted annotation does not exist, create PSP, role, binding.
// - Check if the systemUnrestricted annotation does not exist, create the PSP, role, binding.
// - Check if the globalUnrestricted annotation does not exist, check if PSP exists, and if so,
//   delete it, the role, and the bindings.
//
// non-CIS:
// - Check if the globalUnrestricted annotation does not exist, create PSP, role, binding.
// - Check if the systemUnrestricted annotation does not exist, create PSP, role, binding.
// - Check if the globalRestricted annotation does not exist, check if the PSP exists and
//   if it doesn't, create it. Check to see if the associated role and bindings exist and
//   if they do, delete them.
func SetPSPs(ctx *cli.Context, k8sWrapTransport transport.WrapperFunc) error {
	const (
		kubeConfigPath = "/etc/rancher/rke2/rke2.yaml"
		waitDelay      = time.Second * 1
	)

	// wait until the kubeconfig file is written
	for !kubeConfigExists(kubeConfigPath) {
		time.Sleep(waitDelay)
	}

	cs, err := newClient(kubeConfigPath, k8sWrapTransport)
	if err != nil {
		return err
	}

	for {
		// wait until kube-apiserver is running
		if _, err := cs.Discovery().ServerVersion(); err != nil {
			time.Sleep(waitDelay)
			continue
		}

		// wait until the kube-system namespace is created
		ns, err := cs.CoreV1().Namespaces().Get(context.TODO(), kubeSystemNamespace, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				time.Sleep(waitDelay)
				continue
			}
			return err
		}

		if ctx.String("profile") == "" { // non-CIS mode
			if err := setGlobalUnrestricted(cs, ns); err != nil {
				return err
			}

			if err := setSystemUnrestricted(cs, ns); err != nil {
				return err
			}

			if _, ok := ns.Annotations[namespaceAnnotationGlobalRestricted]; !ok {
				// check if the policy exists
				if _, err := cs.PolicyV1beta1().PodSecurityPolicies().Get(context.TODO(), globalRestrictedPSPName, metav1.GetOptions{}); err != nil {
					if apierrors.IsNotFound(err) {
						tmpl := fmt.Sprintf(globalRestrictedPSPTemplate, globalRestrictedPSPName)
						if err := deployPodSecurityPolicyFromYaml(cs, tmpl); err != nil {
							return err
						}
					}
				}

				// check if role exists and delete it
				role, err := cs.RbacV1().ClusterRoles().Get(context.TODO(), globalRestrictedRoleName, metav1.GetOptions{})
				if err != nil {
					if apierrors.IsNotFound(err) {
						logrus.Info(err)
					} else {
						if err := cs.RbacV1().ClusterRoles().Delete(context.TODO(), role.Name, metav1.DeleteOptions{}); err != nil {
							return err
						}
					}
				}

				// check if role binding exists and delete it
				roleBinding, err := cs.RbacV1().ClusterRoleBindings().Get(context.TODO(), globalRestrictedRoleBindingName, metav1.GetOptions{})
				if err != nil {
					if apierrors.IsNotFound(err) {
						logrus.Info(err)
					}
				} else {
					if err := cs.RbacV1().ClusterRoleBindings().Delete(context.TODO(), roleBinding.Name, metav1.DeleteOptions{}); err != nil {
						return err
					}
				}
				ns.SetAnnotations(map[string]string{namespaceAnnotationGlobalRestricted: cisAnnotationValue})
			}
		} else { // CIS mode
			if _, ok := ns.Annotations[namespaceAnnotationGlobalRestricted]; !ok {
				if _, err := cs.PolicyV1beta1().PodSecurityPolicies().Get(context.TODO(), globalRestrictedPSPName, metav1.GetOptions{}); err != nil {
					tmpl := fmt.Sprintf(globalRestrictedPSPTemplate, globalRestrictedPSPName)
					if err := deployPodSecurityPolicyFromYaml(cs, tmpl); err != nil {
						return err
					}
				}
				if _, err := cs.RbacV1().ClusterRoles().Get(context.TODO(), globalRestrictedRoleName, metav1.GetOptions{}); err != nil {
					if apierrors.IsNotFound(err) {
						tmpl := fmt.Sprintf(roleTemplate, globalRestrictedRoleName, globalRestrictedPSPName)
						if err := deployClusterRoleFromYaml(cs, tmpl); err != nil {
							return err
						}
					}
				}
				if _, err := cs.RbacV1().ClusterRoleBindings().Get(context.TODO(), globalRestrictedRoleBindingName, metav1.GetOptions{}); err != nil {
					if apierrors.IsNotFound(err) {
						tmpl := fmt.Sprintf(globalRestrictedRoleBindingTemplate, globalRestrictedRoleBindingName, globalRestrictedRoleName)
						if err := deployClusterRoleBindingFromYaml(cs, tmpl); err != nil {
							return err
						}
					}
				}
				ns.SetAnnotations(map[string]string{namespaceAnnotationGlobalRestricted: cisAnnotationValue})
			}

			if err := setSystemUnrestricted(cs, ns); err != nil {
				return err
			}

			if _, ok := ns.Annotations[namespaceAnnotationGlobalUnrestricted]; !ok {
				if _, err := cs.PolicyV1beta1().PodSecurityPolicies().Get(context.TODO(), globalRestrictedPSPName, metav1.GetOptions{}); err != nil {
					if apierrors.IsNotFound(err) {
						tmpl := fmt.Sprintf(globalRestrictedPSPTemplate, globalRestrictedPSPName)
						if err := deployPodSecurityPolicyFromYaml(cs, tmpl); err != nil {
							return err
						}
					}
				}

				// check if role exists and if so, delete it
				role, err := cs.RbacV1().ClusterRoles().Get(context.TODO(), globalRestrictedRoleName, metav1.GetOptions{})
				if err != nil {
					if apierrors.IsNotFound(err) {
						logrus.Info(err)
					}
				} else {
					if err := cs.RbacV1().ClusterRoles().Delete(context.TODO(), role.Name, metav1.DeleteOptions{}); err != nil {
						return err
					}
				}

				// check if role binding exists and if so, delete it
				roleBinding, err := cs.RbacV1().ClusterRoleBindings().Get(context.TODO(), globalUnrestrictedRoleName, metav1.GetOptions{})
				if err != nil {
					if apierrors.IsNotFound(err) {
						logrus.Info(err)
					} else {
						if err := cs.RbacV1().ClusterRoleBindings().Delete(context.TODO(), roleBinding.Name, metav1.DeleteOptions{}); err != nil {
							return err
						}
					}
				}

				ns.SetAnnotations(map[string]string{namespaceAnnotationGlobalUnrestricted: cisAnnotationValue})
			}
		}

		// apply node cluster role binding regardless of whether we're in CIS mode or not
		if _, err := cs.RbacV1().ClusterRoleBindings().Get(context.TODO(), globalUnrestrictedRoleName, metav1.GetOptions{}); err != nil {
			if apierrors.IsAlreadyExists(err) {
				logrus.Info(err)
			} else {
				tmpl := fmt.Sprintf(nodeClusterRoleBindingTemplate, globalUnrestrictedPSPName)
				if err := deployClusterRoleBindingFromYaml(cs, tmpl); err != nil {
					return err
				}
			}
		}
		return nil
	}
}

type deployFn func(*kubernetes.Clientset, interface{}) error

// newClient create a new Kubernetes client from configuration.
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

func decodeYamlResource(data interface{}, yaml string) error {
	decoder := yamlutil.NewYAMLToJSONDecoder(bytes.NewReader([]byte(yaml)))
	return decoder.Decode(data)
}

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

func deployPodSecurityPolicyFromYaml(cs *kubernetes.Clientset, pspYaml string) error {
	var psp v1beta1.PodSecurityPolicy
	if err := decodeYamlResource(&psp, pspYaml); err != nil {
		return err
	}
	return retryTo(deployPodSecurityPolicy, cs, psp, defaultRetries, defaultWaitSeconds)
}

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

func deployClusterRoleBindingFromYaml(cs *kubernetes.Clientset, clusterRoleBindingYaml string) error {
	var clusterRoleBinding rbacv1.ClusterRoleBinding
	if err := decodeYamlResource(&clusterRoleBinding, clusterRoleBindingYaml); err != nil {
		return err
	}
	return retryTo(deployClusterRoleBinding, cs, clusterRoleBinding, defaultRetries, defaultWaitSeconds)
}

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

func deployClusterRoleFromYaml(cs *kubernetes.Clientset, clusterRoleYaml string) error {
	var clusterRole rbacv1.ClusterRole
	if err := decodeYamlResource(&clusterRole, clusterRoleYaml); err != nil {
		return err
	}
	return retryTo(deployClusterRole, cs, clusterRole, defaultRetries, defaultWaitSeconds)
}

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

func deployRoleBindingFromYaml(cs *kubernetes.Clientset, roleBindingYaml string) error {
	var roleBinding rbacv1.RoleBinding
	if err := decodeYamlResource(&roleBinding, roleBindingYaml); err != nil {
		return err
	}
	return retryTo(deployRoleBinding, cs, roleBinding, defaultRetries, defaultWaitSeconds)
}

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
