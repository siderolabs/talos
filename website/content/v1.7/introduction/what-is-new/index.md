---
title: What's New in Talos 1.7.0
weight: 50
description: "List of new and shiny features in Talos Linux."
---

See also [upgrade notes]({{< relref "../../talos-guides/upgrading-talos/">}}) for important changes.

## Important Changes

* The [default NTP server](#time-sync) was updated to `time.cloudflare.com` instead of `pool.ntp.org` (only if not specified in the machine configuration).
* Talos Linux [now](#iptables) forces `kubelet` and `kube-proxy` to use `iptables-nft` instead of `iptables-legacy`.
* SBC (Single Board Computers) images are no longer part of the Talos release assets, please read [SBC](#sbc) before upgrading.
* Talos clusters created with `talosctl cluster create` in Docker mode now use a [random port](#containers-docker) for Kubernetes and Talos API.

## Security

### CA Rotation

Talos Linux now supports [rotating the root CA certificate and key]({{< relref "../../advanced/ca-rotation" >}}) for Talos API and Kubernetes API.

## Networking

### Device Selectors

Talos Linux now supports `physical: true` qualifier for [device selectors]({{< relref "../../talos-guides/network/device-selector" >}}), it selects non-virtual network interfaces (i.e. `en0` is selected, while `bond0` is not).

### DNS Caching

Talos Linux now provides a [caching DNS resolver]({{< relref "../../talos-guides/network/host-dns" >}}) for host workloads (including host networking pods).
Host DNS resolver is enabled by default for clusters created with Talos 1.7.

### Time Sync

Default [NTP server]({{< relref "../../talos-guides/configuration/time-sync" >}}) was updated to be `time.cloudflare.com` instead of `pool.ntp.org`.
Default server is only used if the user does not specify any NTP servers in the configuration.

Talos Linux can now sync to PTP devices (e.g. provided by the hypervisor) skipping the network time servers.
In order to activate PTP sync, set `machine.time.servers` to the PTP device name (e.g. `/dev/ptp0`):

```yaml
machine:
  time:
    servers:
      - /dev/ptp0
```

### SideroLink HTTP Proxy

[SideroLink]({{< relref "../../talos-guides/network/siderolink" >}}) connections can now proxy Wireguard UDP packet over existing HTTP/2 SideroLink API connection (for networks where UDP protocol is filtered, but HTTP is allowed).

## Kubernetes

### API Server Service Account Key

Talos Linux 1.7.0 when generating machine configuration uses RSA key for Kubernetes API Server Service Account instead of ECDSA key to provide better compatibility with external OpenID Connect implementations.

### IPTables

Talos Linux now forces `kubelet` and `kube-proxy` to use `iptables-nft` instead of `iptables-legacy` (`xtables`) which was the default
before Talos 1.7.0.

Container images based on `iptables-wrapper` should work without changes, but if there was a direct call to `legacy` mode of `iptables`, make sure
to update to use `iptables-nft`.

## Platforms

### New Supported Platforms

Talos Linux now supports:

* [OpenNebula](https://opennebula.io/) platform ([Talos platform `opennebula`]({{< relref "../../talos-guides/install/virtualized-platforms/opennebula" >}}))
* [Akamai Connected Cloud](https://www.linode.com/) provider ([Talos platform `akamai`]({{< relref "../../talos-guides/install/cloud-platforms/akamai" >}}))

### Containers (`docker`)

The `talosctl cluster create` command now can create [multiple Talos clusters on the same machine]({{< relref "../../talos-guides/install/local-platforms/docker" >}}).
The Kubernetes and Talos APIs are mapped to a random port on the host machine.

Talos Linux now uses provided DNS resolver when running inside a container.

### Talos-in-Kubernetes

Talos Linux now supports running Talos inside [Kubernetes as a pod]({{< relref "../../talos-guides/install/cloud-platforms/kubernetes" >}}): e.g. to run controlplane nodes inside existing Kubernetes cluster.

## SBC

Talos has split the SBC's (Single Board Computers) into separate repositories.
There will not be any more SBC specific release assets as part of Talos release.

The default Talos `installer` image will stop working for SBC's and will fail the upgrade, if used, starting from Talos v1.7.0.

The SBC's images and installers can be generated on the fly using [Image Factory](https://factory.talos.dev) or using [imager]({{< relref "../../talos-guides/install/boot-assets">}}) for custom images.
The list of official SBC's images supported by Image Factory can be found in the [overlays](https://github.com/siderolabs/overlays/) repository.

In order to upgrade an SBC running Talos 1.6 to Talos 1.7, generate an `installer` image with an SBC overlay and use it to upgrade the cluster.

## System Extensions

### Extension Services Configuration

Talos now supports supplying configuration files and environment variables for extension services.
The extension service configuration is a separate config document.
An example is shown below:

```yaml
---
apiVersion: v1alpha1
kind: ExtensionServiceConfig
name: nut-client
configFiles:
  - content: MONITOR ${upsmonHost} 1 remote pass password
    mountPath: /usr/local/etc/nut/upsmon.conf
environment:
  - UPS_NAME=ups
```

For documentation, see [Extension Services Config Files]({{< relref "../../reference/configuration/extensions/extensionserviceconfig" >}}).

> **Note**: The use of `environmentFile` in extension service [spec]({{< relref "../../advanced/extension-services">}}) is now deprecated and will be removed in a future release of Talos,
> use `ExtensionServiceConfig` instead.

### New Extensions

Talos Linux in version v1.7 introduces new [extensions](https://github.com/siderolabs/exensions):

* `kata-containers`
* `spin`
* `v4l-uvc-drivers`
* `vmtoolsd-guest-agent`
* `wasmedge`
* `xen-guest-agent`

## Logging

### Additional Tags

Talos Linux now supports setting [extra tags]({{< relref "../../talos-guides/configuration/logging" >}}) when sending logs in JSON format:

```yaml
machine:
  logging:
    destinations:
      - endpoint: "udp://127.0.0.1:12345/"
        format: "json_lines"
        extraTags:
          server: s03-rack07
```

### Syslog

Talos Linux now starts a basic syslog receiver listening on `/dev/log`.
The receiver can mostly parse both RFC3164 and RFC5424 messages and writes them as JSON formatted message.
The logs can be viewed via `talosctl logs syslogd`.

This is mostly implemented for extension services that log to syslog.

## Miscellaneous

### Kubernetes Upgrade

The [command]({{< relref "../../kubernetes-guides/upgrading-kubernetes" >}}) `talosctl upgrade-k8s` now supports specifying custom image references for Kubernetes components via `--*-image` flags.
The default behavior is unchanged, and the flags are optional.

### KubeSpan

Talos Linux disables by default a [KubeSpan]({{< relref "../../talos-guides/network/kubespan" >}}) feature to harvest additional endpoints from KubeSpan members.
This feature turned out to be less helpful than expected and caused unnecessary performance issues.

Previous behavior can be restored with:

```yaml
machine:
  network:
    kubespan:
        harvestExtraEndpoints: true
```

### Secure Boot ISO

Talos Linux now provides a way to configure systemd-boot ISO 'secure-boot-enroll' option while [generating]({{< relref "../../talos-guides/install/boot-assets" >}}) a SecureBoot ISO image:

```yaml
output:
    kind: iso
    isoOptions:
        sdBootEnrollKeys: force # default is still if-safe
    outFormat: raw
```

### Hardware Watchdog Timers

Talos Linux now supports [hardware watchdog timers]({{< relref "../../advanced/watchdog" >}}) configuration.
If enabled, and the machine becomes unresponsive, the hardware watchdog will reset the machine.

The watchdog can be enabled with the following [configuration document]({{< relref "../../reference/configuration/runtime/watchdogtimerconfig" >}}):

```yaml
apiVersion: v1alpha1
kind: WatchdogTimerConfig
device: /dev/watchdog0
timeout: 3m0s
```

## Component Updates

* Linux: 6.6.26
* etcd: 3.5.11
* Kubernetes: 1.30.0
* containerd: 1.7.15
* runc: 1.1.12
* Flannel: 0.24.4

Talos is built with Go 1.22.2.
