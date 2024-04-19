---
title: "How to enable workers on your control plane nodes"
description: "How to enable workers on your control plane nodes."
aliases:

---

By default, Talos Linux taints control plane nodes so that workloads are not schedulable on them.

In order to allow workloads to run on the control plane nodes (useful for single node clusters, or non-production clusters), follow the procedure below.

Modify the MachineConfig for the controlplane nodes to add `allowSchedulingOnControlPlanes: true`:

```yaml
cluster:
    allowSchedulingOnControlPlanes: true
```

This may be done via editing the `controlplane.yaml` file before it is applied to the control plane nodes, by [editing the machine config]({{< relref "../configuration/editing-machine-configuration" >}}), or by [patching the machine config]({{< relref "../configuration/patching">}}).
