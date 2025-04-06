---
title: What's New in Talos 1.9.0
weight: 50
description: "List of new and shiny features in Talos Linux."
---

See also [upgrade notes]({{< relref "../talos-guides/upgrading-talos">}}) for important changes.

## Important Changes

Please read this section carefully before upgrading to Talos 1.9.0.

### Direct Rendering Manager (DRM)

Starting with Talos 1.9, the `i915` and `amdgpu` DRM drivers have been removed from the Talos base image.
These drivers, along with their firmware, are now included in two new system extensions named `i915` and `amdgpu`.
The previously available extensions `i915-ucode` and `amdgpu-firmware` have been retired.

Upgrades via Image Factory or Omni will automatically include the new extensions if the `i915-ucode` or `amdgpu-firmware` extensions were previously used.

### udevd

Talos previously used `eudev` to provide `udevd`, now it uses `systemd-udevd` instead.

The `systemd-udevd` might change the names of network interfaces with predictable names, potentially causing issues with existing configurations.

## Image Cache

Talos now supports providing a local [Image Cache]({{< relref "../../talos-guides/configuration/image-cache.md" >}}) for container images.

The Image Cache feature can be used to avoid downloading the required images over the network, which can be useful in air-gapped or weak connectivity environments.

## Networking

### Custom DNS Search Domains

Talos now allows to supports specifying custom search domains for Talos nodes using
new machine configuration field [`.machine.network.searchDomains`]({{< relref "../../reference/configuration/v1alpha1/config.md#Config.machine.network" >}}).

For the host the `/etc/resolve.conf` would look like:

```text
nameserver 127.0.0.53

search my-custom-search-name.com my-custom-search-name2.com
```

For the pods it will look something like this:

```text
search default.svc.cluster.local svc.cluster.local cluster.local my-custom-search-name.com my-custom-search-name2.com
nameserver 10.96.0.10
options ndots:5
```

### Device Selectors

Talos now supports matching on [permanent hardware (MAC) address]({{< relref "../../reference/configuration/v1alpha1/config.md#Config.machine.network.interfaces..bond.deviceSelectors." >}}) of the network interfaces.
This is specifically useful to match bond members, as they change their hardware addresses when they become part of the bond.

### Node Address Ordering

Talos supports new experimental address sort algorithm for `NodeAddress` which are used to pick up default addresses for `kubelet`, `etcd`, etc.

It can be enabled with the following config patch:

```yaml
machine:
  features:
    nodeAddressSortAlgorithm: v2
```

The new algorithm prefers more specific prefixes, which is specifically useful for IPv6 addresses.

## Control Groups Analysis

The `talosctl cgroups` command has been added to the `talosctl` tool.
This command allows you to view the [cgroup resource consumption and limits]({{< relref "../../advanced/cgroups-analysis.md" >}}) for a machine, e.g.
`talosctl cgroups --preset memory`.

## Kubernetes

### APIServer Authorization Config

Starting with Talos 1.9, `.cluster.apiServer.authorizationConfig` field supports setting [Kubernetes API server authorization modes](https://kubernetes.io/docs/reference/access-authn-authz/authorization/#using-configuration-file-for-authorization)
using the `--authorization-config` flag.

The machine config field supports a list of `authorizers`.
For instance:

```yaml
cluster:
  apiServer:
    authorizationConfig:
      - type: Node
        name: Node
      - type: RBAC
        name: rbac
```

For new cluster if the Kubernetes API server supports the `--authorization-config` flag, it'll be used by default instead of the `--authorization-mode` flag.
By default Talos will always add the `Node` and `RBAC` authorizers to the list.

When upgrading if either a user-provided `authorization-mode` or `authorization-webhook-*` flag is set via `.cluster.apiServer.extraArgs`, it'll be used instead of the new `AuthorizationConfig`.

Current authorization config can be viewed by running: `talosctl get authorizationconfigs.kubernetes.talos.dev -o yaml`.

### User Namespaces

Talos Linux now supports running Kubernetes pods with user namespaces enabled.
Please refer to the [documentation]({{< relref "../../kubernetes-guides/configuration/usernamespace.md" >}}) for more information.

## Containers

### OCI Base Runtime Spec

Talos now allows to [modify the OCI base runtime spec for the container runtime]({{< relref "../../advanced/oci-base-spec.md" >}}).

### Registry Mirrors

In versions before Talos 1.9, there was a discrepancy between the way Talos itself and CRI plugin resolves registry mirrors:
Talos will never fall back to the default registry if endpoints are configured, while CRI plugin will.

> Note: Talos Linux pulls images for the `installer`, `kubelet`, `etcd`, while all workload images are pulled by the CRI plugin.

In Talos 1.9 this was fixed, so that by default an upstream registry is used as a fallback in all cases, while new registry mirror
[configuration option]({{< relref "../../reference/configuration/v1alpha1/config.md#Config.machine.registries.mirrors.-" >}}) `.skipFallback` can be used to disable this behavior both for Talos and CRI plugin.

## Miscellaneous

### `auditd`

Talos Linux now starts an `auditd` service by default.
Linux kernel audit logs can be fetched with `talosctl logs auditd`.

### `talosctl disks`

The command `talosctl disks` was removed, please use `talosctl get disks`, `talosctl get systemdisk`, and `talosctl get blockdevices` instead.

### `talosctl wipe`

The new command `talosctl wipe disk` allows to wipe a disk or a partition which is not used as a volume.

## New Platforms

### Turing RK1

Talos now supports the [Turning RK1]({{< relref "../../talos-guides/install/single-board-computers/turing_rk1.md" >}}) SOM.

### `nocloud`

On bare-metal, Talos Linux was tested to correctly parse `nocloud` configuration from the following providers:

* [phoenixNAP Bare Metal Cloud](https://phoenixnap.com/)
* [servers.com](https://www.servers.com/)

## Deprecations

### cgroups version 1

Support for `cgroupsv1` is deprecated, and will be removed in Talos 1.10 (for non-container mode).

## Component Updates

* Linux: 6.12.5
* containerd: 2.0.1
* Flannel: 0.26.1
* Kubernetes: 1.32.0
* runc: 1.2.3
* CoreDNS: 1.12.0

Talos is built with Go 1.23.4.

## Contributors

Thanks to the following contributors who made this release possible:

* adilTepe
* Adolfo Ochagavía
* Alessio Moiso
* Andrey Smirnov
* blablu
* Dan Rue
* David Backeus
* Devin Buhl
* Dmitriy Matrenichev
* Dmitry Sharshakov
* Eddie Wang
* egrosdou01
* ekarlso
* Florian Ströger
* Hexoplon
* Jakob Maležič
* Jasmin
* Jean-Francois Roy
* Joakim Nohlgård
* Justin Garrison
* KBAegis
* Mike Beaumont
* Mohammad Amin Mokhtari
* naed3r
* Nebula
* nevermarine
* Nico Berlee
* Noel Georgi
* OliviaBarrington
* Philip Schmid
* Philipp Kleber
* Rémi Paulmier
* Remko Molier
* Robby Ciliberto
* Roman Ivanov
* Ryan Borstelmann
* Sam Stelfox
* Serge Logvinov
* Sergey Melnik
* Skyler Mäntysaari
* solidDoWant
* sophia-coldren
* Spencer Smith
* SpiReCZ
* Steven Cassamajor
* Steven Kreitzer
* Tim Jones
* Utku Ozdemir
* Variant9
