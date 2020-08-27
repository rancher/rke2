# -*- mode: ruby -*-
# vi: set ft=ruby :

# Adapted from https://github.com/containerd/containerd/pull/4451
Vagrant.configure("2") do |config|
  config.vm.box = "centos/7"
  config.vm.provider :virtualbox do |v|
    config.vm.box_url = "https://cloud.centos.org/centos/7/vagrant/x86_64/images/CentOS-7-x86_64-Vagrant-2004_01.VirtualBox.box"
    v.memory = 2048
    v.cpus = 2
  end
  config.vm.provider :libvirt do |v|
    config.vm.box_url = "https://cloud.centos.org/centos/7/vagrant/x86_64/images/CentOS-7-x86_64-Vagrant-2004_01.LibVirt.box"
    v.memory = 2048
    v.cpus = 2
  end

  config.vm.hostname = "rke2-server"

  # Disabled by default. To run:
  #   vagrant up --provision-with=upgrade-packages
  # To upgrade only specific packages:
  #   UPGRADE_PACKAGES=selinux vagrant up --provision-with=upgrade-packages
  #
  config.vm.provision "upgrade-packages", type: "shell", run: "never" do |sh|
    sh.upload_path = "/tmp/vagrant-upgrade-packages"
    sh.env = {
        'UPGRADE_PACKAGES': ENV['UPGRADE_PACKAGES'],
    }
    sh.inline = <<~SHELL
        #!/usr/bin/env bash
        set -eux -o pipefail
        yum -y upgrade ${UPGRADE_PACKAGES}
    SHELL
  end

  # To re-run, installing CNI from RPM:
  #   INSTALL_PACKAGES="containernetworking-plugins" vagrant up --provision-with=install-packages
  #
  config.vm.provision "install-packages", type: "shell", run: "once" do |sh|
    sh.upload_path = "/tmp/vagrant-install-packages"
    sh.env = {
        'INSTALL_PACKAGES': ENV['INSTALL_PACKAGES'],
    }
    sh.inline = <<~SHELL
        #!/usr/bin/env bash
        set -eux -o pipefail
        yum -y install \
            curl \
            gcc \
            git \
            iptables \
            libseccomp-devel \
            libselinux-devel \
            lsof \
            make \
            ${INSTALL_PACKAGES}
    SHELL
  end

  # To re-run this provisioner, installing a different version of go:
  #   GO_VERSION="1.15rc2" vagrant up --provision-with=install-golang
  #
  config.vm.provision "install-golang", type: "shell", run: "once" do |sh|
    sh.upload_path = "/tmp/vagrant-install-golang"
    sh.env = {
        'GO_VERSION': ENV['GO_VERSION'] || "1.14.7",
    }
    sh.inline = <<~SHELL
        #!/usr/bin/env bash
        set -eux -o pipefail
        curl -fsSL "https://dl.google.com/go/go${GO_VERSION}.linux-amd64.tar.gz" | tar Cxz /usr/lib
        ln -fnsv /usr/lib/go/bin/{go,gofmt} /usr/bin
    SHELL
  end

  config.vm.provision "install-cri-tools", type: "shell", run: "once" do |sh|
    sh.upload_path = "/tmp/vagrant-install-cri-tools"
    sh.env = {
        'CRI_TOOLS_VERSION': ENV['CRI_TOOLS_VERSION'] || 'master',
        'GOBIN': '/usr/bin',
        'GOPATH': '/go',
    }
    sh.inline = <<~SHELL
        #!/usr/bin/env bash
        source /etc/environment
        source /etc/profile.d/sh.local
        set -eux -o pipefail
        go get -u github.com/onsi/ginkgo/ginkgo
        go get -d github.com/kubernetes-sigs/cri-tools/...
        cd "$GOPATH"/src/github.com/kubernetes-sigs/cri-tools
        git checkout $CRI_TOOLS_VERSION
        make
        sudo make BINDIR=$GOBIN install
        cat << EOF | sudo tee /etc/crictl.yaml
runtime-endpoint: unix:///run/k3s/containerd/containerd.sock
EOF
        type crictl critest ginkgo
        critest --version
    SHELL
  end

  # SELinux is Enforcing by default.
  # To set SELinux as Disabled on a VM that has already been provisioned:
  #   SELINUX=Disabled vagrant up --provision-with=selinux
  # To set SELinux as Permissive on a VM that has already been provsioned
  #   SELINUX=Permissive vagrant up --provision-with=selinux
  config.vm.provision "selinux", type: "shell", run: "once" do |sh|
    sh.upload_path = "/tmp/vagrant-selinux"
    sh.env = {
        'SELINUX': ENV['SELINUX'] || "Enforcing"
    }
    sh.inline = <<~SHELL
        #!/usr/bin/env bash
        set -eux -o pipefail

        if ! type -p getenforce setenforce &>/dev/null; then
          echo SELinux is Disabled
          exit 0
        fi

        case "${SELINUX}" in
          Disabled)
            if mountpoint -q /sys/fs/selinux; then
              setenforce 0
              umount -v /sys/fs/selinux
            fi
            ;;
          Enforcing)
            mountpoint -q /sys/fs/selinux || mount -o rw,relatime -t selinuxfs selinuxfs /sys/fs/selinux
            setenforce 1
            ;;
          Permissive)
            mountpoint -q /sys/fs/selinux || mount -o rw,relatime -t selinuxfs selinuxfs /sys/fs/selinux
            setenforce 0
            ;;
          *)
            echo "SELinux mode not supported: ${SELINUX}" >&2
            exit 1
            ;;
        esac

        echo SELinux is $(getenforce)
    SHELL
  end

  config.vm.provision "install-rke2", type: "shell", run: "once" do |sh|
    sh.upload_path = "/tmp/vagrant-install-rke2"
    sh.inline = <<~SHELL
        #!/usr/bin/env bash
        set -eux -o pipefail
        cat << EOF >>/etc/yum.repos.d/rpm-rancher-io.repo
[rancher-rke2-common-testing]
name=Rancher RKE2 Common Testing
baseurl=https://rpm-testing.rancher.io/rke2/testing/common/centos/7/noarch
enabled=1
gpgcheck=1
gpgkey=https://rpm-testing.rancher.io/public.key
[rancher-rke2-1-18-testing]
name=Rancher RKE2 1.18 Testing
baseurl=https://rpm-testing.rancher.io/rke2/testing/1.18/centos/7/x86_64
enabled=1
gpgcheck=1
gpgkey=https://rpm-testing.rancher.io/public.key
EOF
        yum -y install rke2-server
        echo >> /etc/sysconfig/rke2-server
        echo 'RKE2_KUBECONFIG_MODE=0644' >> /etc/sysconfig/rke2-server
        systemctl enable --now rke2-server
        cat << 'EOF' > /etc/profile.d/rke2.sh
export KUBECONFIG=/etc/rancher/rke2/rke2.yaml PATH=$PATH:/var/lib/rancher/rke2/bin
EOF
    SHELL
  end

end
