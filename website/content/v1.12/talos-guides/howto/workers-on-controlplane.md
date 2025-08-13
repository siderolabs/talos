---
title: "How to enable workers on your control plane nodes"
description: "How to enable workers on your control plane nodes."
aliases:

---

By default, Talos Linux taints control plane nodes so that workloads are not schedulable on them.

In order to allow workloads to run on the control plane nodes (useful for single node clusters, or non-production clusters), follow the procedure below.

Modify the machine configuration for the controlplane nodes to add `allowSchedulingOnControlPlanes: true`:

```yaml
cluster:
    allowSchedulingOnControlPlanes: true
```

This may be done via editing the `controlplane.yaml` file before it is applied to the control plane nodes, by [editing the machine config]({{< relref "../configuration/editing-machine-configuration" >}}), or by [patching the machine config]({{< relref "../configuration/patching">}}).

## Load Balancer configuration

When a load balancer such as MetalLB is used, the nodeLabel `node.kubernetes.io/exclude-from-external-load-balancers` should also be removed from the control plane nodes.
This label is added by default and instructs load balancers to exclude the node from the list of backend servers used by external load balancers.

In order to remove this label, you can patch the machine configuration for the control plane nodes with the patch:

```yaml
machine:
  nodeLabels:
    node.kubernetes.io/exclude-from-external-load-balancers:
      $patch: delete
```
