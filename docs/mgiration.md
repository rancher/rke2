# Migration from RKE1 to RKE2

In order to migrate from RKE to RKE 2, you need basically two things in particular:

- ETCD data directory
- Cluster CA certificates

Both can be found when you take a RKE1 snapshot, when taking a snapshot in RKE1 an archive file will be created that contain two things, the etcd snapshot and .rkestate of your cluster.

## Introducing Migration Agent

Migration Agent is a tool to lay the groundwork for RKE1 nodes to move to RKE2 nodes, it basically does two main steps among other things:

- Restore the etcd snapshot on etcd nodes to the RKE2 etcd db data directory.
- Copy the CA certs and service account token key from rkestate file to RKE2 data directory.

This tool runs on RKE1 nodes before running RKE2.

### Usage

1- To run the migration you need to take a snapshot first on rke1 nodes:

```
rke etcd snapshot-save --s3 --name rke1snapshot --access-key <access-key> --secret-key <secret-key> --region <region> --folder <folder> --bucket-name <bucket name>
```

For more information, please refer to the RKE1 [official documentation](https://rancher.com/docs/rke/latest/en/etcd-snapshots/one-time-snapshots/)

2- Now you can either run mgiration agent directly on the node, or use the following [mainfest](https://github.com/cwayne18/migration-agent/blob/master/deploy/daemonset.yaml) to deploy migration agent as a daemonset on the RKE1 cluster, before you run the manifest file you need to edit the file to include the information about the s3 snapshot of RKE1:
```
command:
          - "sh"
          - "-c"
          - "migration-agent --s3-region <region> --s3-bucket hgalal-s3 --s3-folder <s3 folder> --s3-access-key <access-key> --s3-secret-key <secret-key>  --snapshot rke1db.zip && sleep 9223372036854775807"
```

3- After running the tool you should see the following paths has been created on controlplane and etcd nodes:

```
/etc/rancher/rke2/config.yaml.d/10-migration.yaml
/var/lib/rancher/rke2/server/
/var/lib/rancher/rke2/server/db/
/var/lib/rancher/rke2/server/manifests/
/var/lib/rancher/rke2/server/tls/
/var/lib/rancher/rke2/server/cred
```

4- The next step is stop docker containers of rke1:

```
systemctl disable docker
systemctl stop docker
```

5- The last step is to install and run rke2 server or agent depend on the type of the node:

```
curl -sfL https://get.rke2.io | sh -
systemctl start rke2-server
systemctl enable rke2-server
```

### Cluster Configuration

One of the functions of the migration-agent is to copy the cluster configuration from the rkestate file to `/etc/rancher/rke2/config.yaml.d/10-migration.yaml`, this includes:

- Cluster CIDR
- Service CIDR
- Service Node Port Range
- Cluster DNS
- Cluster Domain
- Extra API Args
- Extra Scheduler Args
- Extra Controller Manager Args

### Addons Migration

RKE2 deploys addons as helm charts, so migration agent creates a manifest that deletes the old RKE1 addons and let RKE2 deploys addons as Helm charts.