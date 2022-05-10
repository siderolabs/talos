---
title: What's New in Talos 1.0
weight: 50
description: "List of new and shiny features in Talos Linux."
---

## Announcements

### GitHub Organization Change

Talos Linux and other repositories were migrated from the `talos-systems` GitHub organization
to the `siderolabs` organization (github.com/talos-systems -> github.com/siderolabs).

Existing Talos Linux container images (`installer`, `talos`, etc.) are mirrored across both organizations,
but all new images will only be available from `ghcr.io/siderolabs` going forward.

For example, when upgrading Talos use `ghcr.io/siderolabs` instead of `ghcr.io/talos-systems`:

```bash
talosctl upgrade --image ghcr.io/siderolabs/installer:v1.0.0
```

## Extending Talos

### System Extensions

System extensions allow changes to the Talos root filesystem, and can be used to enable different features, including custom
container runtimes, additional firmware, among others.

System extensions are only activated during Talos installation (or upgrade).
Even with system extensions installed, the Talos root filesystem is still immutable and read-only.

Please see [extensions repository](https://github.com/talos-systems/extensions) and [documentation]({{< relref "../talos-guides/configuration/system-extensions/" >}}) for more information.

### Extension Services

Talos now provides a way to extend the system services that Talos runs with [extension services]({{< relref "../advanced/extension-services" >}}).
Extension services should be included in the Talos root filesystem (i.e. via system extensions).

### Static Pods in the Machine Configuration

Talos now accepts [static pod definitions]({{< relref "../advanced/static-pods" >}}) in the `.machine.pods` key of the machine configuration.
Please note that static pod definitions are not validated by Talos, and can be updated without a node reboot.

## Kubernetes

### Kubelet

Kubelet configuration can now be overridden with the `.machine.kubelet.extraConfig` machine configuration field.
As most of the kubelet command line arguments are being deprecated, it is recommended to migrate to `extraConfig`
in place of using `extraArgs`.

A number of conformance tweaks have been made to the `kubelet` to allow it to run without
`protectKernelDefaults`.
This includes both kubelet configuration options and sysctls.
Of particular note is that Talos now sets the `kernel.panic` reboot interval to 10s instead of 1s.
If your kubelet fails to start after the upgrade, please check the `kubelet` logs to determine the problem.

Talos now performing a graceful kubelet shutdown by default on both node shutdown and reboot.
Default shutdown timeouts are 20s for regular priority pods and 10s for critical priority pods.
Timeouts can be overridden with the `.machine.kubelet.extraConfig` machine configuration keys:
`shutdownGracePeriod` and `shutdownGracePeriodCriticalPods`.

### Admission Plugin Configuration

Talos now supports the Kubernetes API server admission plugin configuration via the `.cluster.apiServer.admissionControl` machine configuration field.

This configuration can be used to enable [Pod Security Admission](https://kubernetes.io/docs/concepts/security/pod-security-admission/) plugin and
define cluster-wide default [Pod Security Standards](https://kubernetes.io/docs/concepts/security/pod-security-standards/).

### Pod Security Policy

The Pod Security Policy Kubernetes feature is deprecated and is going to be removed in Kubernetes 1.25.
Talos by default skips setting up PSP with this release (see machine configuration `.cluster.apiServer.disablePodSecurityPolicy`).

### Pinned Kubernetes Version

Command `talosctl gen config` now defaults to Kubernetes version pinning when generating machine configuration.
Previously the default was to omit an explicit Kubernetes version, so Talos picked up the default version it was built against.
Old behavior can be achieved by specifying empty flag value: `--kubernetes-version=`.

### API Server Audit Logs

`kube-apiserver` is now configured to store its audit logs separately from the `kube-apiserver` standard logs and log directly to file.
The `kube-apiserver` will maintain the rotation and retirement of these logs, which are stored in `/var/log/audit/`.
Previously, the audit logs were sent to the `kube-apiserver` `stdout` (along with the rest of its logs) to be collected in the usual manner by Kubernetes.

## Machine Configuration

Talos now preserves machine configuration byte-for-byte as it was submitted to the node.
This means that custom comments and overall machine configuration structure is now preserved.
This allows automation of machine configuration updates via an external mechanism without loss of information.

### Patching Enhancements

`talosctl` commands which accept JSON patches (i.e. `gen config`, `cluster create`, `patch machineconfig`) now support multiple patches, loading patches
from files with `@file.json` syntax, as well as support loading patches with a YAML format.

### Apply Config Enhancements

`talosctl apply/patch/edit` cli commands got revamped.
Separate flags `--on-reboot`, `--immediate`, `--interactive` were replaced
with a single `--mode` flag that can take the following values:

- `auto` new mode that automatically applies the configuration in no-reboot/reboot mode based on the change.
- `no-reboot` force apply immediately, if that is not possible then it fails.
- `reboot` force reboot with applied config.
- `staged` write new machine configuration to [STATE]({{< relref "../learn-more/architecture/#file-system-partitions" >}}), but don't apply it (it will be applied after a reboot).
- `interactive` starts interactive installer, only for `apply`.

## Networking

### Early Boot `bond` Configuration

Talos now supports setting a bond interface from the kernel cmdline using the [`bond=` option](https://man7.org/linux/man-pages/man7/dracut.cmdline.7.html)

## Platforms

### Equinix Metal

`talos.platform` for [Equinix Metal]({{< relref "../talos-guides/install/bare-metal-platforms/equinix-metal" >}}) is renamed from `packet` to `equinixMetal`, the older name is still supported for backwards compatibility.

### Oracle Cloud

Talos now supports [Oracle Cloud]({{< relref "../talos-guides/install/cloud-platforms/oracle" >}}).

### Network Configuration

Platform network configuration was rewritten to avoid modifying Talos machine configuration.
Network configuration is performed independently of the machine configuration presence, so it works
even if Talos is booted into maintenance mode, and without machine configuration in the platform userdata.

### SBCs

Talos now supports [Jetson Nano SBC]({{< relref "../talos-guides/install/single-board-computers/jetson_nano" >}}).

## Component Updates

- Linux: 5.15.32
- Kubernetes: 1.23.5
- CoreDNS: 1.9.1
- etcd: 3.5.2
- containerd: 1.6.2
- runc: 1.1.0

Talos is built with Go 1.17.8

## Hardware

### NVIDIA GPU alpha Support

Talos now has alpha support for NVIDIA GPU based workloads.
Check the [NVIDA GPU support guide]({{< relref "../talos-guides/configuration/nvidia-gpu" >}}) for details.

## Miscellaneous

### Sysfs Kernel Parameters

Talos now supports setting `sysfs` kernel parameters  (`/sys/...`).
Use machine configuration field `.machine.sysfs` to set `sysfs` kernel parameters.

### Wipe System Kernel Parameter

Talos added a new kernel parameter `talos.experimental.wipe=system` which can help resetting the system disk of the machine
and start over with a fresh installation.
See [Resetting a Machine]({{< relref "../talos-guides/resetting-a-machine#kernel-parameter" >}}) on how to use it.
