---
title: What's New in Talos 1.2
weight: 50
description: "List of new and shiny features in Talos Linux."
---

See also [upgrade notes]({{< relref "../talos-guides/upgrading-talos/">}}) for important changes.

## Component Updates

* Linux: 5.15.64
* Flannel 0.19.1
* containerd 1.6.8
* runc: v1.1.4
* Kubernetes: v1.25.0

Talos is built with Go 1.19.

## Kubernetes

### Control Plane Labels and Taints

Talos now defaults to `node-role.kubernetes.io/control-plane` label/taint.

On upgrades Talos now removes the `node-role.kubernetes.io/master` label/taint on control-plane nodes and replaces it with the `node-role.kubernetes.io/control-plane` label/taint.
Workloads that tolerate the old taints or have node selectors with the old labels will need to be updated.

> Previously Talos labeled control plane nodes with both `control-plane` and `master` labels and tainted the node with `master` taint.

### Scheduling on Control Plane Nodes

Machine configuration `.cluster.allowSchedulingOnMasters` is deprecated and replaced by `.cluster.allowSchedulingOnControlPlanes`.
The `.cluster.allowSchedulingOnMasters` will be removed in a future release of Talos.
If both `.cluster.allowSchedulingOnMasters` and `.cluster.allowSchedulingOnControlPlanes` are set to `true`, the `.cluster.allowSchedulingOnControlPlanes` will be used.

### Control Plane Components

Talos now run all Kubernetes Control Plane Components with the CRI default Seccomp Profile and other recommendations as described in
[KEP-2568](https://github.com/kubernetes/enhancements/tree/master/keps/sig-cluster-lifecycle/kubeadm/2568-kubeadm-non-root-control-plane).

### `k8s.gcr.io` Registry

Talos now defaults to adding a registry mirror configuration in the machine configuration for `k8s.gcr.io` pointing to both `registry.k8s.io` and `k8s.gcr.io` unless overridden.

This is in line with the Kubernetes 1.25 release having the new `registry.k8s.io` registry endpoint.

This is only enabled by default on newly generated configurations and not on upgrades.
This can be enabled with a machine configuration as follows:

```yaml
machine:
  registries:
    mirrors:
      k8s.gcr.io:
        endpoints:
          - https://registry.k8s.io
          - https://k8s.gcr.io
```

### `seccomp` Profiles

Talos now supports creating custom seccomp profiles on the host machine which in turn can be used by Kubernetes workloads.
It can be configured in the machine config as below:

```yaml
machine:
  seccompProfiles:
    - name: audit.json
      value:
        defaultAction: SCMP_ACT_LOG
    - name: deny.json
      value: {"defaultAction":"SCMP_ACT_LOG"}
```

This profile data can be either configured as a YAML definition or as a JSON string.

The profiles are created on the host under `/var/lib/kubelet/seccomp/profiles`.

### Default `seccomp` Profile

Talos now runs Kubelet with the CRI default Seccomp Profile enabled.
This can be disabled by setting `.machine.kubelet.defaultRuntimeSeccompProfileEnabled` to `false`.

This feature is not enabled automatically on upgrades, so upgrading to Talos v1.2 needs this to be explicitly enabled.

See [documentation]({{< relref "../kubernetes-guides/configuration/seccomp-profiles/" >}}) for more details.

## Machine Configuration

### Strategic Merge Patching

In addition to JSON (RFC6902) patches Talos now supports [strategic merge patching]({{< relref "../talos-guides/configuration/patching/">}}).

For example, machine hostname can be set with the following patch:

```yaml
machine:
  network:
    hostname: worker1
```

Patch format is detected automatically.

### `talosctl apply-config`

`talosctl apply-config` now supports patching the machine config file in memory before submitting it to the node.

## Networking

### Bridge Support

Talos now supports configuration of Linux bridge interfaces:

```yaml
machine:
  network:
    interfaces:
      - interface: br0
        bridge:
          stp:
            enabled: true
          interfaces:
            - eth0
            - eth1
```

See [configuration reference]({{< relref "../reference/configuration/#bridge" >}}) for more details.

### VLANs

Talos now supports dracut-style `vlan` kernel argument to allow
installing Talos Linux in networks where ports are not tagged
with a default VLAN:

```text
vlan=eth1.5:eth1 ip=172.20.0.2::172.20.0.1:255.255.255.0::eth1.5:::::
```

[Machine configuration]({{<relref "../reference/configuration#vlan" >}}) now supports specifying DHCP options for VLANs.

### Stable Default Hostname

Talos now generates the default hostname (when there is no explicitly specified hostname) for the nodes based on the
node id (e.g. `talos-2gd-76y`) instead of using the DHCP assigned IP address (e.g. `talos-172-20-0-2`).

This ensures that the node hostname is not changed when DHCP assigns a new IP to a node.

> Note: the stable hostname generation algorithm changed between v1.2.0-beta.0 and v1.2.0-beta.1, please take care when upgrading
> from versions >= 1.2.0-alpha.1 to versions >= 1.2.0-beta.1 when using stable default hostname feature.

### Packet Capture

Talos now supports capturing packets on a network interface with `talosctl pcap` command:

```shell
talosctl pcap --interface eth0
```

## Cluster Discovery and KubeSpan

### KubeSpan Kubernetes Network Advertisement

KubeSpan no longer by default advertises Kubernetes pod networks of the node over KubeSpan.
This means that CNI should handle encapsulation of pod-to-pod traffic into the node-to-node tunnel,
and node-to-node traffic will be handled by KubeSpan.
This provides better compatibility with popular CNIs like Calico and Cilium.

Old behavior can be restored by setting `.machine.kubespan.advertiseKubernetesNetworks = true` in the machine config.

### Kubernetes Discovery Backend

Kubernetes cluster discovery backend is now disabled by default for new clusters.
This backend doesn't provide any benefits over the Discovery Service based backend, while it
causes issues for KubeSpan enabled clusters when control plane endpoint is KubeSpan-routed.

For air-gapped installations when the Discovery Service is not enabled, Kubernetes Discovery Backend can be enabled by applying
the following machine configuration patch:

```yaml
cluster:
  discovery:
    registries:
      kubernetes:
        disabled: false
```

## `etcd`

### Advertised and Listen Subnets

Machine configuration setting `cluster.etcd.subnet` is deprecated, but still supported.

Two new configuration settings are introduced to control precisely which subnet is used for etcd peer communication:

```yaml
cluster:
  etcd:
    advertisedSubnets:
       - 10.0.0.0/24
    listenSubnets:
       - 10.0.0.0/24
       - 192.168.0.0/24
```

The `advertisedSubnets` setting is used to control which subnet is used for etcd peer communication, it will be advertised
by each peer for other peers to connect to.
If `advertiseSubnets` is set, `listenSubnets` defaults to the same value, so that
`etcd` only listens on the same subnet as it advertises.
Additional subnets can be configured in `listenSubnets` if needed.

Default behavior hasn't changed - if the `advertisedSubnets` is not set, Talos picks up the first available network address as
an advertised address and `etcd` is configured to listen on all interfaces.

> Note: most of the `etcd` configuration changes are accepted on the fly, but they are fully applied only after a reboot.

## CLI

### Tracking Progress of API Calls

`talosctl` subcommands `shutdown`, `reboot`, `reset` and `upgrade` now have a new flag `--wait` to
wait until the operation is completed, displaying information on the current status of each node.

A new `--debug` flag is added to these commands to get the kernel logs output from these nodes if the operation fails.

![track-cli-action-progress](/images/track-reboot.gif)

### Generating Machine Config from Secrets

It is now possible to pre-generate secret material for the cluster with `talosctl gen secrets`:

```shell
talosctl gen secrets -o cluster1-secrets.yaml
```

Secrets file should be stored in a safe place, and machine configuration for the node in the cluster can be generated on demand
with `talosctl gen config`:

```shell
talosctl gen config --with-secrets cluster1-secrets.yaml cluster1 https://cluster1.example.com:6443/
```

This way configuration can be generated on demand, for example with configuration patches.
Nodes with machine configuration generated from the same secrets file can join each other to form a cluster.

## Integrations

### Talos API access from Kubernetes

Talos now supports access to its API from within Kubernetes.

It can be configured in the machine config as below:

```yaml
machine:
  features:
    kubernetesTalosAPIAccess:
      enabled: true
      allowedRoles:
        - os:reader
      allowedKubernetesNamespaces:
        - kube-system
```

This feature introduces a new custom resource definition, `serviceaccounts.talos.dev`.
Creating custom resources of this type will provide credentials to access Talos API from within Kubernetes.

The new CLI subcommand `talosctl inject serviceaccount` can be used to configure Kubernetes manifests with Talos service accounts as below:

```shell
talosctl inject serviceaccount -f manifests.yaml > manifests-injected.yaml
kubectl apply -f manifests-injected.yaml
```

See [documentation]({{< relref "../advanced/talos-api-access-from-k8s/" >}}) for more details.

## Migration

### Migrating from `kubeadm`

`talosctl gen` command supports generating a secrets bundle from a Kubernetes PKI directory (e.g. `/etc/kubernetes/pki`).

This secrets bundle can then be used to generate a machine config.

This facilitates [migrating clusters]({{< relref "../advanced/migrating-from-kubeadm" >}}) (e.g. created using `kubeadm`) to Talos.

```shell
talosctl gen secrets --kubernetes-bootstrap-token znzio1.1ifu15frz7jd59pv --from-kubernetes-pki /etc/kubernetes/pki
talosctl gen config --with-secrets secrets.yaml my-cluster https://172.20.0.1:6443
```

## Platform Updates

### Metal

The kernel parameter `talos.config` can now substitute system information into placeholders inside its URL query values.

This example shows all supported variables:

```text
http://example.com/metadata?h=${hostname}&m=${mac}&s=${serial}&u=${uuid}
```

## Extensions

### NVIDIA GPU support promoted to beta

NVIDIA GPU support on Talos has been promoted to beta and SideroLabs now publishes the NVIDIA Open GPU Kernel Modules as a system extension making it easier to run GPU workloads on Talos.
Refer to enabling NVIDIA GPU support docs here:

* [OSS Drivers]({{<relref "../talos-guides/configuration/nvidia-gpu/">}})
* [Proprietary Drivers]({{<relref "../talos-guides/configuration/nvidia-gpu-proprietary/">}})
* [Fabric Manager]({{<relref "../talos-guides/configuration/nvidia-fabricmanager/">}})

## Deprecations

`--masters` flag on `talosctl cluster create` is deprecated.
Use `--controlplanes` instead.

Machine configuration `.cluster.allowSchedulingOnMasters` is deprecated and replaced by `.cluster.allowSchedulingOnControlPlanes`.
