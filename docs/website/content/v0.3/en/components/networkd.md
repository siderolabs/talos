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
