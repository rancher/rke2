# Cluster Access

The kubeconfig file stored at `/etc/rancher/rke2/rke2.yaml` is used to configure access to the Kubernetes cluster. 
If you have installed upstream Kubernetes command line tools such as kubectl or helm you will need to configure them with the correct kubeconfig path. 
This can be done by either exporting the `KUBECONFIG` environment variable or by invoking the `--kubeconfig` command line flag. 
Refer to the examples below for details.

Note that some tools, such as kubectl, are installed by default into `/var/lib/rancher/rke2/bin`.

Leverage the KUBECONFIG environment variable:

```
export KUBECONFIG=/etc/rancher/rke2/rke2.yaml
kubectl get pods --all-namespaces
helm ls --all-namespaces
```

Or specify the location of the kubeconfig file in the command:

```
kubectl --kubeconfig /etc/rancher/rke2/rke2.yaml get pods --all-namespaces
helm --kubeconfig /etc/rancher/rke2/rke2.yaml ls --all-namespaces
```

### Accessing the Cluster from Outside with kubectl

Copy `/etc/rancher/rke2/rke2.yaml` on your machine located outside the cluster as `~/.kube/config`. Then replace `127.0.0.1` with the IP or hostname of your RKE2 server. `kubectl` can now manage your RKE2 cluster.
