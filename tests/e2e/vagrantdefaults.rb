def defaultOSConfigure(vm)
  box = vm.box.to_s
  if box.include?("generic/ubuntu")
    vm.provision "netplan dns", type: "shell", inline: "netplan set ethernets.eth0.nameservers.addresses=[8.8.8.8,1.1.1.1]; netplan apply", run: 'once'
    vm.provision "Install jq", type: "shell", inline: "apt-get install -y jq", run: 'once'
  elsif box.include?("Leap") || box.include?("Tumbleweed")
    vm.provision "Install jq", type: "shell", inline: "zypper install -y jq", run: 'once'
  elsif box.match?(/windows.*2019/)
    vm.communicator = "winrm"
  elsif box.match?(/windows.*2022/)
    vm.communicator = "winrm"
  end
end

def installType(vm, version, branch)
  if version == "skip"
    return "INSTALL_RKE2_ARTIFACT_PATH=/tmp" 
  elsif !version.empty?
    return "INSTALL_RKE2_VERSION=#{version}"
  end
  # Grabs the last 5 commit SHA's from the given branch, then purges any commits that do not have a passing CI build
  scripts_location = Dir.exists?("./scripts") ? "./scripts" : "../scripts" 
  vm.provision "shell", path:  scripts_location + "/latest_commit.sh", args: [branch, "/tmp/rke2_commits"]
  return "INSTALL_RKE2_COMMIT=$(head\ -n\ 1\ /tmp/rke2_commits)"
end