---
title: networkd
---

Networkd handles all of the host level network configuration.
Configuration is defined under the `networking` key.

By default, we attempt to issue a DHCP request for every interface on the server.
This can be overridden by supplying one of the following kernel arguments:

- `talos.network.interface.ignore` - specify a list of interfaces to skip discovery on
- `ip` - `ip=<client-ip>:<server-ip>:<gw-ip>:<netmask>:<hostname>:<device>:<autoconf>:<dns0-ip>:<dns1-ip>:<ntp0-ip>` as documented in the [kernel here](https://www.kernel.org/doc/Documentation/filesystems/nfs/nfsroot.txt)
  - ex, `ip=10.0.0.99:::255.0.0.0:control-1:eth0:off:10.0.0.1`

## Examples

Documentation for the network section components can be found under the configuration reference.

### Static Addressing

Static addressing is comprised of specifying `cidr`, `routes` ( remember to add your default gateway ), and `interface`.
Most likely you'll also want to define the `nameservers` so you have properly functioning DNS.

```yaml
machine:
  network:
    hostname: talos
    nameservers:
    - 10.0.0.1
    time:
      servers:
        - time.cloudflare.com
    interfaces:
    - interface: eth0
      cidr: 10.0.0.201/8
      mtu: 8765
      routes:
        - network: 0.0.0.0/0
          gateway: 10.0.0.1
    - interface: eth1
      ignore: true
```

### Additional Addresses for an Interface

In some environments you may need to set additional addresses on an interface.
In the following example, we set two additional addresses on the loopback interface.

```yaml
machine:
  network:
    interfaces:
    - interface: lo0
      cidr: 192.168.0.21/24
    - interface: lo0
      cidr: 10.2.2.2/24


```

### Bonding

The following example shows how to create a bonded interface.

```yaml
machine:
  network:
    interfaces:
    - interface: bond0
      dhcp: true
      bond:
        mode: 802.3ad
        lacprate: fast
        hashpolicy: layer3+4
        miimon: 100
        updelay: 200
        downdelay: 200
        interfaces:
        - eth0
        - eth1
```

### VLANs

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
        cidr: "192.168.2.10/28"
        routes:
        - network: 0.0.0.0/0
          gateway: 192.168.2.1
```
