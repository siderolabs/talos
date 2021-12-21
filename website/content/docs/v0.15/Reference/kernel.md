---
title: Kernel
desription: Linux kernel reference.
---

## Commandline Parameters

Talos supports a number of kernel commandline parameters.  Some are required for
it to operate.  Others are optional and useful in certain circumstances.

Several of these are enforced by the Kernel Self Protection Project [KSPP](https://kernsec.org/wiki/index.php/Kernel_Self_Protection_Project/Recommended_Settings).

**Required** parameters:

- `talos.config`: the HTTP(S) URL at which the machine configuration data can be found
- `talos.platform`: can be one of `aws`, `azure`, `container`, `digitalocean`, `gcp`, `metal`, `packet`, or `vmware`
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
  Partial configuration can be applied as well, e.g. `ip=<:::::::<dns0-ip>:<dns1-ip>:<ntp0-ip>` sets only the DHCP and DNS servers.
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

#### `talos.platform`

  The platform name on which Talos will run.

  Valid options are:
    - `aws`
    - `azure`
    - `container`
    - `digitalocean`
    - `gcp`
    - `metal`
    - `packet`
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

#### `talos.interface`

  The network interface to use for pre-configuration booting.

  If the node has multiple network interfaces, you may specify which interface
  to use by setting this option.

  Keep in mind that Talos uses indexed interface names (eth0, eth1, etc) and not
  "predictable" interface names (enp2s0) or BIOS-enumerated (eno1) names.

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
