---
title: What's New in Talos 1.8.0
weight: 50
description: "List of new and shiny features in Talos Linux."
---

See also [upgrade notes]({{< relref "../../talos-guides/upgrading-talos/">}}) for important changes.

## Important Changes

### Release Artifacts

Starting with Talos v1.8.0, only standard assets would be published as github release assets.
These include:

* `cloud-images.json`
* `talosctl` binaries
* `kernel`
* `initramfs`
* `metal` iso and disk images
* `talosctl-cni-bundle`

All other release assets can be downloaded from [Image Factory]({{< relref "../../talos-guides/install/boot-assets#image-factory" >}}).

### Serial Console for `metal` Platform

Starting from Talos 1.8, the `console=ttyS0` kernel argument is no longer included by default in the metal images and installer.
If you are running Talos virtualized in QEMU (e.g., Proxmox), you can add this as an extra kernel argument if needed.
You can refer to the [Image Factory or Imager documentation]({{< relref "../../talos-guides/install/boot-assets" >}}) for instructions on how to do this.
This change addresses issues such as slow boot or lack of console output on bare metal hardware without a serial console.

### Accessing `/dev/net/tun` in Kubernetes Pods

Talos Linux includes `runc` 1.2, which [no longer](https://github.com/opencontainers/runc/pull/3468) exposes `/dev/net/tun` devices by default in containers.
If you require access to `/dev/net/tun` in your Kubernetes pods (such as when running Tailscale as a pod), you can use [device plugins]({{< relref "../../kubernetes-guides/configuration/device-plugins" >}}) to expose `/dev/net/tun` to the pod.

## Disk Management

The disk management backend has been rewritten to support more complex configurations, but the existing configuration should continue to work as before.

The detailed information about the new disk management subsystem can be found in the [disk management guide]({{< relref "../../talos-guides/configuration/disk-management" >}}).

### `EPHEMERAL` Volume

Talos Linux introduces support for configuring the `EPHEMERAL` volume (`/var`): location (disk), minimum and maximum size, etc.
You can find more information about the configuration in the [disk management guide]({{< relref "../../talos-guides/configuration/disk-management#machine-configuration" >}}).

### Upgrades

In Talos Linux installer, the system disk is never wiped during upgrades.
This means that the `--preserve` flag is now automatically set for `talosctl upgrade` command.

## Kubernetes

### Slim Kubelet Image

Starting from Kubernetes 1.31.0, the `kubelet` container image has been optimized to include fewer utilities.
This change was made as the in-tree CSI plugins were removed in Kubernetes 1.31.0.
The reduction in utilities results in a smaller image size and reduces the potential attack surface.

For Kubernetes versions prior to 1.31.0, two images will be built: the default "fat" image (`v1.x.y`) and a slim image (`v1.x.y-slim`).

For Kubernetes versions 1.31.0 and later, the default tag will point to the slim image, while the "fat" image will be tagged as `v1.x.y-fat`.

### Node Annotations

Talos Linux now supports configuring Kubernetes node annotations via machine configuration (`.machine.nodeAnnotations`) in a way similar to node labels.

### CNI Plugins

Talos Linux now bundles by default the following standard CNI plugins (required by default Flannel installation):

* `bridge`
* `firewall`
* `flannel`
* `host-local`
* `loopback`
* `portmap`

The Talos bundled Flannel manifest was simplified to remove the `install-cni` step.

> Note: Custom CNI plugins can be still copied over to the `/opt/cni/bin` directory using init containers as before.

### Default Node Labels

Talos Linux now includes a default label `node.kubernetes.io/exclude-from-external-load-balancers` for control plane nodes during configuration generation.

### `kube-proxy` Backend

Talos Linux configures kube-proxy >= v1.31.0 to use 'nftables' backend by default.

### Talos Extensions as Kubernetes Node Labels/Annotations

Talos Linux now includes the list of installed extensions as Kubernetes node labels or annotations.

The key format for the labels is `extensions.talos.dev/<name>`, and the value represents the version of the extension.
If the extension name is not a valid label key, it will be skipped.
If the extension version is a valid label value, it will be added as a label; otherwise, it will be added as an annotation.

For Talos machines booted from the Image Factory artifacts, the schematic ID will be published as the annotation `extensions.talos.dev/schematic` since it exceeds the maximum length of 63 characters for label keys.

### DNS Forwarding for CoreDNS pods

Use of the host DNS resolver as the upstream for Kubernetes CoreDNS pods is now enabled by default in new clusters.

To disable this feature, you can use the following configuration:

```yaml
machine:
    features:
        hostDNS:
            enabled: true
            forwardKubeDNSToHost: false
```

Please note that for running clusters, you will need to kill the CoreDNS pods for this change to take effect.

The IP address used for forwarding DNS queries has been changed to the fixed address `169.254.116.108`.
If you are upgrading from Talos 1.7 with `forwardKubeDNSToHost` enabled, you can clean up the old Kubernetes service by running `kubectl delete -n kube-system service host-dns`.

## Hardware Support

### PCI Devices

A list of PCI devices can now be obtained via `PCIDevices` resource, e.g. `talosctl get pcidevices`.

### NVIDIA GPU Support

Starting from Talos 1.8.0, SideroLabs will include extensions for both LTS and Production versions of NVIDIA extensions.

The NVIDIA drivers and the container toolkits now ships an LTS and Production version as per [NVIDIA driver lifecycle](https://docs.nvidia.com/datacenter/tesla/drivers/index.html#lifecycle).

The new extensions names are

* nvidia-container-toolkit-production
* nvidia-container-toolkit-lts
* nvidia-open-gpu-kernel-modules-production
* nvidia-open-gpu-kernel-modules-lts
* nonfree-kmod-nvidia-lts
* nonfree-kmod-nvidia-production

For Talos 1.8, the `-lts` variant follows `535.x` and the `-production` variant follows `550.x` upstream driver versions.

If you are upgrading and already have a schematic ID from the Image Factory, the LTS version of the NVIDIA extension will be retained.

### Device Extra Settle Timeout

Talos Linux now supports a kernel command line argument `talos.device.settle_time=3m` to set the device extra settle timeout to workaround issues with broken drivers.

## Security

### Workload Apparmor Profile

Talos Linux can now apply the default AppArmor profiles to all workloads started via `containerd`, if the machine is installed with the AppArmor LSM enabled in the kernel args (`security=apparmor`).

### Secure Boot

Talos Linux now can optionally include well-known UEFI (Microsoft) SecureBoot keys into the auto-enrollment UEFI database.

### Custom Trusted Roots

Talos Linux now supports adding [custom trusted roots]({{< relref "../../talos-guides/configuration/certificate-authorities" >}}) (CA certificates) via
a [`TrustedRootsConfig`]({{< relref "../../reference/configuration/security/trustedrootsconfig" >}}) configuration document.

## Networking

### Bridge

Talos Linux now support configuring [`vlan_filtering`]({{< relref "../../reference/configuration/v1alpha1/config#Config.machine.network.interfaces..bridge.vlan" >}}) for bridge interfaces.

### KubeSpan

Extra announced endpoints can be added using the [`KubespanEndpointsConfig` document]({{< relref "../../talos-guides/network/kubespan#configuration" >}}).

## Machine Configuration

### Machine Configuration via Kernel Command Line

Talos Linux supports supplying zstd-compressed, base64-encoded machine configuration small documents via the [kernel command line parameter]({{< relref "../../reference/kernel" >}}) `talos.config.inline`.

### Strategic Merge Patches with `$patch: delete`

Talos Linux now supports removing parts of the machine configuration by [patching]({{< relref "../../talos-guides/configuration/patching#strategic-merge-patches" >}}) using the `$patch: delete` syntax similar to the Kubernetes strategic merge patch.

## Miscellaneous

### Diagnostics

Talos Linux now shows diagnostics information for common problems related to misconfiguration via `talosctl health` and Talos dashboard.

### `talos.halt_if_installed` kernel argument

Starting with Talos 1.8, ISO's generated from Boot Assets would have a new kernel argument `talos.halt_if_installed` which would pause the boot sequence until boot timeout if Talos is already installed on the disk.
ISOs generated for pre 1.8 versions would not have this kernel argument.

This can be also explicitly enabled by setting `talos.halt_if_installed=1` in kernel argument.

### Platform Support

Talos Linux now supports [Apache CloudStack platform]({{< relref "../../talos-guides/install/cloud-platforms/cloudstack" >}}).

### ZSTD Compression

Talos Linux now compresses kernel and initramfs using `zstd` (previously `xz` was used).
Linux arm64 kernel is now compressed (previously it was uncompressed).

## Component Updates

* Kubernetes: 1.31.1
* Linux: 6.6.49
* containerd: 2.0.0-rc.4
* runc: 1.2.0-rc.3
* etcd: 3.5.16
* Flannel: 0.25.6
* Flannel CNI plugin: 1.5.1
* CoreDNS: 1.1.13

Talos is built with Go 1.22.7.
