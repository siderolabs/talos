---
title: "Advanced Networking"
description: "How to configure advanced networking options on Talos Linux."
aliases:
  - ../guides/advanced-networking
---

## Static Addressing

Static addressing is comprised of specifying `addresses`, `routes` ( remember to add your default gateway ), and `interface`.
Most likely you'll also want to define the `nameservers` so you have properly functioning DNS.

```yaml
machine:
  network:
    hostname: talos
    nameservers:
      - 10.0.0.1
    interfaces:
      - interface: eth0
        addresses:
          - 10.0.0.201/8
        mtu: 8765
        routes:
          - network: 0.0.0.0/0
            gateway: 10.0.0.1
      - interface: eth1
        ignore: true
  time:
    servers:
      - time.cloudflare.com
```

## Additional Addresses for an Interface

In some environments you may need to set additional addresses on an interface.
In the following example, we set two additional addresses on the loopback interface.

```yaml
machine:
  network:
    interfaces:
      - interface: lo
        addresses:
          - 192.168.0.21/24
          - 10.2.2.2/24
```

## Bonding

The following example shows how to create a bonded interface.

```yaml
machine:
  network:
    interfaces:
      - interface: bond0
        dhcp: true
        bond:
          mode: 802.3ad
          lacpRate: fast
          xmitHashPolicy: layer3+4
          miimon: 100
          updelay: 200
          downdelay: 200
          interfaces:
            - eth0
            - eth1
```

## Setting Up a Bridge

The following example shows how to set up a bridge between two interfaces with an assigned static address.

```yaml
machine:
  network:
    interfaces:
      - interface: br0
        addresses:
          - 192.168.0.42/24
        bridge:
          stp:
            enabled: true
          interfaces:
              - eth0
              - eth1
```

## VLANs

To setup vlans on a specific device use an array of VLANs to add.
The master device may be configured without addressing by setting dhcp to false.

```yaml
machine:
  network:
    interfaces:
      - interface: eth0
        dhcp: false
        vlans:
          - vlanId: 100
            addresses:
              - "192.168.2.10/28"
            routes:
              - network: 0.0.0.0/0
                gateway: 192.168.2.1
```
