---
title: "Networking Resources"
weight: 10
---

Starting with version 0.11, a new implementation of the network configuration subsystem is powered by [COSI](../controllers-resources/).
The new implementation is still using the same machine configuration file format and external sources to configure a node's network, so there should be no difference
in the way Talos works in 0.11.

The most notable change in Talos 0.11 is that all changes to machine configuration `.machine.network` can be applied now in immediate mode (without a reboot) via
`talosctl edit mc --mode=no-reboot` or `talosctl apply-config --mode=no-reboot`.

## Resources

There are six basic network configuration items in Talos:

* `Address` (IP address assigned to the interface/link);
* `Route` (route to a destination);
* `Link` (network interface/link configuration);
* `Resolver` (list of DNS servers);
* `Hostname` (node hostname and domainname);
* `TimeServer` (list of NTP servers).

Each network configuration item has two counterparts:

* `*Status` (e.g. `LinkStatus`) describes the current state of the system (Linux kernel state);
* `*Spec` (e.g. `LinkSpec`) defines the desired configuration.

| Resource           | Status                 | Spec                 |
|--------------------|------------------------|----------------------|
| `Address`          | `AddressStatus`        | `AddressSpec`        |
| `Route`            | `RouteStatus`          | `RouteSpec`          |
| `Link`             | `LinkStatus`           | `LinkSpec`           |
| `Resolver`         | `ResolverStatus`       | `ResolverSpec`       |
| `Hostname`         | `HostnameStatus`       | `HostnameSpec`       |
| `TimeServer`       | `TimeServerStatus`     | `TimeServerSpec`     |

Status resources have aliases with the `Status` suffix removed, so for example
`AddressStatus` is also available as `Address`.

Talos networking controllers reconcile the state so that `*Status` equals the desired `*Spec`.

## Observing State

The current network configuration state can be observed by querying `*Status` resources via
`talosctl`:

```sh
$ talosctl get addresses
NODE         NAMESPACE   TYPE            ID                                       VERSION   ADDRESS                        LINK
172.20.0.2   network     AddressStatus   eth0/172.20.0.2/24                       1         172.20.0.2/24                  eth0
172.20.0.2   network     AddressStatus   eth0/fe80::9804:17ff:fe9d:3058/64        2         fe80::9804:17ff:fe9d:3058/64   eth0
172.20.0.2   network     AddressStatus   flannel.1/10.244.4.0/32                  1         10.244.4.0/32                  flannel.1
172.20.0.2   network     AddressStatus   flannel.1/fe80::10b5:44ff:fe62:6fb8/64   2         fe80::10b5:44ff:fe62:6fb8/64   flannel.1
172.20.0.2   network     AddressStatus   lo/127.0.0.1/8                           1         127.0.0.1/8                    lo
172.20.0.2   network     AddressStatus   lo/::1/128                               1         ::1/128                        lo
```

In the output there are addresses set up by Talos (e.g. `eth0/172.20.0.2/24`) and
addresses set up by other facilities (e.g. `flannel.1/10.244.4.0/32` set up by CNI).

Talos networking controllers watch the kernel state and update resources
accordingly.

Additional details about the address can be accessed via the YAML output:

```sh
$ talosctl get address eth0/172.20.0.2/24 -o yaml
node: 172.20.0.2
metadata:
    namespace: network
    type: AddressStatuses.net.talos.dev
    id: eth0/172.20.0.2/24
    version: 1
    owner: network.AddressStatusController
    phase: running
    created: 2021-06-29T20:23:18Z
    updated: 2021-06-29T20:23:18Z
spec:
    address: 172.20.0.2/24
    local: 172.20.0.2
    broadcast: 172.20.0.255
    linkIndex: 4
    linkName: eth0
    family: inet4
    scope: global
    flags: permanent
```

Resources can be watched for changes with the `--watch` flag to see how configuration changes over time.

Other networking status resources can be inspected with `talosctl get routes`, `talosctl get links`, etc.
For example:

```sh
$ talosctl get resolvers
NODE         NAMESPACE   TYPE             ID          VERSION   RESOLVERS
172.20.0.2   network     ResolverStatus   resolvers   2         ["8.8.8.8","1.1.1.1"]
```

## Inspecting Configuration

The desired networking configuration is combined from multiple sources and presented
as `*Spec` resources:

```sh
$ talosctl get addressspecs
NODE         NAMESPACE   TYPE          ID                   VERSION
172.20.0.2   network     AddressSpec   eth0/172.20.0.2/24   2
172.20.0.2   network     AddressSpec   lo/127.0.0.1/8       2
172.20.0.2   network     AddressSpec   lo/::1/128           2
```

These `AddressSpecs` are applied to the Linux kernel to reach the desired state.
If, for example, an `AddressSpec` is removed, the address is removed from the Linux network interface as well.

`*Spec` resources can't be manipulated directly, they are generated automatically by Talos
from multiple configuration sources (see a section below for details).

If a `*Spec` resource is queried in YAML format, some additional information is available:

```sh
$ talosctl get addressspecs eth0/172.20.0.2/24 -o yaml
node: 172.20.0.2
metadata:
    namespace: network
    type: AddressSpecs.net.talos.dev
    id: eth0/172.20.0.2/24
    version: 2
    owner: network.AddressMergeController
    phase: running
    created: 2021-06-29T20:23:18Z
    updated: 2021-06-29T20:23:18Z
    finalizers:
        - network.AddressSpecController
spec:
    address: 172.20.0.2/24
    linkName: eth0
    family: inet4
    scope: global
    flags: permanent
    layer: operator
```

An important field is the `layer` field, which describes a configuration layer this spec is coming from: in this case, it's generated by a network operator (see below) and is set by the DHCPv4 operator.

## Configuration Merging

Spec resources described in the previous section show the final merged configuration state,
while initial specs are put to a different unmerged namespace `network-config`.
Spec resources in the `network-config` namespace are merged with conflict resolution to produce the final merged representation in the `network` namespace.

Let's take `HostnameSpec` as an example.
The final merged representation is:

```sh
$ talosctl get hostnamespec -o yaml
node: 172.20.0.2
metadata:
    namespace: network
    type: HostnameSpecs.net.talos.dev
    id: hostname
    version: 2
    owner: network.HostnameMergeController
    phase: running
    created: 2021-06-29T20:23:18Z
    updated: 2021-06-29T20:23:18Z
    finalizers:
        - network.HostnameSpecController
spec:
    hostname: talos-default-master-1
    domainname: ""
    layer: operator
```

We can see that the final configuration for the hostname is `talos-default-master-1`.
And this is the hostname that was actually applied.
This can be verified by querying a `HostnameStatus` resource:

```sh
$ talosctl get hostnamestatus
NODE         NAMESPACE   TYPE             ID         VERSION   HOSTNAME                 DOMAINNAME
172.20.0.2   network     HostnameStatus   hostname   1         talos-default-master-1
```

Initial configuration for the hostname in the `network-config` namespace is:

```sh
$ talosctl get hostnamespec -o yaml --namespace network-config
node: 172.20.0.2
metadata:
    namespace: network-config
    type: HostnameSpecs.net.talos.dev
    id: default/hostname
    version: 2
    owner: network.HostnameConfigController
    phase: running
    created: 2021-06-29T20:23:18Z
    updated: 2021-06-29T20:23:18Z
spec:
    hostname: talos-172-20-0-2
    domainname: ""
    layer: default
---
node: 172.20.0.2
metadata:
    namespace: network-config
    type: HostnameSpecs.net.talos.dev
    id: dhcp4/eth0/hostname
    version: 1
    owner: network.OperatorSpecController
    phase: running
    created: 2021-06-29T20:23:18Z
    updated: 2021-06-29T20:23:18Z
spec:
    hostname: talos-default-master-1
    domainname: ""
    layer: operator
```

We can see that there are two specs for the hostname:

* one from the `default` configuration layer which defines the hostname as `talos-172-20-0-2` (default driven by the default node address);
* another one from the layer `operator` that defines the hostname as `talos-default-master-1` (DHCP).

Talos merges these two specs into a final `HostnameSpec` based on the configuration layer and merge rules.
Here is the order of precedence from low to high:

* `default` (defaults provided by Talos);
* `cmdline` (from the kernel command line);
* `platform` (driven by the cloud provider);
* `operator` (various dynamic configuration options: DHCP, Virtual IP, etc);
* `configuration` (derived from the machine configuration).

So in our example the `operator` layer `HostnameSpec` overwrites the `default` layer producing the final hostname `talos-default-master-1`.

The merge process applies to all six core networking specs.
For each spec, the `layer` controls the merge behavior
If multiple configuration specs
appear at the same layer, they can be merged together if possible, otherwise merge result
is stable but not defined (e.g. if DHCP on multiple interfaces provides two different hostnames for the node).

`LinkSpecs` are merged across layers, so for example, machine configuration for the interface MTU overrides an MTU set by the DHCP server.

## Network Operators

Network operators provide dynamic network configuration which can change over time as the node is running:

* DHCPv4
* DHCPv6
* Virtual IP

Network operators produce specs for addresses, routes, links, etc., which are then merged and applied according to the rules described above.

Operators are configured with `OperatorSpec` resources which describe when operators
should run and additional configuration for the operator:

```sh
$ talosctl get operatorspecs -o yaml
node: 172.20.0.2
metadata:
    namespace: network
    type: OperatorSpecs.net.talos.dev
    id: dhcp4/eth0
    version: 1
    owner: network.OperatorConfigController
    phase: running
    created: 2021-06-29T20:23:18Z
    updated: 2021-06-29T20:23:18Z
spec:
    operator: dhcp4
    linkName: eth0
    requireUp: true
    dhcp4:
        routeMetric: 1024
```

`OperatorSpec` resources are generated by Talos based on machine configuration mostly.
DHCP4 operator is created automatically for all physical network links which are not configured explicitly via the kernel command line or the machine configuration.
This also means that on the first boot, without a machine configuration, a DHCP request is made on all physical network interfaces by default.

Specs generated by operators are prefixed with the operator ID (`dhcp4/eth0` in the example above) in the unmerged `network-config` namespace:

```sh
$ talosctl -n 172.20.0.2 get addressspecs --namespace network-config
NODE         NAMESPACE        TYPE          ID                              VERSION
172.20.0.2   network-config   AddressSpec   dhcp4/eth0/eth0/172.20.0.2/24   1
```

## Other Network Resources

There are some additional resources describing the network subsystem state.

The `NodeAddress` resource presents node addresses excluding link-local and loopback addresses:

```sh
$ talosctl get nodeaddresses
NODE          NAMESPACE   TYPE          ID             VERSION   ADDRESSES
10.100.2.23   network     NodeAddress   accumulative   6         ["10.100.2.23","147.75.98.173","147.75.195.143","192.168.95.64","2604:1380:1:ca00::17"]
10.100.2.23   network     NodeAddress   current        5         ["10.100.2.23","147.75.98.173","192.168.95.64","2604:1380:1:ca00::17"]
10.100.2.23   network     NodeAddress   default        1         ["10.100.2.23"]
```

* `default` is the node default address;
* `current` is the set of addresses a node currently has;
* `accumulative` is the set of addresses a node had over time (it might include virtual IPs which are not owned by the node at the moment).

`NodeAddress` resources are used to pick up the default address for `etcd` peer URL, to populate SANs field in the generated certificates, etc.

Another important resource is `Nodename` which provides `Node` name in Kubernetes:

```sh
$ talosctl get nodename
NODE          NAMESPACE      TYPE       ID         VERSION   NODENAME
10.100.2.23   controlplane   Nodename   nodename   1         infra-green-cp-mmf7v
```

Depending on the machine configuration `nodename` might be just a hostname or the FQDN of the node.

`NetworkStatus` aggregates the current state of the network configuration:

```sh
$ talosctl get networkstatus -o yaml
node: 10.100.2.23
metadata:
    namespace: network
    type: NetworkStatuses.net.talos.dev
    id: status
    version: 5
    owner: network.StatusController
    phase: running
    created: 2021-06-24T18:56:00Z
    updated: 2021-06-24T18:56:02Z
spec:
    addressReady: true
    connectivityReady: true
    hostnameReady: true
    etcFilesReady: true
```

## Network Controllers

For each of the six basic resource types, there are several controllers:

* `*StatusController` populates `*Status` resources observing the Linux kernel state.
* `*ConfigController` produces the initial unmerged `*Spec` resources in the `network-config` namespace based on defaults, kernel command line, and machine configuration.
* `*MergeController` merges `*Spec` resources into the final representation in the `network` namespace.
* `*SpecController` applies merged `*Spec` resources to the kernel state.

For the network operators:

* `OperatorConfigController` produces `OperatorSpec` resources based on machine configuration and deafauls.
* `OperatorSpecController` runs network operators watching `OperatorSpec` resources and producing various `*Spec` resources in the `network-config` namespace.

## Configuration Sources

There are several configuration sources for the network configuration, which are described in this section.

### Defaults

* `lo` interface is assigned addresses `127.0.0.1/8` and `::1/128`;
* hostname is set to the `talos-<IP>` where `IP` is the default node address;
* resolvers are set to `8.8.8.8`, `1.1.1.1`;
* time servers are set to `pool.ntp.org`;
* DHCP4 operator is run on any physical interface which is not configured explicitly.

### Cmdline

The kernel command line is parsed for the following options:

* `ip=` option is parsed for node IP, default gateway, hostname, DNS servers, NTP servers;
* `talos.hostname=` option is used to set node hostname;
* `talos.network.interface.ignore=` can be used to make Talos skip network interface configuration completely.

### Platform

Platform configuration delivers cloud environment-specific options (e.g. the hostname).

### Operator

Network operators provide configuration for all basic resource types.

### Machine Configuration

The machine configuration is parsed for link configuration, addresses, routes, hostname,
resolvers and time servers.
Any changes to `.machine.network` configuration can be applied in immediate mode.

## Network Configuration Debugging

Most of the network controller operations and failures are logged to the kernel console,
additional logs with `debug` level are available with `talosctl logs controller-runtime` command.
If the network configuration can't be established and the API is not available, `debug` level
logs can be sent to the console with `debug: true` option in the machine configuration.
