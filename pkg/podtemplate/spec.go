package podtemplate

import (
	"fmt"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/k3s-io/k3s/pkg/cli/cmds"
	"github.com/rancher/rke2/pkg/images"
	v1 "k8s.io/api/core/v1"
)

func (c *Config) APIServer(args []string) (*Spec, error) {
	image, err := c.resolveAndPull(images.KubeAPIServer)
	if err != nil {
		return nil, err
	}

	server := fmt.Sprintf("https://localhost:%d/", cmds.ServerConfig.APIServerPort)

	return &Spec{
		Command:       "kube-apiserver",
		Args:          args,
		Image:         image,
		CPURequest:    c.Resources.KubeAPIServerCPURequest,
		CPULimit:      c.Resources.KubeAPIServerCPULimit,
		MemoryRequest: c.Resources.KubeAPIServerMemoryRequest,
		MemoryLimit:   c.Resources.KubeAPIServerMemoryLimit,
		ExtraEnv:      c.Env.KubeAPIServer,
		ExtraMounts:   c.Mounts.KubeAPIServer,
		ProbeConfs:    c.Probes.KubeAPIServer,
		StartupExec: []string{
			"kubectl",
			"get",
			"--server=" + server,
			"--client-certificate=" + c.DataDir + "/server/tls/client-kube-apiserver.crt",
			"--client-key=" + c.DataDir + "/server/tls/client-kube-apiserver.key",
			"--certificate-authority=" + c.DataDir + "/server/tls/server-ca.crt",
			"--raw=/livez",
		},
		HealthExec: []string{
			"kubectl",
			"get",
			"--server=" + server,
			"--client-certificate=" + c.DataDir + "/server/tls/client-kube-apiserver.crt",
			"--client-key=" + c.DataDir + "/server/tls/client-kube-apiserver.key",
			"--certificate-authority=" + c.DataDir + "/server/tls/server-ca.crt",
			"--raw=/livez",
		},
		ReadyExec: []string{
			"kubectl",
			"get",
			"--server=" + server,
			"--client-certificate=" + c.DataDir + "/server/tls/client-kube-apiserver.crt",
			"--client-key=" + c.DataDir + "/server/tls/client-kube-apiserver.key",
			"--certificate-authority=" + c.DataDir + "/server/tls/server-ca.crt",
			"--raw=/readyz",
		},
		Ports: []v1.ContainerPort{
			{Name: "apiserver", Protocol: v1.ProtocolTCP, ContainerPort: int32(cmds.ServerConfig.APIServerPort)},
		},
	}, nil
}

func (c *Config) ETCD(args []string) (*Spec, error) {
	image, err := c.resolveAndPull(images.ETCD)
	if err != nil {
		return nil, err
	}

	return &Spec{
		Command:       "etcd",
		Args:          args,
		Image:         image,
		HealthPort:    2381,
		HealthPath:    "/health?serializable=true",
		HealthScheme:  "HTTP",
		CPURequest:    c.Resources.EtcdCPURequest,
		CPULimit:      c.Resources.EtcdCPULimit,
		MemoryRequest: c.Resources.EtcdMemoryRequest,
		MemoryLimit:   c.Resources.EtcdMemoryLimit,
		ExtraEnv:      c.Env.Etcd,
		ExtraMounts:   c.Mounts.Etcd,
		ProbeConfs:    c.Probes.Etcd,
		Ports: []v1.ContainerPort{
			{Name: "client", Protocol: v1.ProtocolTCP, ContainerPort: 2379},
			{Name: "peer", Protocol: v1.ProtocolTCP, ContainerPort: 2380},
			{Name: "metrics", Protocol: v1.ProtocolTCP, ContainerPort: 2381},
		},
	}, nil
}

func (c *Config) Scheduler(args []string) (*Spec, error) {
	image, err := c.resolveAndPull(images.KubeScheduler)
	if err != nil {
		return nil, err
	}

	return &Spec{
		Command:       "kube-scheduler",
		Args:          args,
		Image:         image,
		HealthPort:    10259,
		HealthScheme:  "HTTPS",
		ReadyPort:     10259,
		ReadyScheme:   "HTTPS",
		ReadyPath:     "/readyz",
		StartupPort:   10259,
		StartupScheme: "HTTPS",
		CPURequest:    c.Resources.KubeSchedulerCPURequest,
		CPULimit:      c.Resources.KubeSchedulerCPULimit,
		MemoryRequest: c.Resources.KubeSchedulerMemoryRequest,
		MemoryLimit:   c.Resources.KubeSchedulerMemoryLimit,
		ExtraEnv:      c.Env.KubeScheduler,
		ExtraMounts:   c.Mounts.KubeScheduler,
		ProbeConfs:    c.Probes.KubeScheduler,
		Ports: []v1.ContainerPort{
			{Name: "metrics", Protocol: v1.ProtocolTCP, ContainerPort: 10259},
		},
	}, nil
}

func (c *Config) ControllerManager(args []string) (*Spec, error) {
	image, err := c.resolveAndPull(images.KubeControllerManager)
	if err != nil {
		return nil, err
	}

	return &Spec{
		Command:       "kube-controller-manager",
		Args:          args,
		Image:         image,
		HealthPort:    10257,
		HealthScheme:  "HTTPS",
		HealthPath:    "/healthz",
		StartupPort:   10257,
		StartupScheme: "HTTPS",
		StartupPath:   "/healthz",
		CPURequest:    c.Resources.KubeControllerManagerCPURequest,
		CPULimit:      c.Resources.KubeControllerManagerCPULimit,
		MemoryRequest: c.Resources.KubeControllerManagerMemoryRequest,
		MemoryLimit:   c.Resources.KubeControllerManagerMemoryLimit,
		ExtraEnv:      c.Env.KubeControllerManager,
		ExtraMounts:   c.Mounts.KubeControllerManager,
		ProbeConfs:    c.Probes.KubeControllerManager,
		Ports: []v1.ContainerPort{
			{Name: "metrics", Protocol: v1.ProtocolTCP, ContainerPort: 10257},
		},
	}, nil
}

func (c *Config) CloudControllerManager(args []string) (*Spec, error) {
	image, err := c.resolveAndPull(images.CloudControllerManager)
	if err != nil {
		return nil, err
	}

	return &Spec{
		Command:       "cloud-controller-manager",
		Args:          args,
		Image:         image,
		HealthPort:    10258,
		HealthScheme:  "HTTPS",
		HealthPath:    "/healthz",
		StartupPort:   10258,
		StartupScheme: "HTTPS",
		StartupPath:   "/healthz",
		CPURequest:    c.Resources.CloudControllerManagerCPURequest,
		CPULimit:      c.Resources.CloudControllerManagerCPULimit,
		MemoryRequest: c.Resources.CloudControllerManagerMemoryRequest,
		MemoryLimit:   c.Resources.CloudControllerManagerMemoryLimit,
		ExtraEnv:      c.Env.CloudControllerManager,
		ExtraMounts:   c.Mounts.CloudControllerManager,
		ProbeConfs:    c.Probes.CloudControllerManager,
		Ports: []v1.ContainerPort{
			{Name: "metrics", Protocol: v1.ProtocolTCP, ContainerPort: 10258},
		},
	}, nil
}

func (c *Config) KubeProxy(args []string) (*Spec, error) {
	image, err := c.resolveAndPull(images.KubeProxy)
	if err != nil {
		return nil, err
	}

	return &Spec{
		Command:       "kube-proxy",
		Args:          args,
		Image:         image,
		HealthPort:    10256,
		HealthScheme:  "HTTP",
		CPURequest:    c.Resources.KubeProxyCPURequest,
		CPULimit:      c.Resources.KubeProxyCPULimit,
		MemoryRequest: c.Resources.KubeProxyMemoryRequest,
		MemoryLimit:   c.Resources.KubeProxyMemoryLimit,
		ExtraEnv:      c.Env.KubeProxy,
		ExtraMounts:   c.Mounts.KubeProxy,
		ProbeConfs:    c.Probes.KubeProxy,
		Privileged:    true,
		Ports: []v1.ContainerPort{
			{Name: "metrics", Protocol: v1.ProtocolTCP, ContainerPort: 10256},
		},
	}, nil
}

func (c *Config) resolveAndPull(imageName string) (name.Reference, error) {
	image, err := c.Resolver.GetReference(imageName)
	if err != nil {
		return image, err
	}
	if c.ImagesDir != "" {
		if err := images.Pull(c.ImagesDir, imageName, image); err != nil {
			return image, err
		}
	}
	return image, nil
}
