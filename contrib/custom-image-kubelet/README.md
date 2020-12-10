RKE2 Image / Kubelet Override
=====

This repo contains a python script that will generate configuration files and extract binaries from a Kubernetes release manifest YAML.
Releases should be in the format described by [releases.distro.eks.amazonaws.com/v1alpha1 Release](https://github.com/aws/eks-distro-build-tooling/blob/main/release/config/crds/distro.eks.amazonaws.com_releases.yaml).
One example of a vendor providing releases in this format is [EKS Distro](https://github.com/aws/eks-distro#releases).

The resulting configuration will override binaries and images for the following components:
* coredns
* etcd
* kube-apiserver
* kube-controller-manager
* kube-proxy
* kube-scheduler
* kubelet
* metrics-server
* pause

The remaining RKE2 components include:
* Helm Controller
* Calico+Flannel CNI
* Nginx Ingress

Requirements
----

1. RKE2 v1.18.13+rke2r1 or newer (with image/kubelet override support)
1. Python 3
1. Access to AWS ECR (either via AWS CLI IAM keys, or EC2 instance role)
    *This is only necessary if the replacement RKE2 images are stored in ECR*

Installing
-----

On an Ubuntu host:

```bash
curl -sfL https://get.rke2.io | INSTALL_RKE2_VERSION=v1.18.13+rke2r1 sh -

sudo apt update
sudo apt install -y python3-venv python3-wheel python3-pip
python3 -m venv ~/python3
. ~/python3/bin/activate

git clone --shallow git@github.com:rancher/rke2.git
cd rke2/contrib/custom-image-kubelet

pip install -r requirements.txt
```

Running
-----

```bash
sudo ~/python3/bin/python genconfig.py --release-url https://X/kubernetes-1-18/kubernetes-1-18.yaml

systemctl start rke2-server
```

You may also generate the files on an administrative host that can then be embedded into deployment pipelines or copied to multiple hosts:

```bash
./genconfig.py --prefix ./kubernetes-1-18/ --release-url https://X/kubernetes-1-18/kubernetes-1-18.yaml
```

Example Output
-----

```
I Got Release: kubernetes-1-18-9
I Writing HelmChartConfig to /var/lib/rancher/rke2/server/manifests/rke2-kube-proxy-config.yaml
I Writing HelmChartConfig to /var/lib/rancher/rke2/server/manifests/rke2-coredns-config.yaml
I Writing HelmChartConfig to /var/lib/rancher/rke2/server/manifests/rke2-metrics-server-config.yaml
I Extracting files from https://X/kubernetes-1-18/releases/1/artifacts/kubernetes/v1.18.9/kubernetes-node-linux-amd64.tar.gz
I Extracting /var/lib/rancher/rke2/opt/bin/kube-proxy
I Extracting /var/lib/rancher/rke2/opt/bin/kubelet
I Extracting /var/lib/rancher/rke2/opt/bin/kubeadm
I Getting auth tokens for ['X'] in us-east-1
I Writing credentials to /etc/rancher/rke2/registries.yaml
I Writing config to /etc/rancher/rke2/config.yaml
```
