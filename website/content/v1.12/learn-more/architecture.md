---
title: "Architecture"
weight: 20
description: "Learn the system architecture of Talos Linux itself."
---

Talos is designed to be **atomic** in _deployment_ and **modular** in _composition_.

It is atomic in that the entirety of Talos is distributed as a
single, self-contained image, which is versioned, signed, and immutable.

It is modular in that it is composed of many separate components
which have clearly defined gRPC interfaces which facilitate internal flexibility
and external operational guarantees.

All of the main Talos components communicate with each other by gRPC, through a socket on the local machine.
This imposes a clear separation of concerns and ensures that changes over time which affect the interoperation of components are a part of the public git record.
The benefit is that each component may be iterated and changed as its needs dictate, so long as the external API is controlled.
This is a key component in reducing coupling and maintaining modularity.

## File system partitions

Talos uses these partitions with the following labels:

1. **EFI** - stores EFI boot data.
1. **BIOS** - used for GRUB's second stage boot.
1. **BOOT** - used for the boot loader, stores initramfs and kernel data.
1. **META** - stores metadata about the talos node, such as node id's.
1. **STATE** - stores machine configuration, node identity data for cluster discovery and KubeSpan info
1. **EPHEMERAL** - stores ephemeral state information, mounted at `/var`

## The File System

One of the unique design decisions in Talos is the layout of the root file system.
There are three "layers" to the Talos root file system.
At its core the rootfs is a read-only squashfs.
The squashfs is then mounted as a loop device into memory.
This provides Talos with an immutable base.

The next layer is a set of `tmpfs` file systems for runtime specific needs.
Aside from the standard pseudo file systems such as `/dev`, `/proc`, `/run`, `/sys` and `/tmp`, a special `/system` is created for internal needs.
One reason for this is that we need special files such as `/etc/hosts`, and `/etc/resolv.conf` to be writable (remember that the rootfs is read-only).
For example, at boot Talos will write `/system/etc/hosts` and then bind mount it over `/etc/hosts`.
This means that instead of making all of `/etc` writable, Talos only makes very specific files writable under `/etc`.

All files under `/system` are completely recreated on each boot.
For files and directories that need to persist across boots, Talos creates `overlayfs` file systems.
The `/etc/kubernetes` is a good example of this.
Directories like this are `overlayfs` backed by an XFS file system mounted at `/var`.

The `/var` directory is owned by Kubernetes with the exception of the above `overlayfs` file systems.
This directory is writable and used by `etcd` (in the case of control plane nodes), the kubelet, and the CRI (containerd).
Its content survives machine reboots and on machine upgrades, but it is wiped and lost on resets, unless the
`--system-labels-to-wipe` option of [`talosctl reset`]({{< relref "../reference/cli#talosctl-reset" >}})
is used.
