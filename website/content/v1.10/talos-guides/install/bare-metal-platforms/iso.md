---
title: "ISO"
description: "Booting Talos on bare-metal with ISO."
---

Talos can be installed on bare-metal machine using an ISO image.
ISO images for `amd64` and `arm64` architectures are available on the [Talos releases page](https://github.com/siderolabs/talos/releases/latest/).

Talos doesn't install itself to disk when booted from an ISO until the machine configuration is applied.

Please follow the [getting started guide]({{< relref "../../../introduction/getting-started" >}}) for the generic steps on how to install Talos.

> Note: If there is already a Talos installation on the disk, the machine will boot into that installation when booting from a Talos ISO.
> The boot order should prefer disk over ISO, or the ISO should be removed after the installation to make Talos boot from disk.

See [kernel parameters reference]({{< relref "../../../reference/kernel" >}}) for the list of kernel parameters supported by Talos.

There are two flavors of ISO images available:

* `metal-<arch>.iso` supports booting on BIOS and UEFI systems (for x86, UEFI only for arm64)
* `metal-<arch>-secureboot.iso` supports booting on only UEFI systems in SecureBoot mode (via [Image Factory]({{< relref "../../../learn-more/image-factory" >}}))
