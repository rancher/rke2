package rke2

import (
	"bytes"
	"context"
	"fmt"
	"time"

	daemonsConfig "github.com/rancher/k3s/pkg/daemons/config"
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
	cisAnnotationValue = "resolved"

	namespaceAnnotationBase               = "psp.rke2.io/"
	namespaceAnnotationGlobalRestricted   = namespaceAnnotationBase + "global-restricted"
	namespaceAnnotationGlobalUnrestricted = namespaceAnnotationBase + "global-unrestricted"
	namespaceAnnotationSystemUnrestricted = namespaceAnnotationBase + "system-unrestricted"
)

// setGlobalUnrestricted sets the global unrestricted podSecurityPolicy with
// the associated role and rolebinding.
func setGlobalUnrestricted(ctx context.Context, cs *kubernetes.Clientset, ns *v1.Namespace) error {
	if _, ok := ns.Annotations[namespaceAnnotationGlobalUnrestricted]; !ok {
		if _, err := cs.RbacV1().ClusterRoles().Get(ctx, globalUnrestrictedPSPName, metav1.GetOptions{}); err != nil {
			if apierrors.IsNotFound(err) {
				logrus.Infof("Setting PSP: %s", globalUnrestrictedPSPName)
				tmpl := fmt.Sprintf(globalUnrestrictedPSPTemplate, globalUnrestrictedPSPName)
				if err := deployPodSecurityPolicyFromYaml(ctx, cs, tmpl); err != nil {
					return err
				}
			} else {
				return err
			}
		}
		if _, err := cs.RbacV1().ClusterRoles().Get(ctx, globalUnrestrictedRoleName, metav1.GetOptions{}); err != nil {
			if apierrors.IsNotFound(err) {
				logrus.Infof("Setting Cluster Role: %s", globalUnrestrictedRoleName)
				tmpl := fmt.Sprintf(roleTemplate, globalUnrestrictedRoleName, globalUnrestrictedPSPName)
				if err := deployClusterRoleFromYaml(ctx, cs, tmpl); err != nil {
					return err
				}
			} else {
				return err
			}
		}
		if _, err := cs.RbacV1().ClusterRoleBindings().Get(ctx, globalUnrestrictedRoleBindingName, metav1.GetOptions{}); err != nil {
			if apierrors.IsNotFound(err) {
				logrus.Infof("Setting Cluster RoleBinding: %s", globalUnrestrictedRoleBindingName)
				tmpl := fmt.Sprintf(globalUnrestrictedRoleBindingTemplate, globalUnrestrictedRoleBindingName, globalUnrestrictedRoleName)
				if err := deployClusterRoleBindingFromYaml(ctx, cs, tmpl); err != nil {
					return err
				}
			} else {
				return err
			}
		}
		ns.Annotations[namespaceAnnotationGlobalUnrestricted] = cisAnnotationValue
	}
	return nil
}

// setSystemUnrestricted sets the system unrestricted podSecurityPolicy as
// the associated role and rolebinding.
func setSystemUnrestricted(ctx context.Context, cs *kubernetes.Clientset, ns *v1.Namespace) error {
	if _, ok := ns.Annotations[namespaceAnnotationSystemUnrestricted]; !ok {
		if _, err := cs.RbacV1().ClusterRoles().Get(ctx, systemUnrestrictedPSPName, metav1.GetOptions{}); err != nil {
			if apierrors.IsNotFound(err) {
				logrus.Infof("Setting PSP: %s", systemUnrestrictedPSPName)
				tmpl := fmt.Sprintf(systemUnrestrictedPSPTemplate, systemUnrestrictedPSPName)
				if err := deployPodSecurityPolicyFromYaml(ctx, cs, tmpl); err != nil {
					return err
				}
			} else {
				return err
			}
		}
		if _, err := cs.RbacV1().ClusterRoles().Get(ctx, systemUnrestrictedRoleName, metav1.GetOptions{}); err != nil {
			if apierrors.IsNotFound(err) {
				logrus.Infof("Setting Cluster Role: %s", systemUnrestrictedRoleName)
				tmpl := fmt.Sprintf(roleTemplate, systemUnrestrictedRoleName, systemUnrestrictedPSPName)
				if err := deployClusterRoleFromYaml(ctx, cs, tmpl); err != nil {
					return err
				}
			} else {
				return err
			}
		}
		if _, err := cs.RbacV1().ClusterRoleBindings().Get(ctx, systemUnrestrictedRoleBindingName, metav1.GetOptions{}); err != nil {
			if apierrors.IsNotFound(err) {
				logrus.Infof("Setting Cluster RoleBinding: %s", systemUnrestrictedRoleBindingName)
				tmpl := fmt.Sprintf(systemUnrestrictedNodesRoleBindingTemplate, systemUnrestrictedRoleBindingName, systemUnrestrictedRoleName)
				if err := deployClusterRoleBindingFromYaml(ctx, cs, tmpl); err != nil {
					return err
				}
			} else {
				return err
			}
		}
		if _, err := cs.RbacV1().ClusterRoleBindings().Get(ctx, systemUnrestrictedSvcAcctRoleBindingName, metav1.GetOptions{}); err != nil {
			if apierrors.IsNotFound(err) {
				logrus.Infof("Setting Cluster RoleBinding: %s", systemUnrestrictedSvcAcctRoleBindingName)
				tmpl := fmt.Sprintf(systemUnrestrictedServiceAcctRoleBindingTemplate, systemUnrestrictedSvcAcctRoleBindingName, systemUnrestrictedRoleName)
				if err := deployRoleBindingFromYaml(ctx, cs, tmpl); err != nil {
					return err
				}
			} else {
				return err
			}
		}
		ns.Annotations[namespaceAnnotationSystemUnrestricted] = cisAnnotationValue
	}
	return nil
}

// setPSPs sets the default PSP's based on the mode that RKE2 is running in. There is either CIS or non
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
// - If the globalRestricted annotation does not exist, create PSP, role, binding.
// - If the systemUnrestricted annotation does not exist, create the PSP, role, binding.
// - If the globalUnrestricted annotation does not exist, check if PSP exists, and if so,
//   delete it, the role, and the bindings.
//
// non-CIS:
// - If the globalUnrestricted annotation does not exist, then create PSP, role, binding.
// - If the systemUnrestricted annotation does not exist, then create PSP, role, binding.
// - If the globalRestricted annotation does not exist, then check if the PSP exists and
//   if it doesn't, create it. Check if the associated role and bindings exist and
//   if they do, delete them.
func setPSPs(clx *cli.Context) func(context.Context, daemonsConfig.Control) error {
	return func(ctx context.Context, cfg daemonsConfig.Control) error {
		logrus.Info("Applying PSP's...")
		go func() {
			<-cfg.Runtime.APIServerReady

			cs, err := newClient(cfg.Runtime.KubeConfigAdmin, nil)
			if err != nil {
				logrus.Fatalf("psp: new k8s client: %s", err.Error())
			}

			ns, err := cs.CoreV1().Namespaces().Get(ctx, metav1.NamespaceSystem, metav1.GetOptions{})
			if err != nil {
				logrus.Fatalf("psp: get kube-system namespace: %s", err.Error())
			}
			if ns.Annotations == nil {
				ns.Annotations = make(map[string]string)
			}

			if clx.String("profile") == "" { // non-CIS mode
				if err := setGlobalUnrestricted(ctx, cs, ns); err != nil {
					logrus.Fatalf("psp: set globalUnrestricted: %s", err.Error())
				}

				if err := setSystemUnrestricted(ctx, cs, ns); err != nil {
					logrus.Fatalf("psp: set systemUnrestricted: %s", err.Error())
				}

				if _, ok := ns.Annotations[namespaceAnnotationGlobalRestricted]; !ok {
					// check if the policy exists
					if _, err := cs.PolicyV1beta1().PodSecurityPolicies().Get(ctx, globalRestrictedPSPName, metav1.GetOptions{}); err != nil {
						if apierrors.IsNotFound(err) {
							tmpl := fmt.Sprintf(globalRestrictedPSPTemplate, globalRestrictedPSPName)
							if err := deployPodSecurityPolicyFromYaml(ctx, cs, tmpl); err != nil {
								logrus.Fatalf("psp: deploy psp: %s", err.Error())
							}
						} else {
							logrus.Fatalf("psp: get psp: %s", err.Error())
						}
					}

					// check if role exists and delete it
					_, err := cs.RbacV1().ClusterRoles().Get(ctx, globalRestrictedRoleName, metav1.GetOptions{})
					if err != nil {
						switch {
						case apierrors.IsAlreadyExists(err):
							logrus.Infof("Deleting clusterRole: %s", globalRestrictedRoleName)
							if err := cs.RbacV1().ClusterRoles().Delete(ctx, globalRestrictedRoleName, metav1.DeleteOptions{}); err != nil {
								logrus.Fatalf("psp: delete clusterrole: %s", err.Error())
							}
						case apierrors.IsNotFound(err):
							break
						default:
							logrus.Fatalf("psp: get clusterrole: %s", err.Error())
						}
					}

					// check if role binding exists and delete it
					_, err = cs.RbacV1().ClusterRoleBindings().Get(ctx, globalRestrictedRoleBindingName, metav1.GetOptions{})
					if err != nil && !apierrors.IsNotFound(err) {
						logrus.Fatalf("psp: get clusterrole: %s", err.Error())
					}
					if err != nil {
						switch {
						case apierrors.IsAlreadyExists(err):
							logrus.Infof("Deleting clusterRole binding: %s", globalRestrictedRoleBindingName)
							if err := cs.RbacV1().ClusterRoleBindings().Delete(ctx, globalRestrictedRoleBindingName, metav1.DeleteOptions{}); err != nil {
								logrus.Fatalf("psp: delete clusterrole binding: %s", err.Error())
							}
						case apierrors.IsNotFound(err):
							break
						default:
							logrus.Fatalf("psp: get clusterrole binding: %s", err.Error())
						}
					}
					ns.Annotations[namespaceAnnotationGlobalRestricted] = cisAnnotationValue
				}
			} else { // CIS mode
				if _, ok := ns.Annotations[namespaceAnnotationGlobalRestricted]; !ok {
					if _, err := cs.PolicyV1beta1().PodSecurityPolicies().Get(ctx, globalRestrictedPSPName, metav1.GetOptions{}); err != nil {
						tmpl := fmt.Sprintf(globalRestrictedPSPTemplate, globalRestrictedPSPName)
						if err := deployPodSecurityPolicyFromYaml(ctx, cs, tmpl); err != nil {
							logrus.Fatalf("psp: delete clusterrole: %s", err.Error())
						}
					}
					if _, err := cs.RbacV1().ClusterRoles().Get(ctx, globalRestrictedRoleName, metav1.GetOptions{}); err != nil {
						if apierrors.IsNotFound(err) {
							tmpl := fmt.Sprintf(roleTemplate, globalRestrictedRoleName, globalRestrictedPSPName)
							if err := deployClusterRoleFromYaml(ctx, cs, tmpl); err != nil {
								logrus.Fatalf("psp: deploy clusterrole: %s", err.Error())
							}
						} else {
							logrus.Fatalf("psp: get clusterrole: %s", err.Error())
						}
					}
					if _, err := cs.RbacV1().ClusterRoleBindings().Get(ctx, globalRestrictedRoleBindingName, metav1.GetOptions{}); err != nil {
						if apierrors.IsNotFound(err) {
							tmpl := fmt.Sprintf(globalRestrictedRoleBindingTemplate, globalRestrictedRoleBindingName, globalRestrictedRoleName)
							if err := deployClusterRoleBindingFromYaml(ctx, cs, tmpl); err != nil {
								logrus.Fatalf("psp: deploy clusterrole binding: %s", err.Error())
							}
						} else {
							logrus.Fatalf("psp: get clusterrole binding: %s", err.Error())
						}
					}
					ns.Annotations[namespaceAnnotationGlobalRestricted] = cisAnnotationValue
				}

				if err := setSystemUnrestricted(ctx, cs, ns); err != nil {
					logrus.Fatalf("psp: set systemUnrestricted: %s", err.Error())
				}

				if _, ok := ns.Annotations[namespaceAnnotationGlobalUnrestricted]; !ok {
					if _, err := cs.PolicyV1beta1().PodSecurityPolicies().Get(ctx, globalRestrictedPSPName, metav1.GetOptions{}); err != nil {
						if apierrors.IsNotFound(err) {
							tmpl := fmt.Sprintf(globalRestrictedPSPTemplate, globalRestrictedPSPName)
							if err := deployPodSecurityPolicyFromYaml(ctx, cs, tmpl); err != nil {
								logrus.Fatalf("psp: deploy psp: %s", err.Error())
							}
						} else {
							logrus.Fatalf("psp: get psp: %s", err.Error())
						}
					}

					// check if role exists and if so, delete it
					_, err := cs.RbacV1().ClusterRoles().Get(ctx, globalUnrestrictedRoleName, metav1.GetOptions{})
					if err != nil {
						switch {
						case apierrors.IsAlreadyExists(err):
							logrus.Infof("Deleting clusterRole: %s", globalUnrestrictedRoleName)
							if err := cs.RbacV1().ClusterRoles().Delete(ctx, globalUnrestrictedRoleName, metav1.DeleteOptions{}); err != nil {
								logrus.Fatalf("psp: delete clusterrole: %s", err.Error())
							}
						case apierrors.IsNotFound(err):
							break
						default:
							logrus.Fatalf("psp: get clusterrole: %s", err.Error())
						}
					}

					// check if role binding exists and if so, delete it
					_, err = cs.RbacV1().ClusterRoleBindings().Get(ctx, globalUnrestrictedRoleBindingName, metav1.GetOptions{})
					if err != nil {
						switch {
						case apierrors.IsAlreadyExists(err):
							logrus.Infof("Deleting clusterRoleBinding: %s", globalUnrestrictedRoleBindingName)
							if err := cs.RbacV1().ClusterRoleBindings().Delete(ctx, globalUnrestrictedRoleBindingName, metav1.DeleteOptions{}); err != nil {
								logrus.Fatalf("psp: delete clusterrole binding: %s", err.Error())
							}
						case apierrors.IsNotFound(err):
							break
						default:
							logrus.Fatalf("psp: get clusterrole binding: %s", err.Error())
						}
					}
					ns.Annotations[namespaceAnnotationGlobalUnrestricted] = cisAnnotationValue
				}
			}

			// apply node cluster role binding regardless of whether we're in CIS mode or not
			if _, err := cs.RbacV1().ClusterRoleBindings().Get(ctx, nodeClusterRoleBindingName, metav1.GetOptions{}); err != nil {
				if !apierrors.IsAlreadyExists(err) {
					tmpl := fmt.Sprintf(nodeClusterRoleBindingTemplate, globalUnrestrictedPSPName)
					if err := deployClusterRoleBindingFromYaml(ctx, cs, tmpl); err != nil {
						logrus.Fatalf("psp: deploy psp: %s", err.Error())
					}
				} else {
					logrus.Fatalf("psp: get clusterrole binding: %s", err.Error())
				}
			}

			if _, err := cs.CoreV1().Namespaces().Update(ctx, ns, metav1.UpdateOptions{}); err != nil {
				logrus.Fatalf("psp: update namespace annotation: %s", err.Error())
			}

			logrus.Info("Applying PSP's complete")
		}()
		return nil
	}
}

type deployFn func(context.Context, *kubernetes.Clientset, interface{}) error

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

func retryTo(ctx context.Context, runFunc deployFn, cs *kubernetes.Clientset, resource interface{}, retries, wait int) error {
	var err error
	if retries <= 0 {
		retries = defaultRetries
	}
	if wait <= 0 {
		wait = defaultWaitSeconds
	}
	for i := 0; i < retries; i++ {
		if err = runFunc(ctx, cs, resource); err != nil {
			time.Sleep(time.Second * time.Duration(wait))
			continue
		}
		return nil
	}
	return err
}

func deployPodSecurityPolicyFromYaml(ctx context.Context, cs *kubernetes.Clientset, pspYaml string) error {
	var psp v1beta1.PodSecurityPolicy
	if err := decodeYamlResource(&psp, pspYaml); err != nil {
		return err
	}
	return retryTo(ctx, deployPodSecurityPolicy, cs, psp, defaultRetries, defaultWaitSeconds)
}

func deployPodSecurityPolicy(ctx context.Context, cs *kubernetes.Clientset, p interface{}) error {
	psp, ok := p.(v1beta1.PodSecurityPolicy)
	if !ok {
		return fmt.Errorf("invalid type provided: %T, expected: PodSecurityPolicy", p)
	}
	if _, err := cs.PolicyV1beta1().PodSecurityPolicies().Create(ctx, &psp, metav1.CreateOptions{}); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return err
		}
		if _, err := cs.PolicyV1beta1().PodSecurityPolicies().Update(ctx, &psp, metav1.UpdateOptions{}); err != nil {
			return err
		}
	}
	return nil
}

func deployClusterRoleBindingFromYaml(ctx context.Context, cs *kubernetes.Clientset, clusterRoleBindingYaml string) error {
	var clusterRoleBinding rbacv1.ClusterRoleBinding
	if err := decodeYamlResource(&clusterRoleBinding, clusterRoleBindingYaml); err != nil {
		return err
	}
	return retryTo(ctx, deployClusterRoleBinding, cs, clusterRoleBinding, defaultRetries, defaultWaitSeconds)
}

func deployClusterRoleBinding(ctx context.Context, cs *kubernetes.Clientset, crb interface{}) error {
	clusterRoleBinding, ok := crb.(rbacv1.ClusterRoleBinding)
	if !ok {
		return fmt.Errorf("invalid type provided: %T, expected: ClusterRoleBinding", crb)
	}
	if _, err := cs.RbacV1().ClusterRoleBindings().Create(ctx, &clusterRoleBinding, metav1.CreateOptions{}); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return err
		}
		if _, err := cs.RbacV1().ClusterRoleBindings().Update(ctx, &clusterRoleBinding, metav1.UpdateOptions{}); err != nil {
			return err
		}
	}
	return nil
}

func deployClusterRoleFromYaml(ctx context.Context, cs *kubernetes.Clientset, clusterRoleYaml string) error {
	var clusterRole rbacv1.ClusterRole
	if err := decodeYamlResource(&clusterRole, clusterRoleYaml); err != nil {
		return err
	}
	return retryTo(ctx, deployClusterRole, cs, clusterRole, defaultRetries, defaultWaitSeconds)
}

func deployClusterRole(ctx context.Context, cs *kubernetes.Clientset, cr interface{}) error {
	clusterRole, ok := cr.(rbacv1.ClusterRole)
	if !ok {
		return fmt.Errorf("invalid type provided: %T, expected: ClusterRole", cr)
	}
	if _, err := cs.RbacV1().ClusterRoles().Create(ctx, &clusterRole, metav1.CreateOptions{}); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return err
		}
		if _, err := cs.RbacV1().ClusterRoles().Update(ctx, &clusterRole, metav1.UpdateOptions{}); err != nil {
			return err
		}
	}
	return nil
}

func deployRoleBindingFromYaml(ctx context.Context, cs *kubernetes.Clientset, roleBindingYaml string) error {
	var roleBinding rbacv1.RoleBinding
	if err := decodeYamlResource(&roleBinding, roleBindingYaml); err != nil {
		return err
	}
	return retryTo(ctx, deployRoleBinding, cs, roleBinding, defaultRetries, defaultWaitSeconds)
}

func deployRoleBinding(ctx context.Context, cs *kubernetes.Clientset, rb interface{}) error {
	roleBinding, ok := rb.(rbacv1.RoleBinding)
	if !ok {
		return fmt.Errorf("invalid type provided: %T, expected: RoleBinding", rb)
	}
	if _, err := cs.RbacV1().RoleBindings(roleBinding.Namespace).Create(ctx, &roleBinding, metav1.CreateOptions{}); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return err
		}
		if _, err := cs.RbacV1().RoleBindings(roleBinding.Namespace).Update(ctx, &roleBinding, metav1.UpdateOptions{}); err != nil {
			return err
		}
	}
	return nil
}
