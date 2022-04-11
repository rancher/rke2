def defaultOSConfigure(vm)

  if vm.box.include?("ubuntu2004")
    vm.provision "netplan dns", type: "shell", inline: "netplan set ethernets.eth0.nameservers.addresses=[8.8.8.8,1.1.1.1]; netplan apply", run: 'once'
    vm.provision "Install jq", type: "shell", inline: "apt-get install -y jq", run: 'once'
  end
  if vm.box.include?("Leap")
    vm.provision "Install jq", type: "shell", inline: "zypper install -y jq", run: 'once'
  end
  if vm.box.match?(/windows.*2019/)
    vm.communicator = "winrm"
  end
  if vm.box.match?(/windows.*2022/)
    vm.communicator = "winrm"
  end
end
