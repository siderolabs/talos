---
title: "Resetting a Machine"
description: ""
---

From time to time, it may be beneficial to reset a Talos machine to its "original" state.
Bear in mind that this is a destructive action for the given machine.
Doing this means removing the machine from Kubernetes, Etcd (if applicable), and clears any data on the machine that would normally persist a reboot.

## CLI

> WARNING: Running a `talosctl reset` on cloud VM's might result in the VM being unable to boot as this wipes the entire disk.
It might be more useful to just wipe the `STATE` and `EPHEMERAL` partitions on a cloud VM if not booting via `iPXE`.
`talosctl reset --system-labels-to-wipe STATE --system-labels-to-wipe EPHEMERAL`

The API command for doing this is `talosctl reset`.
There are a couple of flags as part of this command:

```bash
Flags:
      --graceful                        if true, attempt to cordon/drain node and leave etcd (if applicable) (default true)
      --reboot                          if true, reboot the node after resetting instead of shutting down
      --system-labels-to-wipe strings   if set, just wipe selected system disk partitions by label but keep other partitions intact keep other partitions intact
```

The `graceful` flag is especially important when considering HA vs. non-HA Talos clusters.
If the machine is part of an HA cluster, a normal, graceful reset should work just fine right out of the box as long as the cluster is in a good state.
However, if this is a single node cluster being used for testing purposes, a graceful reset is not an option since Etcd cannot be "left" if there is only a single member.
In this case, reset should be used with `--graceful=false` to skip performing checks that would normally block the reset.

## Kernel Parameter

Another way to reset a machine is to specify `talos.experimental.wipe=system` kernel parameter.
If the machine got stuck in the boot loop and you access to the console you can use GRUB to specify this kernel argument.
Then when Talos boots for the next time it will reset system disk and reboot.

Next steps can be to install Talos either using PXE boot or by mounting an ISO.
