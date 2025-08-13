---
title: "Network Configuration"
description: "In this guide we will describe how network can be configured on bare-metal platforms."
---

By default, Talos will run DHCP client on all interfaces which have a link, and that might be enough for most of the cases.
If some advanced network configuration is required, it can be done via the [machine configuration]({{< relref "../../../reference/configuration" >}}) file.

But sometimes it is required to apply network configuration even before the machine configuration can be fetched from the network.

## Kernel Command Line

Talos supports some kernel command line parameters to configure network before the machine configuration is fetched.

> Note: Kernel command line parameters are not persisted after Talos installation, so proper network configuration should be done via the machine configuration.

Address, default gateway and DNS servers can be configured via `ip=` kernel command line parameter:

```text
ip=172.20.0.2::172.20.0.1:255.255.255.0::eth0.100:::::
```

Bonding can be configured via `bond=` kernel command line parameter:

```text
bond=bond0:eth0,eth1:balance-rr
```

VLANs can be configured via `vlan=` kernel command line parameter:

```text
vlan=eth0.100:eth0
```

See [kernel parameters reference]({{< relref "../../../reference/kernel" >}}) for more details.

## Platform Network Configuration

Some platforms (e.g. AWS, Google Cloud, etc.) have their own network configuration mechanisms, which can be used to perform the initial network configuration.
There is no such mechanism for bare-metal platforms, so Talos provides a way to use platform network config on the `metal` platform to submit the initial network configuration.

The platform network configuration is a YAML document which contains resource specifications for various network resources.
For the `metal` platform, the [interactive dashboard]({{< relref "../../interactive-dashboard" >}}) can be used to edit the platform network configuration, also the configuration can be
created [manually]({{< relref "../../../advanced/metal-network-configuration" >}}).

The current value of the platform network configuration can be retrieved using the `MetaKeys` resource (key `0x0a`):

```bash
talosctl get meta 0x0a
```

The platform network configuration can be updated using the `talosctl meta` command for the running node:

```bash
talosctl meta write 0x0a '{"externalIPs": ["1.2.3.4"]}'
talosctl meta delete 0x0a
```

The initial platform network configuration for the `metal` platform can be also included into the generated Talos image:

```bash
docker run --rm -i ghcr.io/siderolabs/imager:{{< release >}} iso --arch amd64 --tar-to-stdout --meta 0x0a='{...}' | tar xz
docker run --rm -i --privileged ghcr.io/siderolabs/imager:{{< release >}} image --platform metal --arch amd64 --tar-to-stdout --meta 0x0a='{...}' | tar xz
```

The platform network configuration gets merged with other sources of network configuration, the details can be found in the [network resources guide]({{< relref "../../../learn-more/networking-resources.md#configuration-merging" >}}).

## `nocloud` Network Configuration

Some bare-metal providers provide a way to configure network via the `nocloud` data source.
Talos Linux can automatically pick up this [configuration]({{< relref "../cloud-platforms/nocloud" >}}) when `nocloud` image is used.
