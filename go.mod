module github.com/rancher/rke2

go 1.15

replace (
	github.com/Microsoft/go-winio => github.com/Microsoft/go-winio v0.4.19
	github.com/Microsoft/hcsshim => github.com/Microsoft/hcsshim v0.8.20
	github.com/benmoss/go-powershell => github.com/k3s-io/go-powershell v0.0.0-20201118222746-51f4c451fbd7
	github.com/containerd/containerd => github.com/k3s-io/containerd v1.4.12-k3s1
	github.com/containerd/continuity => github.com/k3s-io/continuity v0.0.0-20210309170710-f93269e0d5c1
	github.com/containerd/cri => github.com/k3s-io/cri v1.4.0-k3s.7 // k3s-release/1.4
	github.com/coreos/flannel => github.com/rancher/flannel v0.12.0-k3s1
	github.com/docker/distribution => github.com/docker/distribution v2.7.1+incompatible
	github.com/docker/docker => github.com/docker/docker v20.10.2+incompatible
	github.com/docker/libnetwork => github.com/docker/libnetwork v0.8.0-dev.2.0.20190624125649-f0e46a78ea34
	github.com/golang/protobuf => github.com/k3s-io/protobuf v1.4.3-k3s1
	github.com/juju/errors => github.com/k3s-io/nocode v0.0.0-20200630202308-cb097102c09f
	github.com/kubernetes-sigs/cri-tools => github.com/k3s-io/cri-tools v1.21.0-k3s1
	github.com/matryer/moq => github.com/rancher/moq v0.0.0-20190404221404-ee5226d43009
	github.com/opencontainers/runc => github.com/opencontainers/runc v1.0.3
	github.com/opencontainers/runtime-spec => github.com/opencontainers/runtime-spec v1.0.3-0.20210316141917-a8c4a9ee0f6b
	go.etcd.io/etcd => github.com/k3s-io/etcd v0.5.0-alpha.5.0.20220113195313-6c2233a709e8
	google.golang.org/genproto => google.golang.org/genproto v0.0.0-20200513103714-09dca8ec2884
	google.golang.org/grpc => google.golang.org/grpc v1.27.1
	gopkg.in/square/go-jose.v2 => gopkg.in/square/go-jose.v2 v2.2.2
	k8s.io/api => github.com/k3s-io/kubernetes/staging/src/k8s.io/api v1.21.8-k3s1
	k8s.io/apiextensions-apiserver => github.com/k3s-io/kubernetes/staging/src/k8s.io/apiextensions-apiserver v1.21.8-k3s1
	k8s.io/apimachinery => github.com/k3s-io/kubernetes/staging/src/k8s.io/apimachinery v1.21.8-k3s1
	k8s.io/apiserver => github.com/k3s-io/kubernetes/staging/src/k8s.io/apiserver v1.21.8-k3s1
	k8s.io/cli-runtime => github.com/k3s-io/kubernetes/staging/src/k8s.io/cli-runtime v1.21.8-k3s1
	k8s.io/client-go => github.com/k3s-io/kubernetes/staging/src/k8s.io/client-go v1.21.8-k3s1
	k8s.io/cloud-provider => github.com/k3s-io/kubernetes/staging/src/k8s.io/cloud-provider v1.21.8-k3s1
	k8s.io/cluster-bootstrap => github.com/k3s-io/kubernetes/staging/src/k8s.io/cluster-bootstrap v1.21.8-k3s1
	k8s.io/code-generator => github.com/k3s-io/kubernetes/staging/src/k8s.io/code-generator v1.21.8-k3s1
	k8s.io/component-base => github.com/k3s-io/kubernetes/staging/src/k8s.io/component-base v1.21.8-k3s1
	k8s.io/component-helpers => github.com/k3s-io/kubernetes/staging/src/k8s.io/component-helpers v1.21.8-k3s1
	k8s.io/controller-manager => github.com/k3s-io/kubernetes/staging/src/k8s.io/controller-manager v1.21.8-k3s1
	k8s.io/cri-api => github.com/k3s-io/kubernetes/staging/src/k8s.io/cri-api v1.21.8-k3s1
	k8s.io/csi-translation-lib => github.com/k3s-io/kubernetes/staging/src/k8s.io/csi-translation-lib v1.21.8-k3s1
	k8s.io/kube-aggregator => github.com/k3s-io/kubernetes/staging/src/k8s.io/kube-aggregator v1.21.8-k3s1
	k8s.io/kube-controller-manager => github.com/k3s-io/kubernetes/staging/src/k8s.io/kube-controller-manager v1.21.8-k3s1
	k8s.io/kube-proxy => github.com/k3s-io/kubernetes/staging/src/k8s.io/kube-proxy v1.21.8-k3s1
	k8s.io/kube-scheduler => github.com/k3s-io/kubernetes/staging/src/k8s.io/kube-scheduler v1.21.8-k3s1
	k8s.io/kubectl => github.com/k3s-io/kubernetes/staging/src/k8s.io/kubectl v1.21.8-k3s1
	k8s.io/kubelet => github.com/k3s-io/kubernetes/staging/src/k8s.io/kubelet v1.21.8-k3s1
	k8s.io/kubernetes => github.com/k3s-io/kubernetes v1.21.8-k3s1
	k8s.io/legacy-cloud-providers => github.com/k3s-io/kubernetes/staging/src/k8s.io/legacy-cloud-providers v1.21.8-k3s1
	k8s.io/metrics => github.com/k3s-io/kubernetes/staging/src/k8s.io/metrics v1.21.8-k3s1
	k8s.io/mount-utils => github.com/k3s-io/kubernetes/staging/src/k8s.io/mount-utils v1.21.8-k3s1
	k8s.io/node-api => github.com/k3s-io/kubernetes/staging/src/k8s.io/node-api v1.21.8-k3s1
	k8s.io/sample-apiserver => github.com/k3s-io/kubernetes/staging/src/k8s.io/sample-apiserver v1.21.8-k3s1
	k8s.io/sample-cli-plugin => github.com/k3s-io/kubernetes/staging/src/k8s.io/sample-cli-plugin v1.21.8-k3s1
	k8s.io/sample-controller => github.com/k3s-io/kubernetes/staging/src/k8s.io/sample-controller v1.21.8-k3s1
	mvdan.cc/unparam => mvdan.cc/unparam v0.0.0-20210104141923-aac4ce9116a7
)

require (
	github.com/containerd/continuity v0.1.0
	github.com/google/go-containerregistry v0.7.0
	github.com/k3s-io/helm-controller v0.11.7
	github.com/pkg/errors v0.9.1
	github.com/rancher/k3s v1.21.8-engine0.0.20220118180854-8755fd45f6eb // engine-1.21
	github.com/rancher/wharfie v0.5.2
	github.com/rancher/wrangler v0.8.10
	github.com/sirupsen/logrus v1.8.1
	github.com/urfave/cli v1.22.4
	google.golang.org/grpc v1.42.0
	k8s.io/api v0.21.8
	k8s.io/apimachinery v0.21.8
	k8s.io/apiserver v0.21.8
	k8s.io/client-go v11.0.1-0.20190409021438-1a26190bd76a+incompatible
	k8s.io/cri-api v0.21.8
	k8s.io/kubernetes v1.21.8
	k8s.io/utils v0.0.0-20210521133846-da695404a2bc
	sigs.k8s.io/yaml v1.2.0
)
