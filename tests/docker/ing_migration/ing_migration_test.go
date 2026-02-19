package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/rancher/rke2/tests"
	"github.com/rancher/rke2/tests/docker"
	"golang.org/x/crypto/bcrypt"
)

var (
	serverCount = flag.Int("serverCount", 1, "number of server nodes")
	agentCount  = flag.Int("agentCount", 1, "number of agent nodes")
	ci          = flag.Bool("ci", false, "running on CI, force cleanup")
	registry    = flag.Bool("registry", false, "start and use a local registry mirror as a pull-through cache")

	tc               *docker.TestConfig
	dualManifestFile = ""
)

// replaceConfigYaml replaces the rke2 config.yaml on the provided node
func replaceConfigYaml(config string, node docker.DockerNode) error {
	tempCnf, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		return err
	}
	defer os.Remove(tempCnf.Name())

	err = os.WriteFile(tempCnf.Name(), []byte(config), 0644)
	if err != nil {
		return err
	}
	cmd := fmt.Sprintf("docker cp %s %s:/etc/rancher/rke2/config.yaml", tempCnf.Name(), node.Name)
	_, err = docker.RunCommand(cmd)
	return err
}

func Test_DockerTraefik(t *testing.T) {
	RegisterFailHandler(Fail)
	flag.Parse()
	RunSpecs(t, "Traefik Docker Test Suite")
}

var _ = Describe("Traefik Tests", Ordered, func() {

	Context("Setup Cluster", func() {
		It("should provision servers and agents", func() {
			var err error
			tc, err = docker.NewTestConfig()
			Expect(err).NotTo(HaveOccurred())
			tc.ServerYaml = "ingress-controller: ingress-nginx"
			if *registry {
				Expect(tc.ProvisionRegistries()).To(Succeed())
			}
			Expect(tc.ProvisionServers(*serverCount)).To(Succeed())
			Expect(tc.ProvisionAgents(*agentCount)).To(Succeed())
			Expect(docker.RestartCluster(append(tc.Servers, tc.Agents...))).To(Succeed())
			Expect(tc.CopyAndModifyKubeconfig()).To(Succeed())
			Eventually(func(g Gomega) {
				g.Expect(tests.CheckDefaultDeployments(tc.KubeconfigFile)).To(Succeed())
				g.Expect(tests.CheckDaemonSets([]string{"rke2-canal", "rke2-ingress-nginx-controller"}, tc.KubeconfigFile)).To(Succeed())
			}, "240s", "5s").Should(Succeed())
			Eventually(func() error {
				return tests.NodesReady(tc.KubeconfigFile, tc.GetNodeNames())
			}, "40s", "5s").Should(Succeed())
		})
	})
	Context("Deploy all ingress workloads", func() {
		It("should deploy web server and ingresses", func() {
			_, err := tc.DeployWorkload("ingress_with_ann.yaml")
			Expect(err).NotTo(HaveOccurred())
			_, err = tc.DeployWorkload("dummy_tls_secret.yaml")
			Expect(err).NotTo(HaveOccurred())

			hPass, err := bcrypt.GenerateFromPassword([]byte("itsASecret"), bcrypt.DefaultCost)
			Expect(err).NotTo(HaveOccurred())

			authFileContent := fmt.Sprintf("user:%s", hPass)
			authFilePath := tc.TestDir + "/auth_file.txt"
			Expect(os.WriteFile(authFilePath, []byte(authFileContent), 0644)).To(Succeed())

			cmd := fmt.Sprintf("kubectl create secret generic basic-auth-secret --from-file=auth=%s -n test-migration --kubeconfig=%s", authFilePath, tc.KubeconfigFile)
			_, err = docker.RunCommand(cmd)
			Expect(err).NotTo(HaveOccurred(), "failed to create basic-auth-secret")
			Expect(os.Remove(authFilePath)).To(Succeed())

		})
		It("should return 200 on a simple app via node IP", func() {
			cmd := "curl -s -o /dev/null --max-time 10 -w '%{http_code}' -H 'Host: simple.example.com' http://" + tc.Servers[0].IP
			Eventually(func() (string, error) {
				return docker.RunCommand(cmd)
			}, "30s", "5s").Should(Equal("200"), "failed to curl simple.example.com")
		})
		It("should return 401 for auth endpoint without credentials", func() {
			cmd := "curl -s -o /dev/null --max-time 10 -w '%{http_code}' -H 'Host: auth.example.com' http://" + tc.Servers[0].IP
			Eventually(func() (string, error) {
				return docker.RunCommand(cmd)
			}, "30s", "5s").Should(Equal("401"))
		})
		It("should return 200 for auth endpoint with valid credentials", func() {
			cmd := "curl -s -o /dev/null --max-time 10 -w '%{http_code}' -u 'user:itsASecret' -H 'Host: auth.example.com' http://" + tc.Servers[0].IP
			Eventually(func() (string, error) {
				return docker.RunCommand(cmd)
			}, "30s", "5s").Should(Equal("200"), "failed to curl auth.example.com with credentials")
		})
		It("should rewrite paths correctly", func() {
			cmd := "curl -s -o /dev/null --max-time 10 -w '%{http_code}' -H 'Host: nonworking.rewrite.example.com' http://" + tc.Servers[0].IP + "/app/test"
			Eventually(func() (string, error) {
				return docker.RunCommand(cmd)
			}, "30s", "5s").Should(Equal("200"), "failed to curl rewrite endpoin")
		})
		It("should maintain session affinity with cookies", func() {
			By("getting initial response and extracting hostname")
			cookieFile := tc.TestDir + "/cookie-jar.txt"
			cmd := "curl -s -i -c " + cookieFile + " -H 'Host: cookie.example.com' http://" + tc.Servers[0].IP
			initialRes, err := docker.RunCommand(cmd)
			Expect(err).NotTo(HaveOccurred(), "failed to get initial cookie response: "+initialRes)
			Expect(initialRes).To(MatchRegexp(`Hostname:\s+\w+`))

			initialHostnameCmd := "curl -s -i -c " + cookieFile + " -H 'Host: cookie.example.com' http://" + tc.Servers[0].IP + " | grep 'Hostname:'"
			initialHostname, err := docker.RunCommand(initialHostnameCmd)
			Expect(err).NotTo(HaveOccurred())

			By("making 5 requests using the cookie jar and verifying same hostname is returned")
			for range 5 {
				cmd := "curl -s -b " + cookieFile + " -H 'Host: cookie.example.com' http://" + tc.Servers[0].IP + " | grep 'Hostname:'"
				Expect(docker.RunCommand(cmd)).To(Equal(initialHostname), "hostname changed, session affinity not working")
			}
			Expect(os.Remove(cookieFile)).To(Succeed())
		})
		It("should return 308 redirect for SSL redirect annotation", func() {
			cmd := "curl -s -o /dev/null --max-time 10 -w '%{http_code}' -H 'Host: ssl.redirect.example.com' http://" + tc.Servers[0].IP + "/"
			Eventually(func() (string, error) {
				return docker.RunCommand(cmd)
			}, "30s", "5s").Should(Equal("308"), "failed to curl ssl.redirect.example.com")
		})
		It("should handle upstream vhost annotations", func() {
			cmd := "curl -s -H 'Host: nonworking.upstreamvhost.example.com' http://" + tc.Servers[0].IP + "/"
			Expect(docker.RunCommand(cmd)).To(ContainSubstring("Host: isitworking"))
		})
	})
	Context("Deploy traefik as a secondary ingress controller", func() {
		It("should assign nginx ingressClassName to all existing ingress resources", func() {
			cmd := `kubectl get ingress --all-namespaces -o custom-columns='NAMESPACE:.metadata.namespace,NAME:.metadata.name' --no-headers | while read NS NAME; do kubectl patch ingress "$NAME" -n "$NS" --type=merge -p '{"spec": {"ingressClassName": "nginx"}}'; done`
			_, err := tc.Servers[0].RunCmdOnNode(cmd)
			Expect(err).NotTo(HaveOccurred(), "failed to patch existing ingress resources")

			cmd = "kubectl get ingress --all-namespaces --no-headers -o custom-columns='NAMESPACE:.metadata.namespace,NAME:.metadata.name,ICLASS:.spec.ingressClassName' --kubeconfig=" + tc.KubeconfigFile
			res, err := docker.RunCommand(cmd)
			Expect(err).NotTo(HaveOccurred(), "failed to get ingress resources:"+res)
			resArray := strings.Split(res, "\n")
			// Last entry is always an empty string
			resArray = resArray[:len(resArray)-1]
			Expect(resArray).To(HaveLen(6))
			Expect(resArray).To(HaveEach(ContainSubstring("nginx")))
		})
		It("restart rke2 with traefik ingress controller", func() {
			newServerYaml := "ingress-controller:\n  - ingress-nginx\n  - traefik"
			Expect(replaceConfigYaml(newServerYaml, tc.Servers[0])).To(Succeed())

			dualIngressManifest := `
apiVersion: helm.cattle.io/v1
kind: HelmChartConfig
metadata:
  name: rke2-traefik
  namespace: kube-system
spec:
  valuesContent: |-
    ports:
      web:
        hostPort: 8000
      websecure:
        hostPort: 8443
    providers:
      kubernetesIngressNginx:
        enabled: true
        ingressClass: "rke2-ingress-nginx-migration"
        controllerClass: 'rke2.cattle.io/ingress-nginx-migration'
`
			var err error
			dualManifestFile, err = docker.StageManifest(dualIngressManifest, tc.Servers)
			Expect(err).NotTo(HaveOccurred())
			Expect(docker.RestartCluster(append(tc.Servers, tc.Agents...))).To(Succeed())
			Eventually(func(g Gomega) {
				g.Expect(tests.CheckDefaultDeployments(tc.KubeconfigFile)).To(Succeed())
				g.Expect(tests.CheckDaemonSets([]string{"rke2-canal", "rke2-ingress-nginx-controller", "rke2-traefik"}, tc.KubeconfigFile)).To(Succeed())
			}, "240s", "5s").Should(Succeed())
		})
		It("should have traefik available as an ingressClass", func() {
			cmd := `kubectl get ingressclass -o 'custom-columns=NAME:.metadata.name,CONTROLLER:.spec.controller,DEFAULT:.metadata.annotations.ingressclass\.kubernetes\.io/is-default-class' --kubeconfig=` + tc.KubeconfigFile
			res, err := docker.RunCommand(cmd)
			Expect(err).NotTo(HaveOccurred(), "failed to get ingressclass:"+res)
			Expect(res).To(MatchRegexp(`nginx\s+k8s\.io\/ingress-nginx\s+<none>`), "ingress-nginx ingressclass not found or not marked default")
			Expect(res).To(MatchRegexp(`traefik\s+traefik\.io\/ingress-controller\s+false`), "traefik ingressclass not found")
		})
	})
	Context("Test sample ingress workload via Traefik ports", func() {
		It("should duplicate the ingresses for migration", func() {

			ingresses := []string{"simple", "auth", "cookie", "nonworking-rewrite", "ssl-redirect", "upstream-vhost"}

			for _, ingressName := range ingresses {
				cmd := "kubectl get ingress " + ingressName + " -n test-migration --kubeconfig=" + tc.KubeconfigFile + " -o json | jq 'del(.metadata.resourceVersion, .metadata.uid, .metadata.creationTimestamp, .metadata.generation, .status)' > ingress-" + ingressName + ".json"
				_, err := docker.RunCommand(cmd)
				Expect(err).NotTo(HaveOccurred(), "failed to get "+ingressName+" ingress resource")
				cmd = "cat ingress-" + ingressName + ".json | jq '.metadata.name = \"" + ingressName + "-traefik\" | .spec.ingressClassName = \"rke2-ingress-nginx-migration\"' | kubectl apply --kubeconfig=" + tc.KubeconfigFile + " -f -"
				_, err = docker.RunCommand(cmd)
				Expect(err).NotTo(HaveOccurred(), "failed to apply "+ingressName+"-traefik ingress resource")
				Expect(os.Remove("ingress-" + ingressName + ".json")).To(Succeed())
			}
		})
		It("should return 200 on a simple app via node IP", func() {
			cmd := "curl -s -o /dev/null --max-time 10 -w '%{http_code}' -H 'Host: simple.example.com' http://" + tc.Servers[0].IP + ":8000"
			Eventually(func() (string, error) {
				return docker.RunCommand(cmd)
			}, "30s", "5s").Should(Equal("200"), "failed to curl simple.example.com")
		})
		It("should return 401 for auth endpoint without credentials", func() {
			cmd := "curl -s -o /dev/null --max-time 10 -w '%{http_code}' -H 'Host: auth.example.com' http://" + tc.Servers[0].IP + ":8000"
			Eventually(func() (string, error) {
				return docker.RunCommand(cmd)
			}, "30s", "5s").Should(Equal("401"))
		})
		It("should return 200 for auth endpoint with valid credentials", func() {
			cmd := "curl -s -o /dev/null --max-time 10 -w '%{http_code}' -u 'user:itsASecret' -H 'Host: auth.example.com' http://" + tc.Servers[0].IP + ":8000"
			Eventually(func() (string, error) {
				return docker.RunCommand(cmd)
			}, "30s", "5s").Should(Equal("200"), "failed to curl auth.example.com with credentials")
		})
		It("should not rewrite paths correctly", func() {
			cmd := "curl -s -o /dev/null --max-time 10 -w '%{http_code}' -H 'Host: nonworking.rewrite.example.com' http://" + tc.Servers[0].IP + ":8000/app/test"
			Eventually(func() (string, error) {
				return docker.RunCommand(cmd)
			}, "30s", "5s").Should(Equal("404"), "curl rewrite endpoint sucedded when it should not have")
		})
		It("should maintain session affinity with cookies", func() {
			By("getting initial response and extracting hostname")
			cookieFile := tc.TestDir + "/cookie-jar.txt"
			cmd := "curl -s -i -c " + cookieFile + " -H 'Host: cookie.example.com' http://" + tc.Servers[0].IP + ":8000"
			initialRes, err := docker.RunCommand(cmd)
			Expect(err).NotTo(HaveOccurred(), "failed to get initial cookie response: "+initialRes)
			Expect(initialRes).To(MatchRegexp(`Hostname:\s+\w+`))

			initialHostnameCmd := "curl -s -i -c " + cookieFile + " -H 'Host: cookie.example.com' http://" + tc.Servers[0].IP + ":8000 | grep 'Hostname:'"
			initialHostname, err := docker.RunCommand(initialHostnameCmd)
			Expect(err).NotTo(HaveOccurred())

			By("making 5 requests using the cookie jar and verifying same hostname is returned")
			for range 5 {
				cmd := "curl -s -b " + cookieFile + " -H 'Host: cookie.example.com' http://" + tc.Servers[0].IP + ":8000 | grep 'Hostname:'"
				Expect(docker.RunCommand(cmd)).To(Equal(initialHostname), "hostname changed, session affinity not working")
			}
			Expect(os.Remove(cookieFile)).To(Succeed())
		})
		It("should return 308 redirect for SSL redirect annotation", func() {
			cmd := "curl -s -o /dev/null --max-time 10 -w '%{http_code}' -H 'Host: ssl.redirect.example.com' http://" + tc.Servers[0].IP + ":8000/"
			Eventually(func() (string, error) {
				return docker.RunCommand(cmd)
			}, "30s", "5s").Should(Equal("308"), "failed to curl ssl.redirect.example.com")
		})
		It("should not handle upstream vhost annotations", func() {
			cmd := "curl -s -H 'Host: nonworking.upstreamvhost.example.com' http://" + tc.Servers[0].IP + ":8000/"
			res, err := docker.RunCommand(cmd)
			Expect(err).NotTo(HaveOccurred(), "failed to curl upstreamvhost endpoint:"+res)
			Expect(res).NotTo(ContainSubstring("Host: isitworking"))
			Expect(res).To(ContainSubstring("Host: nonworking.upstreamvhost"))
		})
	})
	Context("Switch to traefik as the default ingress controller", func() {
		It("restart rke2 with traefik as default ingress controller", func() {
			newServerYaml := "ingress-controller: traefik"
			Expect(replaceConfigYaml(newServerYaml, tc.Servers[0])).To(Succeed())
			By("Updating traefik helm chart with the ingress-nginx compatibility settings")
			Expect(docker.RemoveManifest(dualManifestFile, tc.Servers)).To(Succeed())
			traefikManifest := `
apiVersion: helm.cattle.io/v1
kind: HelmChartConfig
metadata:
  name: rke2-traefik
  namespace: kube-system
spec:
  valuesContent: |-
    providers:
      kubernetesIngressNginx:
        enabled: true
        ingressClass: "nginx"
        controllerClass: 'rke2.cattle.io/ingress-nginx-migration'
`
			_, err := docker.StageManifest(traefikManifest, tc.Servers)
			Expect(err).To(Succeed())
			Expect(docker.RestartCluster(append(tc.Servers, tc.Agents...))).To(Succeed())
			Eventually(func(g Gomega) {
				g.Expect(tests.CheckDefaultDeployments(tc.KubeconfigFile)).To(Succeed())
				g.Expect(tests.CheckDaemonSets([]string{"rke2-canal", "rke2-traefik"}, tc.KubeconfigFile)).To(Succeed())
			}, "240s", "5s").Should(Succeed())
		})
		It("should have traefik is the only ingressClass and marked as default", func() {
			cmd := `kubectl get ingressclass -o 'custom-columns=NAME:.metadata.name,CONTROLLER:.spec.controller,DEFAULT:.metadata.annotations.ingressclass\.kubernetes\.io/is-default-class' --kubeconfig=` + tc.KubeconfigFile
			res, err := docker.RunCommand(cmd)
			Expect(err).NotTo(HaveOccurred(), "failed to get ingressclass:"+res)
			Expect(res).NotTo(MatchRegexp(`nginx\s+k8s\.io\/ingress-nginx`), "ingress-nginx ingressclass was still found")
			Expect(res).To(MatchRegexp(`traefik\s+traefik\.io\/ingress-controller\s+true`), "traefik ingressclass not found or not marked default")
		})
	})
	Context("traefik takes over original ingress resources", func() {
		It("should return 200 on a simple app via node IP", func() {
			cmd := "curl -s -o /dev/null --max-time 10 -w '%{http_code}' -H 'Host: simple.example.com' http://" + tc.Servers[0].IP
			Eventually(func() (string, error) {
				return docker.RunCommand(cmd)
			}, "60s", "5s").Should(Equal("200"), "failed to curl simple.example.com")
		})
		It("should return 401 for auth endpoint without credentials", func() {
			cmd := "curl -s -o /dev/null --max-time 10 -w '%{http_code}' -H 'Host: auth.example.com' http://" + tc.Servers[0].IP
			Eventually(func() (string, error) {
				return docker.RunCommand(cmd)
			}, "30s", "5s").Should(Equal("401"))
		})
		It("should return 200 for auth endpoint with valid credentials", func() {
			cmd := "curl -s -o /dev/null --max-time 10 -w '%{http_code}' -u 'user:itsASecret' -H 'Host: auth.example.com' http://" + tc.Servers[0].IP
			Eventually(func() (string, error) {
				return docker.RunCommand(cmd)
			}, "30s", "5s").Should(Equal("200"), "failed to curl auth.example.com with credentials")
		})
	})
	Context("Cleanup migration ingress resources", func() {
		It("should remove all XXXX-traefik objects", func() {
			ingresses := []string{"simple", "auth", "cookie", "nonworking-rewrite", "ssl-redirect", "upstream-vhost"}
			for _, ing := range ingresses {
				cmd := "kubectl delete ingress " + ing + "-traefik -n test-migration --kubeconfig=" + tc.KubeconfigFile
				_, err := docker.RunCommand(cmd)
				Expect(err).NotTo(HaveOccurred())
			}
		})
	})
})

var failed bool
var _ = AfterEach(func() {
	failed = failed || CurrentSpecReport().Failed()
})

var _ = AfterSuite(func() {
	if tc != nil && failed {
		AddReportEntry("cluster-resources", tc.DumpResources())
		AddReportEntry("pod-logs", tc.DumpPodLogs(20))
		AddReportEntry("journald-logs", tc.DumpServiceLogs(20))
		AddReportEntry("component-logs", tc.DumpComponentLogs(20))
	}
	if *ci || (tc != nil && !failed) {
		tc.Cleanup()
	}
})
