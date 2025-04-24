---
title: What's New in Talos 1.10.0
weight: 50
description: "Discover the latest features and updates in Talos Linux 1.10."
---

For critical changes, refer to the [upgrade notes]({{< relref "../talos-guides/upgrading-talos" >}}).

## Breaking Changes

### UEFI Boot

Talos 1.10 now uses the `systemd-boot` [bootloader]({{< relref "../talos-guides/install/bare-metal-platforms/bootloader" >}}) and [Unified Kernel Images (UKIs)](https://uapi-group.org/specifications/specs/unified_kernel_image/) for UEFI systems.
Previously, this was limited to Secure Boot systems.
Upgrades from Talos 1.9 retain the existing bootloader, so this applies only to fresh installations.

UKIs bundle the kernel, initramfs, and kernel command line arguments into a single file, making kernel arguments unmodifiable without upgrading the UKI.
Consequently, the `.machine.install.extraKernelArgs` field in the machine config is ignored when using `systemd-boot`.

Ensure the correct platform-specific `installer` image is used during upgrades or installations, as it includes Talos-specific `talos.platform` arguments.
Tools like [Image Factory](https://factory.talos.dev/) and [Omni](https://www.siderolabs.com/platform/saas-for-kubernetes/) handle this automatically.
Image Factory now supports `<platform>-installer` images (e.g., `aws-installer` for Amazon EC2) with the appropriate kernel arguments.

### System Extensions

Starting with Talos 1.10, `.machine.install.extensions` is deprecated and has no effect.
The field remains for compatibility with older versions.
Use [Boot Assets]({{< relref "../talos-guides/install/boot-assets" >}}) instead.
The `installer` image is now smaller as tools for host-side extension installation have been removed.

### `cgroups` v1

Talos no longer supports `cgroupsv1` in non-container mode.
The kernel argument `talos.unified_cgroup_hierarchy` is ignored.

> Note: Talos has defaulted to `cgroups` v2 for a long time, so this change should not impact most users.

## New Features

### User Volumes

Talos introduces [user disk volumes]({{< relref "../talos-guides/configuration/disk-management#user-volumes" >}}) via the [`UserVolumeConfig`]({{< relref "../reference/configuration/block/uservolumeconfig" >}}) machine config.
The `.machine.disks` field is deprecated but remains for backward compatibility.

### Driver Rebind

A new machine config, [`PCIDriverRebindConfig`]({{< relref "../reference/configuration/hardware/pcidriverrebindconfig" >}}), allows rebinding PCI device drivers to different targets.

### Ethernet Configuration

Talos now supports `ethtool`-style [Ethernet configuration]({{< relref "../talos-guides/network/ethernet-config" >}}) via [`EthernetConfig`]({{< relref "../reference/configuration/network/ethernetconfig" >}}).
Interface status can be checked with `talosctl get ethernetstatus`.

### Dual-Boot Disk Images and ISOs

For x86, Talos provides dual-boot disk and ISO images that use GRUB for legacy BIOS and `systemd-boot` for UEFI.
On first boot, Talos determines the boot method and removes the unused bootloader.

For arm64, Talos now uses `systemd-boot`.
Secure Boot images exclusively use `systemd-boot` as Secure Boot is UEFI-only.

[Imager]({{< relref "../talos-guides/install/boot-assets" >}}) supports bootloader selection when generating disk images:

```yaml
output:
  kind: image
  imageOptions:
    bootloader: sd-boot # supported options are sd-boot, grub, dual-boot
```

### SELinux

Talos Linux by default now ships an experimental SELinux policy which protects the base operating system from unauthorized access.
The default SELinux mode is `permissive`, meaning that violations are logged but not enforced.
See [SELinux]({{< relref "../advanced/selinux" >}}) for details.

## Component Updates

* Linux: 6.12.24
* CNI plugins: 1.6.2
* runc: 1.2.6
* containerd: 2.0.5
* etcd: 3.5.20
* Flannel: 0.26.7
* Kubernetes: 1.33.0
* CoreDNS: 1.12.1

Talos is built with Go 1.24.2.

## Other Changes

### auditd

Disable Talos' built-in `auditd` service using the kernel parameter `talos.auditd.disabled=1`.

### iSCSI Initiator

Talos now generates `/etc/iscsi/initiatorname.iscsi` based on node identity, ensuring a deterministic IQN.
Update iSCSI targets to use the new IQN, which can be read with `talosctl read /etc/iscsi/initiatorname.iscsi`.

### NVMe NQN

Talos generates `/etc/nvme/hostnqn` and `/etc/nvme/hostid` based on node identity.
The NQN can be read with `talosctl read /etc/nvme/hostnqn`.

### Ingress Firewall

The Ingress Firewall now correctly filters access to Kubernetes NodePort services.

### `kube-apiserver` Authorization Config

The `.cluster.apiServer.authorizationConfig` field now respects the user-defined order of authorizers.
If `Node` and `RBAC` are not explicitly specified, they are appended to the end.

Example:

```yaml
cluster:
  apiServer:
    authorizationConfig:
      - type: Node
        name: Node
      - type: Webhook
        name: Webhook
        webhook:
          connectionInfo:
            type: InClusterConfig
        ...
      - type: RBAC
        name: rbac
```

The `authorization-mode` CLI argument does not support this customization.

### Fully Bootstrapped Builds

Talos 1.10 is built using [[Stageˣ]](https://stagex.tools/), enhancing reproducibility, auditability, and security.
The root filesystem now uses a unified `/usr` structure, with other directories symlinking to `/usr/bin` and `/usr/lib`.
Third-party extensions must adjust their directories accordingly.
