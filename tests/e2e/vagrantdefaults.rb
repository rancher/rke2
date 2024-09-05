def defaultOSConfigure(vm)
  box = vm.box.to_s
  if box.include?("ubuntu")
    vm.provision "netplan dns", type: "shell", inline: "netplan set ethernets.eth0.nameservers.addresses=[8.8.8.8,1.1.1.1]; netplan apply", run: 'once'
    vm.provision "Install jq", type: "shell", inline: "apt-get install -y jq", run: 'once'
  elsif box.include?("Leap") || box.include?("Tumbleweed")
    vm.provision "Install jq", type: "shell", inline: "zypper install -y jq", run: 'once'
  elsif box.match?(/Windows.*2019/) || box.match?(/Windows.*2022/)
    vm.communicator = "winssh"
  end
end

def getInstallType(vm, version, branch)
  if version == "skip"
    return "INSTALL_RKE2_ARTIFACT_PATH=/tmp" 
  elsif !version.empty?
    return "INSTALL_RKE2_VERSION=#{version}"
  end
  # Grabs the last 5 commit SHA's from the given branch, then purges any commits that do not have a passing CI build
  scripts_location = Dir.exist?("./scripts") ? "./scripts" : "../scripts" 
  vm.provision "shell", path:  scripts_location + "/latest_commit.sh", args: [branch, "/tmp/rke2_commits"]
  return "INSTALL_RKE2_COMMIT=$(head\ -n\ 1\ /tmp/rke2_commits)"
end

def cisPrep(vm)
  vm.provision "shell", inline: "useradd -r -c 'etcd user' -s /sbin/nologin -M etcd -U"
  vm.provision "shell", inline: "printf 'vm.panic_on_oom=0\nvm.overcommit_memory=1\nkernel.panic=10\nkernel.panic_on_oops=1' > /etc/sysctl.d/60-rke2-cis.conf; systemctl restart systemd-sysctl"
  if vm.box.to_s.include?("ubuntu")
    vm.provision "Install kube-bench", type: "shell", inline: <<-SHELL
    export KBV=0.8.0
    curl -L "https://github.com/aquasecurity/kube-bench/releases/download/v${KBV}/kube-bench_${KBV}_linux_amd64.deb" -o "kube-bench_${KBV}_linux_amd64.deb"
    dpkg -i "./kube-bench_${KBV}_linux_amd64.deb"
    SHELL
  end
end

# vagrant cannot scp files as root, so we copy manifests to /tmp and then move them to the correct location
def loadManifests(vm, files)
  vm.provision "shell", inline: "mkdir -p /var/lib/rancher/rke2/server"
  vm.provision "shell", inline: "mkdir -p -m 777 /tmp/manifests"
  files.each do |file|
    vm.provision "file", source: file, destination: "/tmp/manifests/#{File.basename(file)}"
  end
  vm.provision "Deploy additional manifests", type: "shell", inline: "mv /tmp/manifests /var/lib/rancher/rke2/server/manifests"
end
