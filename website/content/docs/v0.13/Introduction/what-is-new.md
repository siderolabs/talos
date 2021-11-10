---
title: What's New in Talos 0.13
weight: 5
---

### Cluster Discovery and KubeSpan

This release of Talos includes two new closely related
features: [cluster membership discovery](../../guides/discovery/) and [KubeSpan](../../guides/kubespan/).

KubeSpan is a feature of Talos that automates the setup and maintainance of a full mesh [WireGuard](https://www.wireguard.com) network for your cluster, giving you the ablility to operate hybrid Kubernetes clusters that can span the edge, datacenter, and cloud.
Management of keys and discovery of peers can be completely automated for a zero-touch experience that makes it simple and easy to create hybrid clusters.

These new features are not enabled by default, to enable them please make following changes to the machine configuration:

```yaml
machine:
  network:
    kubespan:
      enabled: true
cluster:
  discovery:
    enabled: true
```

### Reboots via `kexec`

Talos now reboots by default via kexec syscall which means BIOS POST process is skipped.
On bare-metal hardware BIOS POST process might take 10-15 minutes, so Talos reboots 10-15 minutes faster on bare-metal.

Kexec support is enabled by default, but it can be disabled with the following change to the machine configuration:

```yaml
machine:
  sysctls:
    kernel.kexec_load_disabled: "1"
```

### Hetzner, Scaleway, Upcloud and Vultr

Talos now natively supports four new cloud platforms:

* [Hetzner](https://www.hetzner.com/), including VIP support
* [Scaleway](https://www.scaleway.com/en/)
* [Upcloud](https://upcloud.com/)
* [Vultr](https://www.vultr.com/)

Also generic `cloud-init` `nocloud` platform is supported in both networking and storage-based modes.

### etcd Advertised Address

The address advertised by etcd can now be controlled with [new machine configuration option](../../reference/configuration/#etcdconfig) `machine.etcd.subnet`.

### kubelet Node IP

The addresses picked by kubelet can now be controlled with [new machine configuration option](../../reference/configuration/#kubeletconfig) `machine.kubelet.nodeIP.validSubnets`.

### Windows Suport

CLI tool talosctl is now built for Windows and published as part of the [release](https://github.com/talos-systems/talos/releases/tag/v0.13.0).

### Component Updates

* Linux: 5.10.69
* Kubernetes: 1.22.2
* containerd: 1.5.6
* runc: 1.0.2

Talos is built with Go 1.17.1.
