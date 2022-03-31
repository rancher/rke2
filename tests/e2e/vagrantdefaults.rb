def defaultOSConfigure(vm)

  if vm.box.include?("ubuntu2004")
    vm.provision "shell", inline: "sed -i 's/4.2.2.1 4.2.2.2/8.8.8.8/g' /etc/systemd/resolved.conf", run: 'once'
    vm.provision "shell", inline: "service systemd-resolved restart", run: 'once'
    vm.provision "shell", inline: "apt install -y jq", run: 'once'
  end
  if vm.box.include?("Leap")
    vm.provision "shell", inline: "zypper install -y jq", run: 'once'
  end
  
end
