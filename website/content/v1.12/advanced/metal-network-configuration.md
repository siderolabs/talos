---
title: "Metal Network Configuration"
description: "How to use `META`-based network configuration on Talos `metal` platform."
---

> Note: This is an advanced feature which requires deep understanding of Talos and Linux network configuration.

Talos Linux when running on a cloud platform (e.g. AWS or Azure), uses the platform-provided metadata server to provide initial network configuration to the node.
When running on bare-metal, there is no metadata server, so there are several options to provide initial network configuration (before machine configuration is acquired):

- use automatic network configuration via DHCP (Talos default)
- use initial boot [kernel command line parameters]({{< relref "../reference/kernel" >}}) to configure networking
- use automatic network configuration via DHCP just enough to fetch machine configuration and then use machine configuration to set desired advanced configuration.

If DHCP option is available, it is by far the easiest way to configure networking.
The initial boot kernel command line parameters are not very flexible, and they are not persisted after initial Talos installation.

Talos starting with version 1.4.0 offers a new option to configure networking on bare-metal: `META`-based network configuration.

> Note: `META`-based network configuration is only available on Talos Linux `metal` platform.

Talos [dashboard]({{< relref "../talos-guides/interactive-dashboard" >}}) provides a way to configure `META`-based network configuration for a machine using the console, but
it doesn't support all kinds of network configuration.

## Network Configuration Format

Talos `META`-based network configuration is a YAML file with the following format:

```yaml
addresses:
    - address: 147.75.61.43/31
      linkName: bond0
      family: inet4
      scope: global
      flags: permanent
      layer: platform
    - address: 2604:1380:45f2:6c00::1/127
      linkName: bond0
      family: inet6
      scope: global
      flags: permanent
      layer: platform
    - address: 10.68.182.1/31
      linkName: bond0
      family: inet4
      scope: global
      flags: permanent
      layer: platform
links:
    - name: eth0
      up: true
      masterName: bond0
      slaveIndex: 0
      layer: platform
    - name: eth1
      up: true
      masterName: bond0
      slaveIndex: 1
      layer: platform
    - name: bond0
      logical: true
      up: true
      mtu: 0
      kind: bond
      type: ether
      bondMaster:
        mode: 802.3ad
        xmitHashPolicy: layer3+4
        lacpRate: slow
        arpValidate: none
        arpAllTargets: any
        primaryReselect: always
        failOverMac: 0
        miimon: 100
        updelay: 200
        downdelay: 200
        resendIgmp: 1
        lpInterval: 1
        packetsPerSlave: 1
        numPeerNotif: 1
        tlbLogicalLb: 1
        adActorSysPrio: 65535
      layer: platform
routes:
    - family: inet4
      gateway: 147.75.61.42
      outLinkName: bond0
      table: main
      priority: 1024
      scope: global
      type: unicast
      protocol: static
      layer: platform
    - family: inet6
      gateway: '2604:1380:45f2:6c00::'
      outLinkName: bond0
      table: main
      priority: 2048
      scope: global
      type: unicast
      protocol: static
      layer: platform
    - family: inet4
      dst: 10.0.0.0/8
      gateway: 10.68.182.0
      outLinkName: bond0
      table: main
      scope: global
      type: unicast
      protocol: static
      layer: platform
hostnames:
    - hostname: ci-blue-worker-amd64-2
      layer: platform
resolvers: []
timeServers: []
```

Every section is optional, so you can configure only the parts you need.
The format of each section matches the respective network [`*Spec` resource]({{< relref "../learn-more/networking-resources" >}}) `.spec` part, e.g the `addresses:`
section matches the `.spec` of `AddressSpec` resource:

```yaml
# talosctl get addressspecs bond0/10.68.182.1/31 -o yaml | yq .spec
address: 10.68.182.1/31
linkName: bond0
family: inet4
scope: global
flags: permanent
layer: platform
```

So one way to prepare the network configuration file is to boot Talos Linux, apply necessary network configuration using Talos machine configuration, and grab the resulting
resources from the running Talos instance.

In this guide we will briefly cover the most common examples of the network configuration.

### Addresses

The addresses configured are usually routable IP addresses assigned to the machine, so
the `scope:` should be set to `global` and `flags:` to `permanent`.
Additionally, `family:` should be set to either `inet4` or `inet6` depending on the address family.

The `linkName:` property should match the name of the link the address is assigned to, it might be a physical link,
e.g. `en9sp0`, or the name of a logical link, e.g. `bond0`, created in the `links:` section.

Example, IPv4 address:

```yaml
addresses:
    - address: 147.75.61.43/31
      linkName: bond0
      family: inet4
      scope: global
      flags: permanent
      layer: platform
```

Example, IPv6 address:

```yaml
addresses:
    - address: 2604:1380:45f2:6c00::1/127
      linkName: bond0
      family: inet6
      scope: global
      flags: permanent
      layer: platform
```

### Links

For physical network interfaces (links), the most usual configuration is to bring the link up:

```yaml
links:
    - name: en9sp0
      up: true
      layer: platform
```

This will bring the link up, and it will also disable Talos auto-configuration (disables running DHCP on the link).

Another common case is to set a custom MTU:

```yaml
links:
    - name: en9sp0
      up: true
      mtu: 9000
      layer: platform
```

The order of the links in the `links:` section is not important.

#### Bonds

For bonded links, there should be a link resource for the bond itself, and a link resource for each enslaved link:

```yaml
links:
    - name: bond0
      logical: true
      up: true
      kind: bond
      type: ether
      bondMaster:
        mode: 802.3ad
        xmitHashPolicy: layer3+4
        lacpRate: slow
        arpValidate: none
        arpAllTargets: any
        primaryReselect: always
        failOverMac: 0
        miimon: 100
        updelay: 200
        downdelay: 200
        resendIgmp: 1
        lpInterval: 1
        packetsPerSlave: 1
        numPeerNotif: 1
        tlbLogicalLb: 1
        adActorSysPrio: 65535
      layer: platform
    - name: eth0
      up: true
      masterName: bond0
      slaveIndex: 0
      layer: platform
    - name: eth1
      up: true
      masterName: bond0
      slaveIndex: 1
      layer: platform
```

The name of the bond can be anything supported by Linux kernel, but the following properties are important:

- `logical: true` - this is a logical link, not a physical one
- `kind: bond` - this is a bonded link
- `type: ether` - this is an Ethernet link
- `bondMaster:` - defines bond configuration, please see Linux documentation on the available options

For each enslaved link, the following properties are important:

- `masterName: bond0` - the name of the bond this link is enslaved to
- `slaveIndex: 0` - the index of the enslaved link, starting from 0, controls the order of bond slaves

#### VLANs

VLANs are logical links which have a parent link, and a VLAN ID and protocol:

```yaml
links:
    - name: bond0.35
      logical: true
      up: true
      kind: vlan
      type: ether
      parentName: bond0
      vlan:
        vlanID: 35
        vlanProtocol: 802.1ad
```

The name of the VLAN link can be anything supported by Linux kernel, but the following properties are important:

- `logical: true` - this is a logical link, not a physical one
- `kind: vlan` - this is a VLAN link
- `type: ether` - this is an Ethernet link
- `parentName: bond0` - the name of the parent link
- `vlan:` - defines VLAN configuration: `vlanID` and `vlanProtocol`

### Routes

For route configuration, most of the time `table: main`, `scope: global`, `type: unicast` and `protocol: static` are used.

The route most important fields are:

- `dst:` defines the destination network, if left empty means "default gateway"
- `gateway:` defines the gateway address
- `priority:` defines the route priority (metric), lower values are preferred for the same `dst:` network
- `outLinkName:` defines the name of the link the route is associated with
- `src:` sets the source address for the route (optional)

Additionally, `family:` should be set to either `inet4` or `inet6` depending on the address family.

Example, IPv6 default gateway:

```yaml
routes:
    - family: inet6
      gateway: '2604:1380:45f2:6c00::'
      outLinkName: bond0
      table: main
      priority: 2048
      scope: global
      type: unicast
      protocol: static
      layer: platform
```

Example, IPv4 route to `10/8` via `10.68.182.0` gateway:

```yaml
routes:
    - family: inet4
      dst: 10.0.0.0/8
      gateway: 10.68.182.0
      outLinkName: bond0
      table: main
      scope: global
      type: unicast
      protocol: static
      layer: platform
```

### Hostnames

Even though the section supports multiple hostnames, only a single one should be used:

```yaml
hostnames:
    - hostname: host
      domainname: some.org
      layer: platform
```

The `domainname:` is optional.

If the hostname is not set, Talos will use default generated hostname.

### Resolvers

The `resolvers:` section is used to configure DNS resolvers, only single entry should be used:

```yaml
resolvers:
    - dnsServers:
        - 8.8.8.8
        - 1.1.1.1
      layer: platform
```

If the `dnsServers:` is not set, Talos will use default DNS servers.

### Time Servers

The `timeServers:` section is used to configure NTP time servers, only single entry should be used:

```yaml
timeServers:
    - timeServers:
        - 169.254.169.254
      layer: platform
```

If the `timeServers:` is not set, Talos will use default NTP servers.

## Supplying `META` Network Configuration

Once the network configuration YAML document is ready, it can be supplied to Talos in one of the following ways:

- for a running Talos machine, using Talos API (requires already established network connectivity)
- for Talos disk images, it can be embedded into the image
- for ISO/PXE boot methods, it can be supplied via kernel command line parameters as an environment variable

The metal network configuration is stored in Talos `META` partition under the key `0xa` (decimal 10).

In this guide we will assume that the prepared network configuration is stored in the file `network.yaml`.

> Note: as JSON is a subset of YAML, the network configuration can be also supplied as a JSON document.

### Supplying Network Configuration to a Running Talos Machine

Use the `talosctl` to write a network configuration to a running Talos machine:

```bash
talosctl meta write 0xa "$(cat network.yaml)"
```

### Supplying Network Configuration to a Talos Disk Image

Following the [boot assets]({{< relref "../talos-guides/install/boot-assets" >}}) guide, create a disk image passing the network configuration as a `--meta` flag:

```bash
docker run --rm -t -v $PWD/_out:/out -v /dev:/dev --privileged ghcr.io/siderolabs/imager:{{< release >}} metal --meta "0xa=$(cat network.yaml)"
```

### Supplying Network Configuration to a Talos ISO/PXE Boot

As there is no `META` partition created yet before Talos Linux is installed, `META` values can be set as an environment variable `INSTALLER_META_BASE64` passed to the initial boot of Talos.
The supplied value will be used immediately, and also it will be written to the `META` partition once Talos is installed.

When using `imager` to create the ISO, the `INSTALLER_META_BASE64` environment variable will be automatically generated from the `--meta` flag:

```bash
$ docker run --rm -t -v $PWD/_out:/out ghcr.io/siderolabs/imager:{{< release >}} iso --meta "0xa=$(cat network.yaml)"
...
kernel command line: ... talos.environment=INSTALLER_META_BASE64=MHhhPWZvbw==
```

When PXE booting, the value of `INSTALLER_META_BASE64` should be set manually:

```bash
echo -n "0xa=$(cat network.yaml)" | gzip -9 | base64
```

The resulting base64 string should be passed as an environment variable `INSTALLER_META_BASE64` to the initial boot of Talos: `talos.environment=INSTALLER_META_BASE64=<base64-encoded value>`.

### Getting Current `META` Network Configuration

Talos exports `META` keys as resources:

```yaml
# talosctl get meta 0x0a -o yaml
...
spec:
    value: '{"addresses": ...}'
```
