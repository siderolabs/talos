---
title: What's New in Talos 1.4
weight: 50
description: "List of new and shiny features in Talos Linux."
---

See also [upgrade notes]({{< relref "../talos-guides/upgrading-talos">}}) for important changes.

## Interactive Dashboard

Talos now starts a text-based [UI dashboard]({{< relref "../talos-guides/interactive-dashboard" >}}) on virtual console `/dev/tty2` and switches to it by default upon boot.
Kernel logs remain available on `/dev/tty1`.

To switch between virtual TTYs, use the `Alt+F1` and `Alt+F2` keys.

You can disable this new feature by setting the kernel parameter `talos.dashboard.disabled=1`.
The dashboard is disabled by default on SBCs to limit resource usage.

The output to the serial console is not affected by this change.

{{< imgproc "interactive-dashboard-2.png" Fit "920x920" >}}
Interactive Dashboard on QEMU VM
{{< /imgproc >}}

## Boot Process

Talos now ships with the latest Linux LTS kernel 6.1.x.

### GRUB Menu Wipe Options

Talos ISO GRUB menu now an includes an option to wipe completely a Talos installed on a system disk.

Talos GRUB menu for a system disk boot now includes an option to wipe `STATE` and `EPHEMERAL` partition returning the
machine to the maintenance mode.

### Kernel Modules

Talos now automatically loads kernel drivers built as modules.
If any system extensions or the Talos base kernel build provides kernel modules and if they matches the system hardware (via PCI IDs), they will be loaded automatically.
Modules can still be loaded explicitly by defining it in [machine configuration](https://www.talos.dev/v1.4/reference/configuration/#kernelconfig).

At the moment only a small subset of device drivers is built as modules, but we plan to expand this list in the future.

### Kernel Modules Tree

Talos now supports re-building the kernel modules dependency tree information on upgrades.
This allows modules of same name to co-exist as in-tree and external modules.
System Extensions can provide modules installed into `extras` directory and when loading it'll take precedence over the in-tree module.

### Kernel Argument `talos.environment`

Talos now supports passing environment variables via `talos.environment` kernel argument.

Example:

```text
talos.environment=http_proxy=http://proxy.example.com:8080 talos.environment=https_proxy=http://proxy.example.com:8080
```

### Kernel Argument `talos.experimental.wipe`

Talos now supports specifying a list of system partitions to be wiped in the `talos.experimental.wipe` kernel argument.

```text
`talos.experimental.wipe=system:EPHEMERAL,STATE`
```

## Networking

### Bond Device Selectors

Bond links can now be described using device selectors instead of explicit device names:

```yaml
machine:
  network:
    interfaces:
      - interface: bond0
        bond:
          deviceSelectors:
            - hardwareAddr: '00:50:56:*'
            - hardwareAddr: '00:50:57:9c:2c:2d'
```

### VLAN Machine Configuration

Strategic merge config patches now correctly support merging `.vlans` sections of the network interface.

## `talosctl` CLI

### `talosctl etcd`

Talos adds new APIs to make it easier to perform etcd maintenance operations.

These APIs are available via new `talosctl etcd` sub-commands:

* `talosctl etcd alarm list|disarm`
* `talosctl etcd defrag`
* `talosctl etcd status`

See also [etcd maintenance guide]({{< relref "../../advanced/etcd-maintenance.md" >}}).

### `talosctl containers`

`talosctl logs -k` and `talosctl containers -k` now support and output container display names with their ids.
This allows to distinguish between containers with the same name.

### `talosctl dashboard`

A dashboard now shows same information as interactive console (see above), but in a remote way over the Talos API:

{{< imgproc "talos-dashboard.png" Fit "920x600" >}}
talosctl dashboard CLI
{{< /imgproc >}}

Previous monitoring screen can be accessed by using `<F2>` key.

### `talosctl logs`

An issue was fixed which might lead to the log output corruption in the CLI under certain conditions.

### `talosctl netstat`

Talos API was extended to support retrieving a list of network connections (sockets) from the node and pods.
`talosctl netstat` command was added to retrieve the list of network connections.

### `talosctl reset`

Talos now supports resetting user disks through the Reset API,
the list of disks to wipe can be passed using the `--user-disks-to-wipe` flag to the `talosctl reset` command.

## Miscellaneous

### Registry Mirror Catch-All Option

Talos now supports a catch-all option for registry mirrors:

```yaml
machine:
    registries:
        mirrors:
            docker.io:
                - https://registry-1.docker.io/
            "*":
                - https://my-registry.example.com/
```

### Talos API `os:operator` role

Talos now supports a new `os:operator` role for the Talos API.
This role allows everything `os:reader` role allows plus access to maintenance APIs:
rebooting, shutting down a node, accessing packet capture, etcd alarm APIs, etcd backup, etc.

### VMware Platform

Talos now supports loading network configuration on VMWare platform from the `metadata` key.
See [CAPV IPAM Support](https://github.com/kubernetes-sigs/cluster-api-provider-vsphere/blob/main/docs/proposal/20220929-ipam-support.md) and
[Talos issue 6708](https://github.com/siderolabs/talos/issues/6708) for details.

## Component Updates

* Linux: 6.1.24
* containerd: v1.6.20
* runc: v1.1.5
* Kubernetes: v1.27.1
* etcd: v3.5.8
* CoreDNS: v1.10.1
* Flannel: v0.21.4

Talos is built with Go 1.20.3.
