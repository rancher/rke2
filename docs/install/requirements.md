RKE2 is very lightweight, but has some minimum requirements as outlined below.

## Prerequisites

Two nodes cannot have the same hostname.

If all your nodes have the same hostname, set the `node-name` parameter in the RKE2 config file for each node you add to the cluster to have a different node name.

## Operating Systems

### Linux
RKE2 has been tested and validated on the following operating systems, and their subsequent non-major releases:

*    Ubuntu 18.04 (amd64)
*    Ubuntu 20.04 (amd64)
*    CentOS/RHEL 7.8 (amd64)
*    CentOS/RHEL 8.2 (amd64)
*    SLES 15 SP2 (amd64) (v1.18.16+rke2r1 and newer)

### Windows
**Windows Support is currently Experimental as of v1.21.3+rke2r1**
**Windows Support requires choosing Calico as the CNI for the RKE2 cluster**

The RKE2 Windows Node (Worker) agent has been tested and validated on the following operating systems, and their subsequent non-major releases:

* Windows Server 2019 LTSC (amd64) (OS Build 17763.2061)
* Windows Server SAC 2004 (amd64) (OS Build 19041.1110)
* Windows Server SAC 20H2 (amd64) (OS Build 19042.1110)

**Note** The Windows Server Containers feature needs to be enabled for the RKE2 Windows agent to work.

Open a new Powershell window with Administrator privileges
```powershell
powershell -Command "Start-Process PowerShell -Verb RunAs"
```

In the new Powershell window, run the following command.
```powershell
Enable-WindowsOptionalFeature -Online -FeatureName containers –All
```

This will require a reboot for the `Containers` feature to properly function.

## Hardware

Hardware requirements scale based on the size of your deployments. Minimum recommendations are outlined here.

### Linux/Windows
*    RAM: 4GB Minimum (we recommend at least 8GB)
*    CPU: 2 Minimum (we recommend at least 4CPU)

#### Disks

RKE2 performance depends on the performance of the database, and since RKE2 runs etcd embeddedly and it stores the data dir on disk, we recommend using an SSD when possible to ensure optimal performance.

## Networking

**Important:** If your node has NetworkManager installed and enabled, [ensure that it is configured to ignore CNI-managed interfaces.](https://docs.rke2.io/known_issues/#networkmanager)

The RKE2 server needs port 6443 and 9345 to be accessible by other nodes in the cluster.

All nodes need to be able to reach other nodes over UDP port 8472 when Flannel VXLAN is used.

If you wish to utilize the metrics server, you will need to open port 10250 on each node.

**Important:** The VXLAN port on nodes should not be exposed to the world as it opens up your cluster network to be accessed by anyone. Run your nodes behind a firewall/security group that disables access to port 8472.

<figcaption>Inbound Rules for RKE2 Server Nodes</figcaption>

| Protocol | Port | Source | Description
|-----|-----|----------------|---|
| TCP | 9345 | RKE2 agent nodes | Kubernetes API
| TCP | 6443 | RKE2 agent nodes | Kubernetes API
| UDP | 8472 | RKE2 server and agent nodes | Required only for Flannel VXLAN
| TCP | 10250 | RKE2 server and agent nodes | kubelet
| TCP | 2379 | RKE2 server nodes | etcd client port
| TCP | 2380 | RKE2 server nodes | etcd peer port
| TCP | 30000-32767 | RKE2 server and agent nodes | NodePort port range
| UDP | 8472 | RKE2 server and agent nodes | Cilium CNI VXLAN
| TCP | 4240 | RKE2 server and agent nodes | Cilium CNI health checks
| ICMP | 8/0 | RKE2 server and agent nodes | Cilium CNI health checks
| TCP | 179 | RKE2 server and agent nodes | Calico CNI with BGP
| UDP | 4789 | RKE2 server and agent nodes | Calico CNI with VXLAN
| TCP | 5473 | RKE2 server and agent nodes | Calico CNI with Typha
| UDP | 8472 | RKE2 server and agent nodes | Canal CNI with VXLAN
| TCP | 9099 | RKE2 server and agent nodes | Canal health checks
| UDP | 51820 | RKE2 server and agent nodes | Canal CNI with WireGuard IPv4
| UDP | 51821 | RKE2 server and agent nodes | Canal CNI with WireGuard IPv6/dual-stack

<figcaption>Inbound Rules for RKE2 Windows Agent Nodes</figcaption>

### Windows Specific Inbound Network Rules
| Protocol | Port | Source | Description
|-----|-----|----------------|---|
| UDP | 4789 | RKE2 server nodes | Required for Calico and Flannel VXLAN

Typically, all outbound traffic will be allowed.
