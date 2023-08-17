---
title: "How to scale up a Talos cluster"
description: "How to add more nodes to a Talos Linux cluster."
aliases:

---

To add more nodes to a Talos Linux cluster, follow the same procedure as when initially creating the cluster:

- boot the new machines to install Talos Linux
- apply the `worker.yaml` or `controlplane.yaml` configuration files to the new machines

You need the `controlplane.yaml` and `worker.yaml` that were created when you initially deployed your cluster.
These contain the certificates that enable new machines to join.

Once you have the IP address, you can then apply the correct configuration for each machine you are adding, either `worker` or `controlplane`.

```bash
  talosctl apply-config --insecure \
    --nodes [NODE IP] \
    --file controlplane.yaml
```

The insecure flag is necessary because the PKI infrastructure has not yet been made available to the node.

You do not need to bootstrap the new node.
Regardless of whether you are adding a control plane or worker node, it will now join the cluster in its role.
