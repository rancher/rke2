# Flannel support in rke2

## Established
## Revisit by
## Status

Accepted

## Context

Currently RKE2-windows users can only deploy with one supported CNI plugin: Calico. In the last weeks, we have had several users complaining about a TCP Reset issue that breaks their applications. For example, applications that rely on a stable connectivity towards external nodes, like Jenkins Server, are impossible to operate if TCP Resets appears.

The TCP Reset issue is known by Tigera (the company behind Calico) and documented [here](https://docs.tigera.io/calico/latest/getting-started/kubernetes/windows-calico/limitations#pod-to-pod-connections-are-dropped-with-tcp-reset-packets)
As described in that doc, the issue should only appear when using network policies. Customers state that when using network policies the problem is constant but even without network policies, the TCP Resets appear from time to time with no clear trigger for it.

It is hard to understand where are those TCP Resets coming from because the code creating the virtual network infrastructure in Windows is closed sourced and the documentation is very poor and not really explaining the details, so we can only especulate. I think the problem is that windows creates a TCP Proxy in their VMSwitches when using VFP (Virtual Filtering Platform) and whenever a rule, affecting the pod, of VFP changes, the TCP Proxy is recreated and thus all TCP connections are reset. It is probably part of the design of VFP and thus complicated to really work around it. Calico uses VFP to do the network policies.

We have been communicating all this to Tigera and Microsoft but so far, the resolution to the problem does not seem to be close at hand. It feels like a complicated problem created by the VFP design.

Moreover, some of these users were previous users of RKE1-windows with flannel and they were happy with it. THey are asking to have the possibility to continue with flannel in RKE2.

## Proposal

Include flannel as a CNI plugin alternative for RKE2. We know that it has limitations and it is really simple but it seems it could be enough for Windows users that are feeling the pain of the Calico TCP Resets

### Strength

* We can offer a plan B for customers that can't afford getting TCP Resets
* We have an alternative while Microsoft and Tigera fix the problem
* We are maintainers of flannel and could support it easily

### Weakness
* Yet another cni plugin to support
* Flannel is very limited. We should document it very well to avoid disappointment on non-knowledgeable customers
