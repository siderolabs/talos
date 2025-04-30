---
title: "Kernel"
description: "Linux kernel reference."
---

## Commandline Parameters

Talos supports a number of kernel commandline parameters.  Some are required for
it to operate.  Others are optional and useful in certain circumstances.

Several of these are enforced by the Kernel Self Protection Project [KSPP](https://kspp.github.io/Recommended_Settings).

**Required** parameters:

* `talos.platform`: can be one of `akamai`, `aws`, `azure`, `container`, `digitalocean`, `equinixMetal`, `gcp`, `hcloud`, `metal`, `nocloud`, `openstack`, `oracle`, `scaleway`, `upcloud`, `vmware` or `vultr`
* `slab_nomerge`: required by KSPP
* `pti=on`: required by KSPP

**Recommended** parameters:

* `init_on_alloc=1`: advised by KSPP, enabled by default in kernel config
* `init_on_free=1`: advised by KSPP, enabled by default in kernel config

### Available Talos-specific parameters

#### `ip`

Initial configuration of the interface, routes, DNS, NTP servers (multiple `ip=` kernel parameters are accepted).

Full documentation is available in the [Linux kernel docs](https://www.kernel.org/doc/Documentation/filesystems/nfs/nfsroot.txt).

`ip=<client-ip>:<server-ip>:<gw-ip>:<netmask>:<hostname>:<device>:<autoconf>:<dns0-ip>:<dns1-ip>:<ntp0-ip>`

Talos will use the configuration supplied via the kernel parameter as the initial network configuration.
This parameter is useful in the environments where DHCP doesn't provide IP addresses or when default DNS and NTP servers should be overridden
before loading machine configuration.
Partial configuration can be applied as well, e.g. `ip=:::::::<dns0-ip>:<dns1-ip>:<ntp0-ip>` sets only the DNS and NTP servers.

IPv6 addresses can be specified by enclosing them in the square brackets, e.g. `ip=[2001:db8::a]:[2001:db8::b]:[fe80::1]::controlplane1:eth1::[2001:4860:4860::6464]:[2001:4860:4860::64]:[2001:4860:4806::]`.

`<netmask>` can use either an IP address notation (IPv4: `255.255.255.0`, IPv6: `[ffff:ffff:ffff:ffff::0]`), or simply a number of one bits in the netmask (`24`).

`<device>` can be traditional interface naming scheme `eth0, eth1` or `enx<MAC>`, example: `enx78e7d1ea46da`

DHCP can be enabled by setting `<autoconf>` to `dhcp`, example: `ip=:::::eth0.3:dhcp`.
Alternative syntax is `ip=eth0.3:dhcp`.

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
* `bond=bond0:eth0,eth1`
* `bond=bond0:eth0,eth1:balance-rr`

An example of a bond configuration with all options specified:

`bond=bond1:eth3,eth4:mode=802.3ad,xmit_hash_policy=layer2+3:1450`

This will create a bond interface named `bond1` with `eth3` and `eth4` as slaves and set the bond mode to `802.3ad`, the transmit hash policy to `layer2+3` and bond interface MTU to 1450.

#### `vlan`

The interface vlan configuration.

Full documentation is available in the [Dracut kernel docs](https://man7.org/linux/man-pages/man7/dracut.cmdline.7.html).

Talos will use the `vlan=` kernel parameter if supplied to set the initial vlan configuration.
This parameter is useful in environments where the switch ports are VLAN tagged with no native VLAN.

Only one vlan can be configured at this stage.

An example of a vlan configuration including static ip configuration:

`vlan=eth0.100:eth0 ip=172.20.0.2::172.20.0.1:255.255.255.0::eth0.100:::::`

This will create a vlan interface named `eth0.100` with `eth0` as the underlying interface and set the vlan id to 100 with static IP 172.20.0.2/24 and 172.20.0.1 as default gateway.

#### `net.ifnames=0`

Disable the predictable network interface names by specifying `net.ifnames=0` on the kernel command line.

#### `panic`

The amount of time to wait after a panic before a reboot is issued.

Talos will always reboot if it encounters an unrecoverable error.
However, when collecting debug information, it may reboot too quickly for
humans to read the logs.
This option allows the user to delay the reboot to give time to collect debug
information from the console screen.

A value of `0` disables automatic rebooting entirely.

#### `talos.config`

The URL at which the machine configuration data may be found (only for `metal` platform, with the kernel parameter `talos.platform=metal`).

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

##### `metal-iso`

When the kernel parameter `talos.config=metal-iso` is set, Talos will attempt to load the machine configuration from any block device with a filesystem label of `metal-iso`.
Talos will look for a file named `config.yaml` in the root of the filesystem.

For example, such ISO filesystem can be created with:

```sh
mkdir iso/
cp config.yaml iso/
mkisofs -joliet -rock -volid 'metal-iso' -output config.iso iso/
```

#### `talos.config.auth.*`

Kernel parameters prefixed with `talos.config.auth.` are used to configure [OAuth2 authentication for the machine configuration]({{< relref "../advanced/machine-config-oauth" >}}).

#### `talos.config.inline`

The kernel parameter `talos.config.inline` can be used to provide initial minimal machine configuration directly on the kernel command line, when other means of providing the configuration are not available.
The machine configuration should be `zstd` compressed and base64-encoded to be passed as a kernel parameter.

> Note: The kernel command line has a limited size (4096 bytes), so this method is only suitable for small configuration documents.

One such example is to provide [a custom CA certificate]({{<  relref "../talos-guides/configuration/certificate-authorities" >}}) via `TrustedRootsConfig` in the machine configuration:

```shell
cat config.yaml | zstd --compress --ultra -22 | base64 -w 0
```

Please note that configuration from this argument is only loaded if the configuration hasn't been yet saved to `STATE` partition.

#### `talos.platform`

The platform name on which Talos will run.

Valid options are:

* `akamai`
* `aws`
* `azure`
* `container`
* `digitalocean`
* `equinixMetal`
* `gcp`
* `hcloud`
* `metal`
* `nocloud`
* `openstack`
* `oracle`
* `scaleway`
* `upcloud`
* `vmware`
* `vultr`

#### `talos.board`

The board name, if Talos is being used on an ARM64 SBC.

Supported boards are:

* `bananapi_m64`: Banana Pi M64
* `libretech_all_h3_cc_h5`: Libre Computer ALL-H3-CC
* `rock64`: Pine64 Rock64
* ...

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

* `halt`
* `poweroff`

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

* `system` resets system disk.
* `system:EPHEMERAL,STATE` resets ephemeral and state partitions. Doing this reverts Talos into maintenance mode.

#### `talos.auditd.disabled`

By default, Talos runs `auditd` service capturing kernel audit events.
If you set `talos.auditd.disabled=1`, this behavior will be disabled, and you can run your own `auditd` service.

#### `talos.dashboard.disabled`

By default, Talos redirects kernel logs to virtual console `/dev/tty1` and starts the dashboard on `/dev/tty2`,
then switches to the dashboard tty.

If you set `talos.dashboard.disabled=1`, this behavior will be disabled.
Kernel logs will be sent to the currently active console and the dashboard will not be started.

It is set to be `1` by default on SBCs.

#### `talos.environment`

Each value of the argument sets a default environment variable.
The expected format is `key=value`.

Example:

```text
talos.environment=http_proxy=http://proxy.example.com:8080 talos.environment=https_proxy=http://proxy.example.com:8080
```

#### `talos.device.settle_time`

The time in Go duration format to wait for devices to settle before starting the boot process.
By default, Talos waits for `udevd` to scan and settle, but with some RAID controllers `udevd` might
report settled devices before they are actually ready.
Adding this kernel argument provides extra settle time on top of `udevd` settle time.
The maximum value is `10m` (10 minutes).

Example:

```text
talos.device.settle_time=3m
```

#### `talos.halt_if_installed`

If set to `1`, Talos will pause the boot sequence and keeps printing a message until the boot timeout is reached if it detects that it is already installed.
This is useful if booting from ISO/PXE and you want to prevent the machine accidentally booting from the ISO/PXE after installation to the disk.
