---
title: "Ethernet Configuration"
description: "How to configure Ethernet network link settings."
---

Talos Linux allows you to configure Ethernet network link settings, such as ring configuration or disabling TCP checksum offloading.
The settings and their values closely follow `ethtool` command line options, so you can use similar recipes.

## Observing Current Status

You can observe current Ethernet settings in the `EthernetStatus` resource:

```yaml
# talosctl get ethernetstatus enp0s2 -o yaml
spec:
    rings:
        rx-max: 256
        tx-max: 256
        rx: 256
        tx: 256
        tx-push: false
        rx-push: false
    features:
        tx-scatter-gather: on
        tx-checksum-ipv4: off [fixed]
        tx-checksum-ip-generic: on
        tx-checksum-ipv6: off [fixed]
        highdma: on [fixed]
        tx-scatter-gather-fraglist: off [fixed]
        tx-vlan-hw-insert: off [fixed]
        rx-vlan-hw-parse: off [fixed]
        rx-vlan-filter: on [fixed]
        vlan-challenged: off [fixed]
        tx-generic-segmentation: on
        rx-gro: on
        rx-lro: off [fixed]
        tx-tcp-segmentation: on
        tx-gso-robust: on [fixed]
        tx-tcp-ecn-segmentation: on
        tx-tcp-mangleid-segmentation: off
        tx-tcp6-segmentation: on
        tx-fcoe-segmentation: off [fixed]
        tx-gre-segmentation: off [fixed]
        tx-gre-csum-segmentation: off [fixed]
        tx-ipxip4-segmentation: off [fixed]
        tx-ipxip6-segmentation: off [fixed]
        tx-udp_tnl-segmentation: off [fixed]
        tx-udp_tnl-csum-segmentation: off [fixed]
        tx-gso-partial: off [fixed]
        tx-tunnel-remcsum-segmentation: off [fixed]
        tx-sctp-segmentation: off [fixed]
        tx-esp-segmentation: off [fixed]
        tx-udp-segmentation: off
        tx-gso-list: off [fixed]
        tx-checksum-fcoe-crc: off [fixed]
        tx-checksum-sctp: off [fixed]
        rx-ntuple-filter: off [fixed]
        rx-hashing: off [fixed]
        rx-checksum: on [fixed]
        tx-nocache-copy: off
        loopback: off [fixed]
        rx-fcs: off [fixed]
        rx-all: off [fixed]
        tx-vlan-stag-hw-insert: off [fixed]
        rx-vlan-stag-hw-parse: off [fixed]
        rx-vlan-stag-filter: off [fixed]
        l2-fwd-offload: off [fixed]
        hw-tc-offload: off [fixed]
        esp-hw-offload: off [fixed]
        esp-tx-csum-hw-offload: off [fixed]
        rx-udp_tunnel-port-offload: off [fixed]
        tls-hw-tx-offload: off [fixed]
        tls-hw-rx-offload: off [fixed]
        rx-gro-hw: on
        tls-hw-record: off [fixed]
        rx-gro-list: off
        macsec-hw-offload: off [fixed]
        rx-udp-gro-forwarding: off
        hsr-tag-ins-offload: off [fixed]
        hsr-tag-rm-offload: off [fixed]
        hsr-fwd-offload: off [fixed]
        hsr-dup-offload: off [fixed]
    channels:
        combined-max: 1
        combined: 1
```

The available features depend on the network card and driver.
Some values are fixed by the driver and hardware and cannot be changed.

## Configuration

Use the [EthernetConfig]({{< relref "../../reference/configuration/network/ethernetconfig" >}}) document to configure Ethernet settings.
You can append the machine config document to the machine configuration (separating with `---`), or apply it as a [machine configuration patch]({{< relref "../configuration/patching" >}}).

For example, to disable TCP segmentation on transmit:

```yaml
apiVersion: v1alpha1
kind: EthernetConfig
name: enp0s2
features:
  tx-tcp-segmentation: false
```

For rings and channels configuration, values can be increased if they do not exceed the maximum supported by the network card (the maximum values are reported in the status with `-max` suffix).
