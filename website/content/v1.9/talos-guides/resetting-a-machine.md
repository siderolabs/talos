---
title: "Resetting a Machine"
description: "Steps on how to reset a Talos Linux machine to a clean state."
aliases:
  - ../guides/resetting-a-machine
---

Occasionally, it may be necessary to reset a Talos machine to its "original" state.
Keep in mind that this is a destructive action for the given machine.
This process involves removing the machine from Kubernetes, `etcd` (if applicable), and clearing any data on the machine that would normally persist after a reboot.

## CLI

To reset a machine, use the `talosctl reset` command:

```sh
talosctl reset -n <node_ip_to_be_reset>
```

> WARNING: Running `talosctl reset` on cloud VMs might result in the VM being unable to boot as this wipes the entire disk.
> It might be more practical to only wipe the [STATE]({{< relref "../learn-more/architecture/#file-system-partitions" >}}) and [EPHEMERAL]({{< relref "../learn-more/architecture/#file-system-partitions" >}}) partitions on a cloud VM if not booting via `iPXE`.
> `talosctl reset --system-labels-to-wipe STATE --system-labels-to-wipe EPHEMERAL`

The command includes several flags:

```bash
Flags:
      --graceful                        if true, attempt to cordon/drain node and leave etcd (if applicable) (default true)
      --reboot                          if true, reboot the node after resetting instead of shutting down
      --system-labels-to-wipe strings   if set, just wipe selected system disk partitions by label but keep other partitions intact
```

The `graceful` flag is particularly important when considering HA vs. non-HA Talos clusters.
If the machine is part of an HA cluster, a normal, graceful reset should work fine as long as the cluster is in a good state.
However, if this is a single-node cluster used for testing purposes, a graceful reset is not an option since `etcd` cannot be "left" if there is only a single member.
In this case, use the reset command with `--graceful=false` to skip checks that would normally block the reset.

## Kernel Parameter

Another method to reset a machine is by specifying the `talos.experimental.wipe=system` kernel parameter.
If the machine is stuck in a boot loop and you have access to the console, you can use GRUB to specify this kernel argument.
When Talos boots next, it will reset the system disk and reboot.

The next steps can include installing Talos either using PXE boot or by mounting an ISO.
