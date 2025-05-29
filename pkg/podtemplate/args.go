package podtemplate

import (
	"fmt"

	"github.com/k3s-io/k3s/pkg/cli/cmds"
	"github.com/rancher/rke2/pkg/images"
	v1 "k8s.io/api/core/v1"
)

func (c *Config) APIServer() (*Args, error) {
	imageName := images.KubeAPIServer
	image, err := c.Resolver.GetReference(imageName)
	if err != nil {
		return nil, err
	}
	if c.ImagesDir != "" {
		if err := images.Pull(c.ImagesDir, imageName, image); err != nil {
			return nil, err
		}
	}

	server := fmt.Sprintf("https://localhost:%d/", cmds.ServerConfig.APIServerPort)

	return &Args{
		Command:       "kube-apiserver",
		Image:         image,
		CISMode:       c.CISMode,
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

func (c *Config) ETCD() (*Args, error) {
	imageName := images.ETCD
	image, err := c.Resolver.GetReference(imageName)
	if err != nil {
		return nil, err
	}
	if c.ImagesDir != "" {
		if err := images.Pull(c.ImagesDir, imageName, image); err != nil {
			return nil, err
		}
	}

	return &Args{
		Command:       "etcd",
		Image:         image,
		CISMode:       c.CISMode,
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

func (c *Config) Scheduler() (*Args, error) {
	imageName := images.KubeScheduler
	image, err := c.Resolver.GetReference(imageName)
	if err != nil {
		return nil, err
	}
	if c.ImagesDir != "" {
		if err := images.Pull(c.ImagesDir, imageName, image); err != nil {
			return nil, err
		}
	}

	return &Args{
		Command:       "kube-scheduler",
		Image:         image,
		CISMode:       c.CISMode,
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

func (c *Config) ControllerManager() (*Args, error) {
	imageName := images.KubeControllerManager
	image, err := c.Resolver.GetReference(imageName)
	if err != nil {
		return nil, err
	}
	if c.ImagesDir != "" {
		if err := images.Pull(c.ImagesDir, imageName, image); err != nil {
			return nil, err
		}
	}

	return &Args{
		Command:       "kube-controller-manager",
		Image:         image,
		CISMode:       c.CISMode,
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

func (c *Config) CloudControllerManager() (*Args, error) {
	imageName := images.CloudControllerManager
	image, err := c.Resolver.GetReference(imageName)
	if err != nil {
		return nil, err
	}
	if c.ImagesDir != "" {
		if err := images.Pull(c.ImagesDir, imageName, image); err != nil {
			return nil, err
		}
	}

	return &Args{
		Command:       "cloud-controller-manager",
		Image:         image,
		CISMode:       c.CISMode,
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

func (c *Config) KubeProxy() (*Args, error) {
	imageName := images.KubeProxy
	image, err := c.Resolver.GetReference(imageName)
	if err != nil {
		return nil, err
	}
	if c.ImagesDir != "" {
		if err := images.Pull(c.ImagesDir, imageName, image); err != nil {
			return nil, err
		}
	}

	return &Args{
		Command:       "kube-proxy",
		Image:         image,
		CISMode:       c.CISMode,
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
