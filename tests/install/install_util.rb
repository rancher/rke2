def waitForControlPlane(vm, box)
  hostname = box.include?("opensuse") ? "$(hostnamectl --static)" : "$(hostname)"
  vm.provision "rke2-wait-for-cp", type: "shell", run: ENV['CI'] == 'true' ? 'never' : 'once' do |sh|
    sh.inline = <<~SHELL
    #!/usr/bin/env bash
    set -eu -o pipefail
    echo 'Waiting for node (and static pods) to be ready ...'
    time {
        timeout 240 bash -c 'while ! (kubectl wait --for condition=ready node/#{hostname} 2>/dev/null); do sleep 5; done'
        timeout 240 bash -c 'while ! (kubectl wait --for condition=ready -n kube-system pod/etcd-#{hostname} 2>/dev/null); do sleep 5; done'
        timeout 240 bash -c 'while ! (kubectl wait --for condition=ready -n kube-system pod/kube-apiserver-#{hostname} 2>/dev/null); do sleep 5; done'
        timeout 240 bash -c 'while ! (kubectl wait --for condition=ready -n kube-system pod/kube-scheduler-#{hostname} 2>/dev/null); do sleep 5; done'
        timeout 240 bash -c 'while ! (kubectl wait --for condition=ready -n kube-system pod/kube-proxy-#{hostname} 2>/dev/null); do sleep 5; done'
        timeout 240 bash -c 'while ! (kubectl wait --for condition=ready -n kube-system pod/kube-controller-manager-#{hostname} 2>/dev/null); do sleep 5; done'
        timeout 240 bash -c 'while ! (kubectl wait --for condition=ready -n kube-system pod/cloud-controller-manager-#{hostname} 2>/dev/null); do sleep 5; done'
    }
    kubectl get node,all -A -o wide
    SHELL
  end
end


def waitForCanal(vm)
  vm.provision "rke2-wait-for-canal", type: "shell", run: ENV['CI'] == 'true' ? 'never' : 'once' do |sh|
    sh.inline = <<~SHELL
    #!/usr/bin/env bash
    set -eu -o pipefail
    time {
        timeout 240 bash -c 'while ! (kubectl --namespace kube-system rollout status --timeout 10s daemonset/rke2-canal 2>/dev/null); do sleep 5; done'
    }
    SHELL
  end
end

def waitForCoreDNS(vm)
  vm.provision "rke2-wait-for-coredns", type: "shell", run: ENV['CI'] == 'true' ? 'never' : 'once' do |sh|
    sh.inline = <<~SHELL
    #!/usr/bin/env bash
    set -eu -o pipefail
    time {
        timeout 240 bash -c 'while ! (kubectl --namespace kube-system rollout status --timeout 10s deploy/rke2-coredns-rke2-coredns 2>/dev/null); do sleep 5; done'
        timeout 240 bash -c 'while ! (kubectl --namespace kube-system rollout status --timeout 10s deploy/rke2-coredns-rke2-coredns-autoscaler 2>/dev/null); do sleep 5; done'
    }
    SHELL
  end
end

def waitForIngressNginx(vm)
  vm.provision "rke2-wait-for-ingress-nginx", type: "shell", run: ENV['CI'] == 'true' ? 'never' : 'once' do |sh|
    sh.inline = <<~SHELL
    #!/usr/bin/env bash
    set -eu -o pipefail
    time {
        timeout 240 bash -c 'while ! (kubectl --namespace kube-system rollout status --timeout 10s daemonset/rke2-ingress-nginx-controller 2>/dev/null); do sleep 5; done'
    }
    SHELL
  end
end

def waitForMetricsServer(vm)
  vm.provision "rke2-wait-for-metrics-server", type: "shell", run: ENV['CI'] == 'true' ? 'never' : 'once' do |sh|
    sh.inline = <<~SHELL
    #!/usr/bin/env bash
    set -eu -o pipefail
    time {
        timeout 240 bash -c 'while ! (kubectl --namespace kube-system rollout status --timeout 10s deploy/rke2-metrics-server 2>/dev/null); do sleep 5; done'
    }
    SHELL
  end
end

def checkRKE2Processes(vm)
  vm.provision "rke2-procps", type: "shell", run: ENV['CI'] == 'true' ? 'never' : 'once' do |sh|
    sh.inline = <<~SHELL
    #!/usr/bin/env bash
    set -eux -o pipefail
    ps auxZ | grep -E 'etcd|kube|rke2|container|spc_t|unconfined_t' | grep -v grep
    SHELL
  end
end

def kubectlStatus(vm)
  vm.provision "rke2-status", type: "shell", run: ENV['CI'] == 'true' ? 'never' : 'once' do |sh|
    sh.inline = <<~SHELL
    #!/usr/bin/env bash
    set -eux -o pipefail
    kubectl get node,all -A -o wide
    SHELL
  end
end

def mountDirs(vm)
  vm.provision "rke2-mount-directory", type: "shell", run: ENV['CI'] == 'true' ? 'never' : 'once' do |sh|
    sh.inline = <<~SHELL
    #!/usr/bin/env bash
    set -eu -o pipefail
    echo 'Mounting server dir'
    mount --bind /var/lib/rancher/rke2/server /var/lib/rancher/rke2/server
    SHELL
  end
end

def checkMountPoint(vm)
  vm.provision "rke2-check-mount", type: "shell", run: ENV['CI'] == 'true' ? 'never' : 'once' do |sh|
    sh.inline = <<~SHELL
    #!/usr/bin/env bash
    set -eu -o pipefail
    echo 'Check the mount'
    mount | grep /var/lib/rancher/rke2/server
    SHELL
  end
end

def runKillAllScript(vm)
  vm.provision "rke2-killall", type: "shell", run: ENV['CI'] == 'true' ? 'never' : 'once' do |sh|
    sh.inline = <<~SHELL
    #!/usr/bin/env bash
    set -eu -o pipefail
    echo 'Run kill all'
    # This script runs as sudo, which may not have the PATH set correctly.
    PATH=/usr/local/bin:$PATH rke2-killall.sh
    SHELL
  end
end
