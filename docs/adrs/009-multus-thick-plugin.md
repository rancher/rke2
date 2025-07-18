# Multus thick plugin support in rke2

Proposal date: 2025-06-18

## Status

## Context

Multus is a k8s CNI multiplexer that allows the user to add several network interfaces to a pod.
In 2023, the upstream Multus project introduced a new way to deploy Multus called "thick" plugin as opposed to the method already supported in rke2 called "thin" plugin.

We have received [a request](https://github.com/harvester/harvester/issues/7042) from the Harvester team to support Multus thick plugin in rke2. 
See also SURE-8203.

In addition, the thick plugin deployment should become the default method from upstream in the future.

### Pros
- allows the use of [multus-dynamic-networks-controller](https://github.com/k8snetworkplumbingwg/multus-dynamic-networks-controller) by Harvester to implement [interface hotplugging](https://kubevirt.io/user-guide/network/hotplug_interfaces/) with Kubevirt
- Multus thick plugin provides better metrics

### Cons
- higher resource consumption than with thin plugin
- more complex Multus chart to support
    - new optional components are added to the chart
    - both plugins need to be validated
- more cases to test in QA

## Implementation steps
### In rke2
- no changes needed

### In rke2-charts
- build the new hardened images:
    - multus-daemon (already done)
    - multus-dynamics-network-controller
- add the multus-thick option + configuration for each supported CNI
- add the multus-dynamics-network-controller as an option

## Decision