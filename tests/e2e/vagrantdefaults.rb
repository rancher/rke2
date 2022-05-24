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