---
title: "Network Device Selector"
description: "How to configure network devices by selecting them using hardware information"
aliases:
  - ../../guides/device-selector
---

## Configuring Network Device Using Device Selector

`deviceSelector` is an alternative method of configuring a network device:

```yaml
machine:
  ...
  network:
    interfaces:
      - deviceSelector:
          driver: virtio_net
          hardwareAddr: "00:00:*"
        address: 192.168.88.21
```

Selector has the following traits:

- qualifiers match a device by reading the hardware information in `/sys/class/net/...`
- qualifiers are applied using logical `AND`
- `machine.network.interfaces.deviceConfig` option is mutually exclusive with `machine.network.interfaces.interface`
- if the selector matches multiple devices, the controller will apply config to all of them

The available hardware information used in the selector can be observed in the `LinkStatus` resource (works in maintenance mode):

```yaml
# talosctl get links eth0 -o yaml
spec:
  ...
  hardwareAddr: 4e:95:8e:8f:e4:47
  permanentAddr: 4e:95:8e:8f:e4:47
  busPath: 0000:06:00.0
  driver: alx
  pciID: 1969:E0B1
```

The following qualifiers are available:

- `driver` - matches a device by its driver name
- `hardwareAddr` - matches a device by its hardware address
- `permanentAddr` - matches a device by its permanent hardware address
- `busPath` - matches a device by its PCI bus path
- `pciID` - matches a device by its PCI vendor and device ID
- `physical` - matches only physical devices (vs. virtual devices, e.g. bonds and VLANs)

All qualifiers except for `physical` support wildcard matching using `*` character.

## Using Device Selector for Bonding

Device selectors can be used to configure bonded interfaces:

```yaml
machine:
  ...
  network:
    interfaces:
      - interface: bond0
        bond:
          mode: balance-rr
          deviceSelectors:
            - permanentAddr: '00:50:56:8e:8f:e4'
            - permanentAddr: '00:50:57:9c:2c:2d'
```

In this example, the `bond0` interface will be created and bonded using two devices with the specified hardware addresses.
For bonding, use `permanentAddr` instead of `hardwareAddr` to match the permanent hardware address of the device, as `hardwareAddr` might change
as the link becomes part of the bond.
