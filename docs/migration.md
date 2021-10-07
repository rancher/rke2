# Migration from RKE1 to RKE2

In order to migrate from Rancher Kubernetes Engine (RKE1) to RKE2, you need two things:

- ETCD data directory
- Cluster CA certificates

Both can be found when you take a RKE1 snapshot, as the RKE1 snapshot archive contains two things: an etcd snapshot, and `.rkestate` of your cluster.

## Introducing Migration Agent

migration-agent is a tool that lays the groundwork for RKE1 nodes to move to RKE2 nodes. It accomplishes this in two main steps:

- Restore the etcd snapshot on etcd nodes to the RKE2 etcd db data directory.
- Copy the CA certs and service account token key from `.rkestate` file to RKE2 data directory.

This tool runs on RKE1 nodes before running RKE2.

### Usage

1. To run the migration you need to take a snapshot first on rke1 nodes:

    ```
    rke etcd snapshot-save --s3 --name rke1snapshot --access-key <access-key> --secret-key <secret-key> --region <region> --folder <folder> --bucket-name <bucket name>
    ```
	
For more information, please refer to the RKE1 [official documentation](https://rancher.com/docs/rke/latest/en/etcd-snapshots/one-time-snapshots/)

2. Now you can either run migration agent directly on the node, or use the following [manifest](https://github.com/rancher/migration-agent/blob/master/deploy/daemonset.yaml) to deploy migration-agent as a daemonset on the RKE1 cluster. Before you apply the manifest file, you need to edit the file to include the information about the s3 snapshot of RKE1:
    ```
    command:
    - "sh"
    - "-c"
    - "migration-agent --s3-region <region> --s3-bucket <s3 bucket> --s3-folder <s3 folder> --s3-access-key <access-key> --s3-secret-key <secret-key>  --snapshot rke1db.zip && sleep 9223372036854775807"
    ```

3. After running the tool you should see the following paths have been created on control-plane and etcd nodes:
	
    ```
    /etc/rancher/rke2/config.yaml.d/10-migration.yaml
    /var/lib/rancher/rke2/server/
    /var/lib/rancher/rke2/server/db/
    /var/lib/rancher/rke2/server/manifests/
    /var/lib/rancher/rke2/server/tls/
    /var/lib/rancher/rke2/server/cred
    ```

4. The next step is stop docker containers of rke1:
	
    ```
    systemctl disable docker
    systemctl stop docker
    ```

5. The last step is to install and run rke2 server or agent depend on the type of the node:
	
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

RKE2 deploys addons as helm charts, so migration-agent creates a manifest that deletes the old RKE1 addons and let RKE2 deploys addons as Helm charts.

### Cluster Addons Configuration Migration

One of the migration-agent features is migrating all configuration for the cluster addons to rke2, this includes:

- CoreDNS configuration
- Metrics-Server configuration
- Ingress nginx configuration 

#### CoreDNS Configuration

RKE1 adds several configuration options for CoreDNS, migration-agent makes sure that these configs are migrated to a HelmchartConfig which will be used to configure the CoreDNS helmChart:

| CoreDNS Optinos                                      	|
|--------------------------------------------	|
| PriorityClassName                          	|
| NodeSelector                               	|
| RollingUpdate                              	|
| Tolerations                                	|
| AutoScalerConfig.Enabled                   	|
| AutoScalerConfig.PriorityClassName         	|
| AutoScalerConfig.Min                       	|
| AutoScalerConfig.Max                       	|
| AutoScalerConfig.CoresPerReplica           	|
| utoScalerConfig.NodesPerReplica            	|
| AutoScalerConfig.PreventSinglePointFailure 	|

#### Metrics Server Configuration

migration-agent also does the same for Metrics Server

| Metrics Server Options	|
|:---------------------:	|
|   PriorityClassName   	|
|      NodeSelector     	|
|          RBAC         	|
|      Tolerations      	|


#### Ingress Nginx Configuration

| Nginx Ingress Config 	|
|:--------------------:	|
|       ConfigMap      	|
|     NodeSelector     	|
|       ExtraArgs      	|
|       ExtraEnvs      	|
|     ExtraVolumes     	|
|   ExtraVolumeMounts  	|
|      Tolerations     	|
|       DNSPolicy      	|
|       HTTPPort       	|
|       HTTPSPort      	|
|   PriorityClassName  	|
| DefaultBackendConfig 	|


### Cloud Provider Support

migration-agent is able to migrate cloud provider configuration, this happens by copying the rke1 config file to the rke2 configuration directory and then passes down flags to RKE2 to include the name and path of cloud provider config:
    ```
    --cloud-provider-config
    --cloud-provider-name
    ```

### Private Registry Support

The agent also adds the ability to migrate private registry configuration, this happens by copying the private registries configured in the cluster.yaml file in rke1. Unfortunately RKE1 lacks the feature of passing TLS configuration to private registries and depends on Docker TLS configuration manually on each node, so to account for that migration-agent supports a flag --registry which Configure private registry TLS paths, syntax should be `<registry url>,<ca cert path>,<cert path>,<key path>`.


### CNI Configuration Migration


RKE1 and RKE2 both support Calico and Canal CNI, so migration-agent will be able to migrate the CNI only if Canal or Calico is used.