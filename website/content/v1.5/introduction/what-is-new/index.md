---
title: What's New in Talos 1.5
weight: 50
description: "List of new and shiny features in Talos Linux."
---

See also [upgrade notes]({{< relref "../talos-guides/upgrading-talos">}}) for important changes.

## Predictable Network Interface Names

Starting with version Talos 1.5, network interfaces are renamed to [predictable names](https://www.freedesktop.org/wiki/Software/systemd/PredictableNetworkInterfaceNames/)
same way as `systemd` does that in other Linux distributions.

The naming schema `enx78e7d1ea46da` (based on MAC addresses) is enabled by default, the order of interface naming decisions is:

* firmware/BIOS provided index numbers for on-board devices (example: `eno1`)
* firmware/BIOS provided PCI Express hotplug slot index numbers (example: `ens1`)
* physical/geographical location of the connector of the hardware (example: `enp2s0`)
* interfaces's MAC address (example: `enx78e7d1ea46da`)

The predictable network interface names features can be disabled by specifying `net.ifnames=0` in the kernel command line.
Talos automatically adds the `net.ifnames=0` kernel argument when upgrading from Talos versions before 1.5, so upgrades to 1.5 don't require any manual intervention.

This change doesn't affect "cloud" platforms, like AWS, as Talos automatically adds `net.ifnames=0` to the kernel command line.

## SecureBoot

Talos now supports booting on UEFI systems in [SecureBoot]({{< relref "../../talos-guides/install/bare-metal-platforms/secureboot.md" >}}) mode.
When combined with TPM-based disk encryption, this provides Trusted Boot experience.

## Boot Assets Generation

Talos provides [a new unified way]({{< relref "../../talos-guides/install/boot-assets.md" >}}) to generate various boot assets, including ISOs, disk images, PXE boot files, installer container images etc., which can be
further customized with system extensions, extra kernel arguments.

## Kubernetes

### KubePrism - Kubernetes API Server In-Cluster Load Balancer

Talos now supports configuring the [KubePrism]({{< relref "../../kubernetes-guides/configuration/kubeprism.md">}}) - Kubernetes API Server in-cluster load balancer with machine config
`features.kubePrism.port` and `features.kubePrism.enabled` fields.

If enabled, KubePrism binds to `localhost` and runs on the same port on every machine in the cluster.
The default value for KubePrism endpoint is https://localhost:7445.

The KubePrism is used by the `kubelet`, `kube-scheduler`, `kube-controller-manager`
and `kube-proxy` by default and can be passed to the CNIs like Cilium and Calico.

The KubePrism provides access to the Kubernetes API endpoint even if the external loadbalancer
is not healthy, provided that the worker nodes can reach to the controlplane machine addresses directly.

### XFS Quota

Talos 1.5+ enables XFS project quota support by default, also enabling by default
kubelet feature gate `LocalStorageCapacityIsolationFSQuotaMonitoring` to use xfs quotas
to monitor volume usage instead of `du`.

This feature is controlled by the `.machine.features.diskQuotaSupport` field in the machine config,
it is set to true for new clusters.

When upgrading from a previous version, the feature can be enabled by setting the field to true.
On the first mount of a volume, the quota information will be recalculated, which may take some time.

## System Extensions

### Installing System Extensions

The way to install system extensions on the machine using `machine.install.extensions` machine configuration option is now deprecated,
please use instead [the boot asset generation process]({{< relref "../../talos-guides/install/boot-assets.md" >}}) to create an image with system extension pre-installed.

### Extension Services

Talos now supports setting `environmentFile` for an [extension service container spec]({{< relref "../../advanced/extension-services.md#container" >}}).
The extension waits for the file to be present before starting the service.

## Disk Encryption

### TPM-based Disk Encryption

Talos now supports encrypting `STATE`/`EPHEMERAL` with [keys bound to a TPM device]({{< relref "../../talos-guides/install/bare-metal-platforms/secureboot.md" >}}).
The TPM device must be TPM2.0 compatible.
This type of disk encryption should be used when booting Talos in SecureBoot mode.

Example machine config:

```yaml
machine:
  systemDiskEncryption:
    ephemeral:
      provider: luks2
      keys:
        - slot: 0
          tpm: {}
    state:
      provider: luks2
      keys:
        - slot: 0
          tpm: {}
```

### Network KMS Disk Encryption

Talos now supports new type of encryption keys which are sealed/unsealed with an external KMS server:

```yaml
machine:
  systemDiskEncryption:
    ephemeral:
      provider: luks2
      keys:
        - kms:
            endpoint: https://1.2.3.4:443
          slot: 0
```

gRPC API definitions and a simple reference implementation of the KMS server can be found in this
[repository](https://github.com/siderolabs/kms-client/blob/main/cmd/kms-server/main.go).

## Container Images

### `talosctl image` Command

A new set of commands was introduced to manage container images in the CRI:

* `talosctl image list` shows list of available images
* `talosctl image pull` allows to pre-pull an image into the CRI

Both new commands accept `--namespace` flag with two possible values:

* `cri` (default): images managed by the CRI (Kubernetes workloads)
* `system`: images managed by Talos (`etcd` and `kubelet`)

### `talosctl upgrade-k8s` Image Pre-pulling

The command `talosctl upgrade-k8s` now by default pre-pulls images for Kubernetes controlplane components
and kubelet.
This provides an early check for missing images, and minimizes downtime during Kubernetes
rolling component update.

## Component Updates

* Linux: 6.1.45
* containerd: 1.6.23
* runc: 1.1.9
* etcd: 3.5.9
* Kubernetes: 1.28.0
* Flannel: 0.22.1

Talos is built with Go 1.20.7.

Talos now builds many device drivers as kernel modules in the x86 Linux kernel, which get automatically loaded on boot based on the hardware detected.

## Deprecations

### Machine Configuration Option `.machine.install.bootloader`

The `.machine.install.bootloader` option in the machine config is deprecated and will be removed in Talos 1.6.
This was a no-op for a long time: the bootloader is always installed.

### RDMA/RoCE support

Talos no longer loads by default `rdma_rxe` Linux driver, which is required for RoCE support.
If the driver is required, it can be enabled by specifying `rdma_rxe` in the `.machine.kernel.modules` field in the machine config.

### `talosctl images` Command

The command `talosctl images` was renamed to `talosctl image default`.

The backward-compatible alias is kept in Talos 1.5, but it will be dropped in Talos 1.6.
