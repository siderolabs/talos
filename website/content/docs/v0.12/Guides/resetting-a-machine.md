---
title: "Resetting a Machine"
description: ""
---

From time to time, it may be beneficial to reset a Talos machine to its "original" state.
Bear in mind that this is a destructive action for the given machine.
Doing this means removing the machine from Kubernetes, Etcd (if applicable), and clears any data on the machine that would normally persist a reboot.

The API command for doing this is `talosctl reset`.
There are a couple of flags as part of this command:

```bash
Flags:
      --graceful   if true, attempt to cordon/drain node and leave etcd (if applicable) (default true)
      --reboot     if true, reboot the node after resetting instead of shutting down
```

The `graceful` flag is especially important when considering HA vs. non-HA Talos clusters.
If the machine is part of an HA cluster, a normal, graceful reset should work just fine right out of the box as long as the cluster is in a good state.
However, if this is a single node cluster being used for testing purposes, a graceful reset is not an option since Etcd cannot be "left" if there is only a single member.
In this case, reset should be used with `--graceful=false` to skip performing checks that would normally block the reset.
