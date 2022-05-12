---
title: Secrets Encryption
---

## Secrets Encryption Config

RKE2 supports encrypting Secrets at rest, and will do the following automatically:

- Generate an AES-CBC key
- Generate an encryption config file with the generated key:

```yaml
{
  "kind": "EncryptionConfiguration",
  "apiVersion": "apiserver.config.k8s.io/v1",
  "resources": [
    {
      "resources": [
        "secrets"
      ],
      "providers": [
        {
          "aescbc": {
            "keys": [
              {
                "name": "aescbckey",
                "secret": "xxxxxxxxxxxxxxxxxxx"
              }
            ]
          }
        },
        {
          "identity": {}
        }
      ]
    }
  ]
}
```

- Pass the config to the Kubernetes APIServer as encryption-provider-config

Once enabled any created secret will be encrypted with this key. Note that if you disable encryption then any encrypted secrets will not be readable until you enable encryption again using the same key.

## Secrets Encryption Tool
_Available as of v1.21.8+rke2r1_

RKE2 contains a utility [subcommand](https://docs.rke2.io/subcommands/#secrets-encrypt) `secrets-encrypt`, which allows administrators to perform the following tasks:

- Adding new encryption keys
- Rotating and deleting encryption keys
- Reencrypting secrets

>**Warning:** Failure to follow proper procedure when rotating secrets encryption keys can cause permanent data loss. Proceed with caution.

### Single-Server Encryption Key Rotation

To rotate secrets encryption keys on a single-node cluster:

1. Prepare:

    ```
    rke2 secrets-encrypt prepare
    ```

2. Restart the `kube-apiserver` pod:

    ```
    # Get the kube-apiserver container ID
    export CONTAINER_RUNTIME_ENDPOINT="unix:///var/run/k3s/containerd/containerd.sock"
    crictl ps --name kube-apiserver
    # Stop the pod
    crictl stop <CONTAINER_ID>
    ```

3. Rotate:

    ```
    rke2 secrets-encrypt rotate
    ```

4. Restart the `kube-apiserver` pod again
5. Reencrypt:

    ```
    rke2 secrets-encrypt reencrypt
    ```


### Multi-Server Encryption Key Rotation
To rotate secrets encryption keys on HA setups:

>**Note:** In this example, 3 servers are used to for a HA cluster, referred to as S1, S2, S3. While not required, it is recommended that you pick one server node from which to run the `secrets-encrypt` commands.

1. Prepare on S1

    ```
    rke2 secrets-encrypt prepare
    ```

2. Sequentially Restart S1, S2, S3
    ```
    systemctl restart rke2-server.service
    ```
    Wait for the systemctl command to return before restarting the next server.

3. Rotate on S1

    ```
    rke2 secrets-encrypt rotate
    ```

4. Sequentially Restart S1, S2, S3

5. Reencrypt on S1

    ```
    rke2 secrets-encrypt reencrypt
    ```
    Wait until reencryption is finished, either via server logs `journalctl -u rke2-server` or via `rke2 secrets-encrypt status`. The status will return `reencrypt_finished` when done.

6. Sequentially Restart S1, S2, S3

### Secrets Encryption Status
The `secrets-encrypt status` subcommand displays information about the current status of secrets encryption on the node.

An example of the command on a single-server node:  
```
$ rke2 secrets-encrypt status
Encryption Status: Enabled
Current Rotation Stage: start
Server Encryption Hashes: All hashes match

Active  Key Type  Name
------  --------  ----
 *      AES-CBC   aescbckey

```

Another example on HA cluster, after rotating the keys, but before restarting the servers:  
```
$ rke2 secrets-encrypt status
Encryption Status: Enabled
Current Rotation Stage: rotate
Server Encryption Hashes: hash does not match between node-1 and node-2

Active  Key Type  Name
------  --------  ----
 *      AES-CBC   aescbckey-2021-12-10T22:54:38Z
        AES-CBC   aescbckey

```

Details on each section are as follows:  

- __Encryption Status__: Displayed whether secrets encryption is disabled or enabled on the node  
- __Current Rotation Stage__: Indicates the current rotation stage on the node.  
  Stages are: `start`, `prepare`, `rotate`, `reencrypt_request`, `reencrypt_active`, `reencrypt_finished`  
- __Server Encryption Hashes__: Useful for HA clusters, this indicates whether all servers are on the same stage with their local files. This can be used to identify whether a restart of servers is required before proceeding to the next stage. In the HA example above, node-1 and node-2 have different hashes, indicating that they currently do not have the same encryption configuration. Restarting the servers will sync up their configuration.
- __Key Table__: Summarizes information about the secrets encryption keys found on the node.  
  * __Active__: The "*" indicates which, if any, of the keys are currently used for secrets encryption. An active key is used by Kubernetes to encrypt any new secrets.
  * __Key Type__: RKE2 only supports the `AES-CBC` key type. Find more info [here.](https://kubernetes.io/docs/tasks/administer-cluster/encrypt-data/#providers)
  * __Name__: Name of the encryption key.  