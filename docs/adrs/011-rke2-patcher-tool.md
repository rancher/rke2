# Prime tool: rke2-patcher

## Established

2026-03-02

## Revisit by

N/A

## Subject

As of today, when an ugly CVE gets discovered, the user must normally wait for the next RKE2 monthly release to get the CVE fixed. That typically means waiting between 1 and 4 weeks. That is quite long if the CVE opens up important security holes and as a result, the user must take temporary precautions and apply limitations in the cluster, which can be annoying and costly.

We could improve this process substantially by allowing users to consume the image that fixes the CVEs as soon as it is ready and verified. The flow would be:
1 - Image is shipped with the CVE fix
2 - QA verifies the new image works correctly with a set of automated tests
3 - Image tag is published in a specific place that guarantees the RKE2 users of that version can consume that image
4 - A tool is capable of collecting such information and patch the image in a running RKE2 cluster to consume the new image

#### rke2-patcher tool

The rke2-patcher tool is a binary deployed in one control-plane node of an RKE2 cluster and must have access to the kube-api of the cluster and to the directory where `HelmChartConfig` files are deployed (by default: /var/lib/rancher/rke2/server/manifests).

The tool "rke2-patcher" can execute 3 different actions:

* rke2-patcher image-cve <component>

It requires `trivy` to be installed in the node. It queries the image being run by <component> and runs a scan. It outputs the CVEs.

Example:
```bash
$> ./rke2-patcher image-cve calico-operator
component: calico-operator
image: docker.io/rancher/mirrored-calico-operator:v1.40.3
scanner: trivy
CVEs (5):
- CVE-2025-61726
- CVE-2025-61728
- CVE-2025-61730
- CVE-2025-68121
- CVE-2026-22771
```

* rke2-patcher image-list <component>

It queries the registry to list what are the current images available for such component for the current RKE2 version. It also specifies which one is being used in the cluster. 

Example:
```bash
$> ./rke2-patcher image-list calico-operator
component: calico-operator
repository: rancher/mirrored-calico-operator
running image(s):
- docker.io/rancher/mirrored-calico-operator:v1.40.3 (pods: 1)
available tags (20):
- v1.40.7 (updated 2026-02-23T10:17:17Z)
- v1.40.3 (updated 2026-01-05T13:30:33Z) <-- in use
```

* rke2-patcher image-patch <component>

It generates a `HelmChartConfig` patching the image of the <component>, so that it points to the latest in the list. It moves hat `HelmChartConfig` to the `server/manifests` directory. It includes a `--dry-run` flag to understand what is going to be written
```bash
$> ./rke2-patcher image-patch calico-operator --dry-run
component: calico-operator
current image: docker.io/rancher/mirrored-calico-operator:v1.40.3
current tag: v1.40.3
new tag: v1.40.7
dry-run: true
would write HelmChartConfig: /var/lib/rancher/rke2/server/manifests/calico-operator-config-rke2-patcher.yaml
---
apiVersion: helm.cattle.io/v1
kind: HelmChartConfig
metadata:
  name: rke2-calico
  namespace: kube-system
spec:
  valuesContent: |-
    tigeraOperator:
      image: rancher/mirrored-calico-operator
      version: v1.40.7
      registry: docker.io
```

This tool can be used for the common RKE2 components:
* calico-operator
* canal
* cilium-operator
* cluster-autoscaler
* coredns
* csi-snapshotter
* dns-node-cache
* flannel
* ingress-nginx
* metrics-server
* snapshot-controller
* traefik


#### Problems to be solved

* If a user is already patching a component with a HelmChartConfig. What to do? Merge both together into one and add one extra warning step?
* Trivy must be installed separately. Maybe we could run trivy inside a K8s Job? That would allow zero-dependency with the OS
* Could we build a limited version for Airgap users?


#### Planned Limitations

In the first release of this tool we will introduce some limitations:

1 - Only two tags will be available per component and per release: the image tag with the CVE and the image tag without the CVE. Allowing more than two will increase QA's testing complexity exponentially

2 - A few components could be patched per release but not all. Again, if we allow all, the amount of possible combinations will be too much

3 - We are only covering some of the RKE2 components and not all, especially not the ones directly connected to the Kubernetes release to reduce complexity

## Current unknowns

1 - We will need to define the set of automated tests that QA should run using the new image. There is a balance we need to decide between "velocity" and "thoroughness".

2 - Once QA give the green light, we need to define how we will map rke2 versions to the available 2 tags for a specific component. A simple way could be by using the release notes of each RKE2: https://documentation.suse.com/cloudnative/rke2/latest/en/release-notes/v1.35.X.html. A metadata server that includes this information would be best but probably more complicated.

3 - If it is successful, it will probably make sense to integrate it into rke2 commands. But how to define success?

4 - If we keep it only for Prime users, how to limit it accessibility?

## Status

