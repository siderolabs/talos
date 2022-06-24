---
title: "Kernel"
description: "Linux kernel reference."
---

## Commandline Parameters

Talos supports a number of kernel commandline parameters.  Some are required for
it to operate.  Others are optional and useful in certain circumstances.

Several of these are enforced by the Kernel Self Protection Project [KSPP](https://kernsec.org/wiki/index.php/Kernel_Self_Protection_Project/Recommended_Settings).

**Required** parameters:

- `talos.config`: the HTTP(S) URL at which the machine configuration data can be found
- `talos.platform`: can be one of `aws`, `azure`, `container`, `digitalocean`, `gcp`, `metal`, `equinixMetal`, or `vmware`
- `init_on_alloc=1`: required by KSPP
- `slab_nomerge`: required by KSPP
- `pti=on`: required by KSPP

**Recommended** parameters:

 - `init_on_free=1`: advised by KSPP if minimizing stale data lifetime is
     important

### Available Talos-specific parameters

#### `ip`

  Initial configuration of the interface, routes, DNS, NTP servers.

  Full documentation is available in the [Linux kernel docs](https://www.kernel.org/doc/Documentation/filesystems/nfs/nfsroot.txt).

  `ip=<client-ip>:<server-ip>:<gw-ip>:<netmask>:<hostname>:<device>:<autoconf>:<dns0-ip>:<dns1-ip>:<ntp0-ip>`

  Talos will use the configuration supplied via the kernel parameter as the initial network configuration.
  This parameter is useful in the environments where DHCP doesn't provide IP addresses or when default DNS and NTP servers should be overridden
  before loading machine configuration.
  Partial configuration can be applied as well, e.g. `ip=<:::::::<dns0-ip>:<dns1-ip>:<ntp0-ip>` sets only the DNS and NTP servers.

  IPv6 addresses can be specified by enclosing them in the square brackets, e.g. `ip=[2001:db8::a]:[2001:db8::b]:[fe80::1]::master1:eth1::[2001:4860:4860::6464]:[2001:4860:4860::64]:[2001:4860:4806::]`.

#### `bond`

  Bond interface configuration.

  Full documentation is available in the [Dracut kernel docs](https://man7.org/linux/man-pages/man7/dracut.cmdline.7.html).

  `bond=<bondname>:<bondslaves>:<options>:<mtu>`

  Talos will use the `bond=` kernel parameter if supplied to set the initial bond configuration.
  This parameter is useful in environments where the switch ports are suspended if the machine doesn't setup a LACP bond.

  If only the bond name is supplied, the bond will be created with `eth0` and `eth1` as slaves and bond mode set as `balance-rr`

  All these below configurations are equivalent:

  * `bond=bond0`
  * `bond=bond0:`
  * `bond=bond0::`
  * `bond=bond0:::`
  * `bond=bond0:eth0:eth1`
  * `bond=bond0:eth0:eth1:balance-rr`

  An example of a bond configuration with all options specified:

  `bond=bond1:eth3,eth4:mode=802.3ad,xmit_hash_policy=layer2+3:1450`

  This will create a bond interface named `bond1` with `eth3` and `eth4` as slaves and set the bond mode to `802.3ad`, the transmit hash policy to `layer2+3` and bond interface MTU to 1450.

#### `panic`

  The amount of time to wait after a panic before a reboot is issued.

  Talos will always reboot if it encounters an unrecoverable error.
  However, when collecting debug information, it may reboot too quickly for
  humans to read the logs.
  This option allows the user to delay the reboot to give time to collect debug
  information from the console screen.

  A value of `0` disables automatic rebooting entirely.

#### `talos.config`

  The URL at which the machine configuration data may be found.
  
  This parameter supports variable substitution inside URL query values for the following case-insensitive placeholders:
  
  * `${uuid}` the SMBIOS UUID
  * `${serial}` the SMBIOS Serial Number
  * `${mac}` the MAC address of the first network interface attaining link state `up`
  * `${hostname}` the hostname of the machine
  
  The following example

  `http://example.com/metadata?h=${hostname}&m=${mac}&s=${serial}&u=${uuid}`
  
  may translate to
  
  `http://example.com/metadata?h=myTestHostname&m=52%3A2f%3Afd%3Adf%3Afc%3Ac0&s=0OCZJ19N65&u=40dcbd19-3b10-444e-bfff-aaee44a51fda`
  
  For backwards compatibility we insert the system UUID into the query parameter `uuid` if its value is empty. As in

  `http://example.com/metadata?uuid=` => `http://example.com/metadata?uuid=40dcbd19-3b10-444e-bfff-aaee44a51fda`

#### `talos.platform`

  The platform name on which Talos will run.

  Valid options are:
    - `aws`
    - `azure`
    - `container`
    - `digitalocean`
    - `gcp`
    - `metal`
    - `equinixMetal`
    - `vmware`

#### `talos.board`

  The board name, if Talos is being used on an ARM64 SBC.

  Supported boards are:
    - `bananapi_m64`: Banana Pi M64
    - `libretech_all_h3_cc_h5`: Libre Computer ALL-H3-CC
    - `rock64`: Pine64 Rock64
    - `rpi_4`: Raspberry Pi 4, Model B

#### `talos.hostname`

  The hostname to be used.
  The hostname is generally specified in the machine config.
  However, in some cases, the DHCP server needs to know the hostname
  before the machine configuration has been acquired.

  Unless specifically required, the machine configuration should be used
  instead.

#### `talos.shutdown`

  The type of shutdown to use when Talos is told to shutdown.

  Valid options are:
    - `halt`
    - `poweroff`

#### `talos.network.interface.ignore`

  A network interface which should be ignored and not configured by Talos.

  Before a configuration is applied (early on each boot), Talos attempts to
  configure each network interface by DHCP.
  If there are many network interfaces on the machine which have link but no
  DHCP server, this can add significant boot delays.

  This option may be specified multiple times for multiple network interfaces.

#### `talos.experimental.wipe`

  Resets the disk before starting up the system.

  Valid options are:
    - `system` resets system disk.
