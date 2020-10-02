In this section, you'll learn how to create backups of the rke2 cluster data and to restore the cluster from backup.

**Note:** /var/lib/rancher/rke2 is the default data directory for rke2, its however configurable via --data-dir

### Creating Snapshots

Snapshots are enabled by default.

The snapshot directory defaults to `/var/lib/rancher/rke2/server/db/snapshots`.

To configure the snapshot interval or the number of retained snapshots, refer to the [options.](#options)


## Cluster Reset

RKE2 enables a feature to reset the cluster to one member cluster by passing `--cluster-reset` flag, when passing this flag to rke2 server it will reset the cluster with the same data dir in place, the data directory for etcd exists in `/var/lib/rancher/rke2/server/db/etcd`, this flag can be passed in the events of quorum loss in the cluster.

To pass the reset flag, first you need to stop RKE2 service if its enabled via systemd:

```
systemctl stop rke2-server
rke2 server --cluster-reset
```

**Result:**  A message in the logs say that RKE2 can be restarted without the flags. Start rke2 again and it should start rke2 as a 1 member cluster.
 
### Restoring a Cluster from a Snapshot

When RKE2 is restored from backup, the old data directory will be moved to `/var/lib/rancher/rke2/server/db/etcd-old-%date%/`. Then RKE2 will attempt to restore the snapshot by creating a new data directory, then starting etcd with a new RKE2 cluster with one etcd member.

To restore the cluster from backup, run RKE2 with the `--cluster-reset` option, with the `--cluster-reset-restore-path` also given:

```
rke2 server \
  --cluster-reset \
  --cluster-reset-restore-path=<PATH-TO-SNAPSHOT>
```

**Result:**  A message in the logs says that RKE2 can be restarted without the flags. Start RKE2 again and should run successfully and be restored from the specified snapshot.

When rke2 resets the cluster, it creates a file at `/var/lib/rancher/rke2/server/db/etc/reset-file`. If you want to reset the cluster again, you will need to delete this file.

### Options

These options can be passed in with the command line, or in the configuration file. Which may be easier to use.

| Options | Description |
| ----------- | --------------- |
| `--etcd-disable-snapshots` | Disable automatic etcd snapshots |
| `--etcd-snapshot-schedule-cron` value  |  Snapshot interval time in cron spec. eg. every 5 hours `* */5 * * *`(default: `0 */12 * * *`) |
| `--etcd-snapshot-retention` value  | Number of snapshots to retain (default: 5) |
| `--etcd-snapshot-dir` value  | Directory to save db snapshots. (Default location: `${data-dir}/db/snapshots`) |
| `--cluster-reset`  | Forget all peers and become sole member of a new cluster. This can also be set with the environment variable `[$K3S_CLUSTER_RESET]`.
| `--cluster-reset-restore-path` value | Path to snapshot file to be restored
