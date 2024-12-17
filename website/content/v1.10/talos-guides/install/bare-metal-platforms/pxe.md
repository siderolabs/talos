---
title: "PXE"
description: "Booting Talos over the network on bare-metal with PXE."
---

Talos can be installed on bare-metal using PXE service.
There are more detailed guides for PXE booting using [Matchbox]({{< relref "matchbox">}}).

This guide describes generic steps for PXE booting Talos on bare-metal.

First, download the `vmlinuz` and `initramfs` assets from the [Talos releases page](https://github.com/siderolabs/talos/releases/latest/).
Set up the machines to PXE boot from the network (usually by setting the boot order in the BIOS).
There might be options specific to the hardware being used, booting in BIOS or UEFI mode, using iPXE, etc.

Talos requires the following kernel parameters to be set on the initial boot:

* `talos.platform=metal`
* `slab_nomerge`
* `pti=on`

When booted from the network without machine configuration, Talos will start in maintenance mode.

Please follow the [getting started guide]({{< relref "../../../introduction/getting-started" >}}) for the generic steps on how to install Talos.

See [kernel parameters reference]({{< relref "../../../reference/kernel" >}}) for the list of kernel parameters supported by Talos.

> Note: If there is already a Talos installation on the disk, the machine will boot into that installation when booting from network.
> The boot order should prefer disk over network.

Talos can automatically fetch the machine configuration from the network on the initial boot using `talos.config` kernel parameter.
A metadata service (HTTP service) can be implemented to deliver customized configuration to each node for example by using the MAC address of the node:

```text
talos.config=https://metadata.service/talos/config?mac=${mac}
```

> Note: The `talos.config` kernel parameter supports other substitution variables, see [kernel parameters reference]({{< relref "../../../reference/kernel" >}}) for the full list.

PXE booting can be also performed via [Image Factory]({{< relref "../../../learn-more/image-factory" >}}).
