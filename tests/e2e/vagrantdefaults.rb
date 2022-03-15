def defaultOSConfigure(vm)

  if vm.box.include?("ubuntu2004")
    vm.provision "shell", inline: "systemd-resolve --set-dns=8.8.8.8 --interface=eth0", run: 'once'
    vm.provision "shell", inline: "apt install -y jq", run: 'once'
  end
  if vm.box.include?("Leap")
    vm.provision "shell", inline: "zypper install -y jq", run: 'once'
  end
  
end
