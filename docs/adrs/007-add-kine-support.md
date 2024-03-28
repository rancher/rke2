# Add kine support to RKE2

## Established

## Revisit by

## Status

Accepted

## Context

This ADR is a introduction of kine support for RKE2. However, for this support to be implemented, it was necessary to add kine with TLS in K3s.
Which was done in this [PR](https://github.com/k3s-io/k3s/pull/9572), It was needed since rke2 cannot connect to kine without tls via the api server.

When rke2 is started with the `--datastore-endpoint` flag, it will disable the etcd pod and set the `cluster-init` flag to be `false`, to avoid the etcd part of k3s to be started.
Kine will use the etcd client certificate to authenticate the connection to the kine server that will be a `unixs` socket type.

### Pros

- With the integration of kine, it is now possible to use the `--datastore-endpoint` flag among others related to kine. This allows for a more versatile configuration of the datastore,
providing users with the flexibility to choose their preferred storage backend.

### Cons

- Kine can only be utilized with TLS due to the requirements of the API server.

## Other changes needed in k3s to better support kine in rke2

When testing rke2 with kine, there was some changes to avoid panics (specially when we are talking about `etcd`) and to make it work with tls. The changes are that when the user
uses `--database-endpoint` and other flags related to `etcd only` nodes, we have to ignore this flags or simply end the process with a error message.

We decided to set a error message and end the process, since it is not clear to the user that the flags are being ignored.

### Pros of Ignoring the flags

- It is possible to avoid panics and rke2 will run as expected.

### Cons of Ignoring the flags

- It will be not very clear to the user that the flags are being ignored.

### Pros of Ending the process with a error message

- Rke2 will run as expected with transparency to the user.

### Cons of Ending the process with a error message

- The user will have to change the flags to make rke2 run.
