---
title: What's New in Talos 1.11.0
weight: 50
description: "Discover the latest features and updates in Talos Linux 1.11."
---

For critical changes, refer to the [upgrade notes]({{< relref "../talos-guides/upgrading-talos" >}}).

## Important Changes

### Kubernetes Version Validation

Talos now validates Kubernetes version in the image submitted in the machine configuration for [compatibility]({{< relref "support-matrix" >}}).
Previously this check was performed only on upgrade, but now it is consistently applied to upgrade, initial provisioning, and machine configuration updates.
The default image references have the version tag, e.g. `ghcr.io/siderolabs/kubelet:v1.33.0`, which is used for validation.

This implies that all image references should contain the tag, even if the image is pinned by digest (e.g. `ghcr.io/siderolabs/kubelet:v1.33.0@sha256:0072b6738306b927cb85ad53999c2f9691f2f533cff22f4afc30350c3b9e62bb`).

## Disk Management

### Raw and Existing Volumes

In addition to the existing support for [user volumes]({{< relref "../talos-guides/configuration/disk-management/user" >}}), Talos now supports two new types of volumes:

* [raw volumes]({{< relref "../talos-guides/configuration/disk-management/raw" >}}) - allows to allocate unformatted disk space as a partition.
* [existing volumes]({{< relref "../talos-guides/configuration/disk-management/existing" >}}) - allows to use existing data partitions or disks.

### Swap and Zswap

Talos now supports [swap]({{< relref "../talos-guides/configuration/swap" >}}) on block devices, and `zswap`, a compressed cache for swap pages.

### Disk Encryption

[Disk encryption]({{< relref "../talos-guides/configuration/disk-encryption" >}}) for system volumes is now managed by the [`VolumeConfig`]({{< relref "../reference/configuration/block/volumeconfig" >}}) machine configuration document.
Legacy configuration in `v1alpha1` (`.machine.systemDiskEncryption`) machine configuration is still supported.

New per encryption key option `lockToSTATE` is added to the `VolumeConfig` document, which allows to lock the volume encryption key to the secret salt in the `STATE` volume.
So, if the `STATE` volume is wiped or replaced, the volume encryption key will not be usable anymore.

### Disk Wipe

Talos now supports `talosctl disk wipe` command in maintenance mode (`talosctl disk wipe <disk> --insecure`).

## SBOMs

Talos now publishes [Software Bill of Materials (SBOM)]({{< relref "../advanced/sbom" >}}) in the SPDX format.

## `etcd`

The default version of `etcd` in Talos is now 3.6.4.

As `etcd` 3.6.x introduce a new storage format, Talos now supports the `etcd` [downgrade API]({{< relref "../advanced/etcd-maintenance#downgrade-v36-to-v35" >}}) to allow downgrading the `etcd` cluster (storage format) to a previous version
if downgrading to 3.5.x is required.

## Platform Updates

### Azure

Talos on Azure now defaults to MTU of 1400 bytes for the `eth0` interface to avoid packet fragmentation issues.
The default MTU can be overridden with machine configuration.

### VMware

Talos VMWare platform now supports `arm64` architecture in addition to `amd64`.

## `talosctl cluster create` with QEMU on macOS

When using Apple Silicon, Talos now supports running the [`talosctl cluster create` command with the QEMU provisioner on macOS]({{< relref "../talos-guides/install/local-platforms/qemu" >}}).
This was previously only supported on Linux.
This feature allows to create a test Talos cluster on macOS using QEMU virtual machines, and use it to verify Talos configuration and features.

## Miscellaneous Changes

### Bootloader

Talos increases the default boot partition size to 2 GiB to accommodate larger images (with many system extensions included).
On UEFI systems with `systemd-boot` (default since Talos 1.10), Talos now installs itself correctly into the UEFI boot order.

### New Resources

Talos Linux now provides several new resources for inspecting the system state:

* the kernel command line as a `KernelCmdline` resource (`talosctl get cmdline`);
* the list of loaded modules as a `LoadedKernelModule` resource (`talosctl get modules`);
* the booted entry (for `systemd-boot`) as a `BootedEntry` resource (`talosctl get bootedentry`).

### IMA support removed

Talos now drops the IMA (Integrity Measurement Architecture) support.
This feature was not used in Talos for any meaningful security purpose
and has historically caused performance issues.

### Early Inline Configuration

Talos now supports passing early inline configuration via the `talos.config.early` [kernel parameter]({{< relref "../reference/kernel#talosconfigearly-and-talosconfiginline" >}}).
This allows to pass the configuration before the platform config source is probed, which is useful for early boot configuration.
The value of this parameter has same format as the `talos.config.inline` parameter, i.e. it should be base64 encoded and zstd-compressed.

## Component Updates

* Linux: 6.12.40
* Kubernetes: 1.34.0-rc.1
* runc: 1.3.0
* etcd: 3.6.4
* containerd: 2.1.4
* Flannel CNI plugin: 1.7.1-flannel1
* Flannel: 0.27.2
* CoreDNS: 1.12.2
* xfsprogs: 6.15.0
* systemd-udevd and systemd-boot: 257.7
* lvm2: 2.03.33
* cryptsetup: 2.8.0

Talos is built with Go 1.24.6.

## Contributors

* Andrey Smirnov
* Noel Georgi
* Dmitrii Sharshakov
* Orzelius
* Mateusz Urbanek
* Orzelius
* Justin Garrison
* Spencer Smith
* Steve Francis
* Till Hoffmann
* Utku Ozdemir
* Andrew Longwill
* Artem Chernyshev
* Michael Robbins
* Alexandre GV
* Marat Bakeev
* Oguz Kilcan
* Olav Thoresen
* Thibault VINCENT
* 459below
* Alvaro "Chamo" Linares Cabre
* Amarachi Iheanacho
* Brian Brookman
* Bryan Mora
* Clément Nussbaumer
* Damien
* David R
* Dennis Marttinen
* Dmitriy Matrenichev
* Joakim Nohlgård
* Jorik Jonker
* Justin Seely
* Luke Cousins
* Marco Mihai Condrache
* Markus Reiter
* Martyn Ranyard
* Michael Moerz
* Mike
* Misha Aksenov
* Tan Siewert
* Tom
* Tom Keur
* jvanthienen-gluo
* killcity
* yashutanu
