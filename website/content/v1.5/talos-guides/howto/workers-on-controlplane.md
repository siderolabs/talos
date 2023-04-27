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

This may be done via editing the `controlplane.yaml` file before it is applied to the controlplane nodes, by `talosctl edit machineconfig`, or by [patching the machine config]({{< relref "../configuration/patching">}}).

> Note: if you edit or patch the machine config on a running control plane node to set `allowSchedulingOnControlPlanes: true`, it will be applied immediately, but will not have any effect until the next reboot.
You may reboot the nodes via `talosctl reboot`.

You may also immediately make the control plane nodes schedulable by running the below:

```bash
kubectl taint nodes --all  node-role.kubernetes.io/control-plane-
```

Note that unless `allowSchedulingOnControlPlanes: true` is set in the machine config, the nodes will be tainted again on next reboot.
