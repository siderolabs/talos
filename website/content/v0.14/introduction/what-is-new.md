---
title: What's New in Talos 0.14
weight: 5
---

### Kubelet

Kubelet configuration can be updated without node restart (`.machine.kubelet` section of machine configuration) with commands
`talosctl edit mc --immediate`, `talosctl apply-config --immediate`, `talosctl patch mc --immediate`.

Kubelet service can now be restarted with `talosctl service kubelet restart`.

Kubelet node IP configuration (`.machine.kubelet.nodeIP.validSubnets`) can now include negative subnet matches (prefixed with `!`).

### Kubernetes Upgrade Enhancements

`talosctl upgrade-k8s` was improved to:

* sync all boostrap manifest resources in the Kubernetes cluster with versions bundled with current version Talos
* upgrade `kubelet` to the version of the control plane components (without node reboot)

So there is no need to update CoreDNS, Flannel container manually after running `upgrade-k8s` anymore.

### Log Shipping

Talos can now [ship system logs](../../guides/logging/)
to the configured destination using either JSON-over-UDP or JSON-over-TCP:
see `.machine.logging` machine configuration option.

### NTP Sync

Talos NTP sync process was improved to align better with kernel time adjustment periods and to filter out spikes.

### `talosctl support`

`talosctl` CLI tool now has a new subcommand `support` that gathers all
cluster information that could help with debugging in.

Output of the command is a `zip` archive with all Talos service logs, Kubernetes pod logs and manifests,
Talos resources manifests and so on.
Generated archive does not contain any secret information, so it is safe to send it for analysis to a third party.

### Component Updates

* Linux: 5.15.6
* etcd: 3.5.1
* containerd: 1.5.8
* runc: 1.0.3
* Kubernetes: 1.23.1
* CoreDNS: 1.8.6
* Flannel (default CNI): 0.15.1

Talos is built with Go 1.17.5

### Cluster Discovery

[Cluster Discovery](../../guides/discovery/) is enabled by default for Talos 0.14.
Cluster Discovery can be disabled with `talosctl gen config --with-cluster-discovery=false`.

## Kexec and capabilities

When kexec support is disabled
Talos no longer drops Linux capabilities (`CAP_SYS_BOOT` and `CAP_SYS_MODULES`) for child processes.
That is helpful for advanced use-cases like Docker-in-Docker.

If you want to permanently disable kexec and capabilities dropping, pass `kexec_load_disabled=1` argument to the kernel.

For example:

```yaml
install:
  extraKernelArgs:
    - sysctl.kernel.kexec_load_disabled=1
```

Please note that capabilities are dropped before machine configuration is loaded,
so disabling kexec via `machine.sysctls` will not be enough.

### `installer` and `imager` images

Talos supports two target architectures: `amd64` and `arm64`, so all Talos images are built for both `amd64` and `arm64`.

New image `imager` was added which contains Talos assets for both architectures which allows to generate Talos disk images
cross-arch: e.g. generate Talos Raspberry PI disk image on `amd64` machine.

As `installer` image is used only to do initial install and upgrades, it now contains Talos assets for a specific architecture.
This reduces size of the `installer` image leading to faster upgrades and less memory usage.

There are no user-visible changes except that now `imager` container image should be used to produce Talos disk images.

### SideroLink

A set of Talos ehancements is going to unlock a number of exciting features in the upcoming release of [Sidero](https://www.sidero.dev/):

* `SideroLink`: a point-to-point Wireguard tunnel connecting Talos node back to the provisioning platform (Sidero).
* event sink (kernel arg `talos.event.sink=http://10.0.0.1:4000`) delivers Talos internal events to the specified destination.
* kmsg log delivery (kernel arg `talos.logging.kernel=tcp://10.0.0.1:4001`) sends kernel logs as JSON lines over TCP or UDP.

### VLAN Enhancements

Talos now supports setting MTU and Virtual IPs on VLAN interfaces.
