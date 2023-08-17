---
title: "How to scale down a Talos cluster"
description: "How to remove nodes from a Talos Linux cluster."
aliases:

---

To remove nodes from a Talos Linux cluster:

- `talosctl -n <IP.of.node.to.remove> reset`
- `kubectl delete node <nodename>`

The command [`talosctl reset`]({{< relref "../../reference/cli/#talosctl-reset">}}) will cordon and drain the node, leaving `etcd` if required, and then erase its disks and power down the system.

This command will also remove the node from registration with the discovery service, so it will no longer show up in `talosctl get members`.

It is still necessary to remove the node from Kubernetes, as noted above.
