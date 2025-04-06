---
title: What's New in Talos 1.6.0
weight: 50
description: "List of new and shiny features in Talos Linux."
---

See also [upgrade notes]({{< relref "../talos-guides/upgrading-talos">}}) for important changes.

## Breaking Changes

### Linux Firmware

Starting with Talos 1.6, Linux firmware is not included in the default initramfs.

Users that need Linux firmware can pull them as an extension during install time using the [Image Factory]({{< relref "../../learn-more/image-factory.md" >}}) service.
If the initial boot requires firmware, a [custom ISO can be built]({{< relref "../../talos-guides/install/boot-assets.md" >}}) with the firmware included using the Image Factory service or using the `imager`.
This also ensures that the linux-firmware is not tied to a specific Talos version.

The list of firmware packages which were removed from the default `initramfs` and are now available as extensions:

* [bnx2 and bnx2x firmware (Broadcom NetXtreme II)](https://github.com/siderolabs/extensions/tree/main/firmware/bnx2-bnx2x)
* [Intel ICE firmware (Intel(R) Ethernet Controller 800 Series)](https://github.com/siderolabs/extensions/tree/main/firmware/intel-ice-firmware)

### Network Device Selectors

Previously, [network device selectors]({{< relref "../../talos-guides/network/device-selector.md" >}}) only matched the first link, now the configuration is applied to all matching links.

### `talosctl images` command

The command `images` deprecated in Talos 1.5 was removed, please use `talosctl images default` instead.

### `.persist` Machine Configuration Option

The option `.persist` deprecated in Talos 1.5 was removed, the machine configuration is always persisted.

## New Features

### Kubernetes `n-5` Version Support

Talos Linux starting with version 1.6 supports the latest Kubernetes `n-5` versions, for release 1.6.0 this means [support]({{< relref "../support-matrix.md" >}}) for Kubernetes versions 1.24-1.29.
This allows users to make it easier to upgrade to new Talos Linux versions without having to upgrade Kubernetes at the same time.

> See [Kubernetes release support](https://kubernetes.io/releases/) for the list of supported versions by Kubernetes project.

### OAuth2 Machine Config Flow

Talos Linux when running on the `metal` platform can be configured to authenticate the machine configuration download using [OAuth2 device flow]({{< relref "../../advanced/machine-config-oauth.md" >}}).

### Ingress Firewall

Talos Linux now supports configuring the [ingress firewall rules]({{< relref "../../talos-guides/network/ingress-firewall.md" >}}).

## Improvements

### Component Updates

* Linux: 6.1.67
* Kubernetes: 1.29.0
* containerd: 1.7.10
* runc: 1.1.10
* etcd: 3.5.11
* CoreDNS: 1.11.1
* Flannel: 0.23.0

Talos is built with Go 1.21.5.

### Extension Services

Talos now starts Extension Services early in the boot process, this allows guest agents packaged as extension services to be started in maintenance mode.

### Flannel Configuration

Talos Linux now supports customizing default Flannel manifest with extra arguments for `flanneld`:

```yaml
cluster:
  network:
    cni:
      flannel:
        extraArgs:
          - --iface-can-reach=192.168.1.1
```

### Kernel Arguments

Talos and Imager now supports dropping kernel arguments specified in `.machine.install.extraKernelArgs` or as `--extra-kernel-arg` to `imager`.
Any kernel argument that starts with a `-` is dropped.
Kernel arguments to be dropped can be specified either as `-<key>` which would remove all arguments that start with `<key>` or as `-<key>=<value>` which would remove the exact argument.

For example, `console=ttyS0` can be dropped by specifying `-console=ttyS0` as an extra argument.

### `kube-scheduler` Configuration

Talos now supports specifying the `kube-scheduler` configuration in the Talos configuration file.
It can be set under `cluster.scheduler.config` and `kube-scheduler` will be automatically configured to with the correct flags.

### Kubernetes Node Taint Configuration

Similar to `machine.nodeLabels` Talos Linux now provides `machine.nodeTaints` machine configuration field to configure Kubernetes `Node` taints.

### Kubelet Credential Provider Configuration

Talos now supports specifying the kubelet credential provider configuration in the Talos configuration file.
It can be set under `machine.kubelet.credentialProviderConfig` and kubelet will be automatically configured to with the correct flags.
The credential binaries are expected to be present under `/usr/local/lib/kubelet/credentialproviders`.
Talos System Extensions can be used to install the credential binaries.

### KubePrism

[KubePrism]({{< relref "../../kubernetes-guides/configuration/kubeprism.md" >}}) is enabled by default on port 7445.

### Sysctl

Talos now handles sysctl/sysfs key names in line with sysctl.conf(5):

* if the first separator is '/', no conversion is done
* if the first separator is '.', dots and slashes are remapped

Example (both sysctls are equivalent):

```yaml
machine:
  sysctls:
    net/ipv6/conf/eth0.100/disable_ipv6: "1"
    net.ipv6.conf.eth0/100.disable_ipv6: "1"
```

### User Disks

Talos Linux now supports specifying user disks in `.machine.disks` machine configuration links via `udev` symlinks, e.g. `/dev/disk/by-id/XXXX`.

### Packet Capture

Talos Linux provides more performant implementation server-side for the packet capture API (`talosctl pcap` CLI).

### Memory Usage and Performance

Talos Linux core components now use less memory and start faster.
