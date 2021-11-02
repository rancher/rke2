# Etcd Backup and Restore

In this section, you'll learn how to create backups of the rke2 cluster data and to restore the cluster from backup.

**Note:** /var/lib/rancher/rke2 is the default data directory for rke2, it is configurable however via `data-dir` parameter.

### Creating Snapshots

Snapshots are enabled by default.

The snapshot directory defaults to `/var/lib/rancher/rke2/server/db/snapshots`.

To configure the snapshot interval or the number of retained snapshots, refer to the [options.](#options)

In RKE2, snapshots are stored on each etcd node. If you have multiple etcd or etcd + control-plane nodes, you will have multiple copies of local etcd snapshots.

## Cluster Reset

RKE2 enables a feature to reset the cluster to one member cluster by passing `--cluster-reset` flag, when passing this flag to rke2 server it will reset the cluster with the same data dir in place, the data directory for etcd exists in `/var/lib/rancher/rke2/server/db/etcd`, this flag can be passed in the events of quorum loss in the cluster.

To pass the reset flag, first you need to stop RKE2 service if its enabled via systemd:

```
systemctl stop rke2-server
rke2 server --cluster-reset
```

**Result:**  A message in the logs say that RKE2 can be restarted without the flags. Start rke2 again and it should start rke2 as a 1 member cluster.

### Restoring a Snapshot to Existing Nodes

When RKE2 is restored from backup, the old data directory will be moved to `/var/lib/rancher/rke2/server/db/etcd-old-%date%/`. RKE2 will then attempt to restore the snapshot by creating a new data directory and start etcd with a new RKE2 cluster with one etcd member.

1. You must stop RKE2 service on all server nodes if it is enabled via systemd. Use the following command to do so:
```
systemctl stop rke2-server
```

2. Next, you will initiate the restore from snapshot on the first server node with the following commands:
```
rke2 server \
  --cluster-reset \
  --cluster-reset-restore-path=<PATH-TO-SNAPSHOT>
```

3. Once the restore process is complete, start the rke2-server service on the first server node as follows:
```
systemctl start rke2-server
```

4. Remove the rke2 db directory on the other server nodes as follows:
```
rm -rf /var/lib/rancher/rke2/server/db
```

5. Start the rke2-server service on other server nodes with the following command:
```
systemctl start rke2-server
```

**Result:**  After a successful restore, a message in the logs says that etcd is running, and RKE2 can be restarted without the flags. Start RKE2 again, and it should run successfully and be restored from the specified snapshot.

When rke2 resets the cluster, it creates an empty file at `/var/lib/rancher/rke2/server/db/reset-flag`. This file is harmless to leave in place, but must be removed in order to perform subsequent resets or restores. This file is deleted when rke2 starts normally.


### Restoring a Snapshot to New Nodes

**Warning:** For all versions of rke2 v.1.20.9 and prior, you will need to back up and restore certificates first due to a known issue in which bootstrap data might not save on restore (Steps 1 - 3 below assume this scenario). See [note](#other-notes-on-restoring-a-snapshot) below for an additional version-specific restore caveat on restore.

1. Back up the following: `/var/lib/rancher/rke2/server/cred`, `/var/lib/rancher/rke2/server/tls`, `/var/lib/rancher/rke2/server/token`, `/etc/rancher`

2. Restore the certs in Step 1 above to the first new server node.

3. Install rke2 v1.20.8+rke2r1 on the first new server node as in the following example:
```
curl -sfL https://get.rke2.io | INSTALL_RKE2_VERSION="v1.20.8+rke2r1" sh -`
```

4. Stop RKE2 service on all server nodes if it is enabled and initiate the restore from snapshot on the first server node with the following commands:
```
systemctl stop rke2-server
rke2 server \
  --cluster-reset \
  --cluster-reset-restore-path=<PATH-TO-SNAPSHOT>
```

5. Once the restore process is complete, start the rke2-server service on the first server node as follows:
```
systemctl start rke2-server
```

6. You can continue to add new server and worker nodes to cluster per standard [RKE2 HA installation documentation](https://docs.rke2.io/install/ha/#3-launch-additional-server-nodes).


### Other Notes on Restoring a Snapshot

> * When performing a restore from backup, users do not need to restore a snapshot using the same version of RKE2 with which the snapshot was created. Users may restore using a more recent version. Be aware when changing versions at restore which etcd version is in use.

> * By default, snapshots are enabled and are scheduled to be taken every 12 hours. The snapshots are written to `${data-dir}/server/db/snapshots` with the default `${data-dir}` being `/var/lib/rancher/rke2`.

> **Version-specific requirement for rke2 v1.20.11+rke2r1**

> * When restoring RKE2 from backup to a new node in rke2 v1.20.11+rke2r1, you should ensure that all pods are stopped following the initial restore by running `rke2-killall.sh` as follows:
```
curl -sfL https://get.rke2.io | sudo INSTALL_RKE2_VERSION=v1.20.11+rke2r1
rke2 server \
 --cluster-reset \
 --cluster-reset-restore-path=<PATH-TO-SNAPSHOT> \
 --token=<token used in the original cluster>
rke2-killall.sh
```
> Once the restore process is complete, enable and start the rke2-server service on the first server node as follows:    
```
systemctl enable rke2-server
systemctl start rke2-server
```

### Options

These options can be set in the configuration file:

| Options | Description |
| ----------- | --------------- |
| `etcd-disable-snapshots` | Disable automatic etcd snapshots |
| `etcd-snapshot-schedule-cron` value  |  Snapshot interval time in cron spec. eg. every 4 hours `0 */4 * * *`(default: `0 */12 * * *`) |
| `etcd-snapshot-retention` value  | Number of snapshots to retain (default: 5) |
| `etcd-snapshot-dir` value  | Directory to save db snapshots. (Default location: `${data-dir}/db/snapshots`) |
| `cluster-reset`  | Forget all peers and become sole member of a new cluster. This can also be set with the environment variable `[$RKE2_CLUSTER_RESET]`.
| `cluster-reset-restore-path` value | Path to snapshot file to be restored

### S3 Compatible API Support

rke2 supports writing etcd snapshots to and restoring etcd snapshots from systems with S3-compatible APIs. S3 support is available for both on-demand and scheduled snapshots.

The arguments below have been added to the `server` subcommand. These flags exist for the `etcd-snapshot` subcommand as well however the `--etcd-s3` portion is removed to avoid redundancy.

| Options | Description |
| ----------- | --------------- |
| `--etcd-s3` | Enable backup to S3 |
| `--etcd-s3-endpoint` | S3 endpoint url |
| `--etcd-s3-endpoint-ca` | S3 custom CA cert to connect to S3 endpoint |
| `--etcd-s3-skip-ssl-verify` | Disables S3 SSL certificate validation |
| `--etcd-s3-access-key` |  S3 access key |
| `--etcd-s3-secret-key` | S3 secret key" |
| `--etcd-s3-bucket` | S3 bucket name |
| `--etcd-s3-region` | S3 region / bucket location (optional). defaults to us-east-1 |
| `--etcd-s3-folder` | S3 folder |

To perform an on-demand etcd snapshot and save it to S3:

```
rke2 etcd-snapshot \
  --s3 \
  --s3-bucket=<S3-BUCKET-NAME> \
  --s3-access-key=<S3-ACCESS-KEY> \
  --s3-secret-key=<S3-SECRET-KEY>
```

To perform an on-demand etcd snapshot restore from S3, first make sure that rke2 isn't running. Then run the following commands:

```
rke2 server \
  --cluster-reset \
  --etcd-s3 \
  --cluster-reset-restore-path=<SNAPSHOT-NAME> \
  --etcd-s3-bucket=<S3-BUCKET-NAME> \
  --etcd-s3-access-key=<S3-ACCESS-KEY> \
  --etcd-s3-secret-key=<S3-SECRET-KEY>
```
