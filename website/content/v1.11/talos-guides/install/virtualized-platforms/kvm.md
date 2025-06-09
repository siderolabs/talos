---
title: "KVM"
aliases:
  - ../../../virtualized-platforms/kvm
---

Talos is known to work on KVM.

We don't yet have a documented guide specific to KVM; however, you can have a look at our
[Vagrant & Libvirt guide]({{< relref "./vagrant-libvirt" >}}) which uses KVM for virtualization.

Also [`talosctl cluster create` with QEMU]({{< relref "../local-platforms/qemu" >}}) uses KVM under the hood.

> Note: For the network interface emulation, `virtio` and `e1000` are supported.
