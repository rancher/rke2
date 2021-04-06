RKE2 is very lightweight, but has some minimum requirements as outlined below.

## Prerequisites

Two nodes cannot have the same hostname.

If all your nodes have the same hostname, set the `node-name` parameter in the RKE2 config file for each node you add to the cluster to have a different node name.

## Operating Systems

RKE2 has been tested and validated on the following operating systems and their subsequent non-major releases:

*    Ubuntu 18.04 (amd64)
*    Ubuntu 20.04 (amd64)
*    CentOS/RHEL 7.8 (amd64)
*    CentOS/RHEL 8.2 (amd64)
*    SLES 15 SP2 (amd64) (v1.18.16+rke2r1 and newer)

## Hardware

Hardware requirements scale based on the size of your deployments. Minimum recommendations are outlined here.

*    RAM: 512MB Minimum (we recommend at least 1GB)
*    CPU: 1 Minimum

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

Typically all outbound traffic is allowed.
