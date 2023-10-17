---
title: "Predictable Interface Names"
description: "How to use predictable interface naming."
---

Starting with version Talos 1.5, network interfaces are renamed to [predictable names](https://www.freedesktop.org/wiki/Software/systemd/PredictableNetworkInterfaceNames/)
same way as `systemd` does that in other Linux distributions.

The naming schema `enx78e7d1ea46da` (based on MAC addresses) is enabled by default, the order of interface naming decisions is:

* firmware/BIOS provided index numbers for on-board devices (example: `eno1`)
* firmware/BIOS provided PCI Express hotplug slot index numbers (example: `ens1`)
* physical/geographical location of the connector of the hardware (example: `enp2s0`)
* interfaces's MAC address (example: `enx78e7d1ea46da`)

The predictable network interface names features can be disabled by specifying `net.ifnames=0` in the kernel command line.

>Note: Talos automatically adds the `net.ifnames=0` kernel argument when upgrading from Talos versions before 1.5, so upgrades to 1.5 don't require any manual intervention.

"Cloud" platforms, like AWS, still use old `eth0` naming scheme as Talos automatically adds `net.ifnames=0` to the kernel command line.

## Single Network Interface

When running Talos on a machine with a single network interface, predictable interface names might be confusing, as it might come up as `enxSOMETHING` which is hard to address.
There are two ways to solve this:

* disable the feature by supplying `net.ifnames=0` to the initial boot of Talos, Talos will persist `net.ifnames=0` over installs/upgrades.
* use [device selectors]({{< relref "./device-selector" >}}):

  ```yaml
  machine:
    network:
      interfaces:
        - deviceSelector:
            busPath: "0*" # should select any hardware network device, if you have just one, it will be selected
          # any configuration can follow, e.g:
          addresses: [10.3.4.5/24]
  ```
