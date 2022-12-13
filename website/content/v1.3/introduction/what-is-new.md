---
title: What's New in Talos 1.3
weight: 50
description: "List of new and shiny features in Talos Linux."
---

See also [upgrade notes]({{< relref "../talos-guides/upgrading-talos/">}}) for important changes.

## Component Updates

* Kubernetes: v1.26.0
* Flannel: v0.20.2
* CoreDNS: v1.10.0
* etcd: v3.5.6
* Linux: 5.15.82
* containerd: v1.6.12

Talos is built with Go 1.19.4.

## Kubernetes

### `kube-apiserver` Custom Audit Policy

Talos now supports setting custom audit policy for `kube-apiserver` in the machine configuration.

```yaml
cluster:
  apiServer:
    auditPolicy: |
      apiVersion: audit.k8s.io/v1
      kind: Policy
      rules:
        - level: Metadata
```

### `etcd` Secrets Encryption with `secretbox` algorithm

By default new clusters will use `secretbox` for etcd [secrets encryption](https://kubernetes.io/docs/tasks/administer-cluster/encrypt-data/) instead of AES-CBC.
If both are configured then `secretbox` will take precedence for new writes.
Old clusters may keep using AES-CBC.
To enable `secretbox` you need to add an encryption secret at `cluster.secretboxEncryptionSecret` after an upgrade to Talos 1.3.
You should keep `aescbcEncryptionSecret` however, even if `secretbox` is enabled, older data will still be encrypted with AES-CBC.

How to generate the secret for `secretbox`:

```bash
dd if=/dev/random of=/dev/stdout bs=32 count=1 | base64
```

### Node Labels

Talos now supports specifying node labels in the machine configuration:

```yaml
machine:
  nodeLabels:
    rack: rack1a
    zone: us-east-1a
```

Changes to the node labels will be applied immediately without restarting `kubelet`.

Talos keeps track of the owned node labels in the `talos.dev/owned-labels` annotation.

### Static Pod Manifests

Talos by default (for new clusters) doesn't configure `kubelet` to watch `/etc/kubernetes/manifests` directory for static pod manifests.
Talos-managed static pods are served via local HTTP server which prevents potential security vulnerabilities related to malicious static pods manifests
being placed to the aforementioned directory.

Static pods should always be configured in `machine.pods` instead of using `machine.files` to put files to `/etc/kubernetes/manifests` directory.
To re-enable support for `/etc/kubernetes/manifests` you may set `machine.kubelet.disableManifestsDirectory`.

Example:

```yaml
machine:
  kubelet:
    disableManifestsDirectory: no
```

## etcd

### `etcd` Consistency Check

Talos enables [--experimental-compact-hash-check-enabled](https://github.com/etcd-io/etcd/pull/14120) option by default to improve
`etcd` store consistency guarantees.

This options is only available with `etcd` >= v3.5.5, so Talos doesn't support versions of `etcd` older than v3.5.5 (Talos 1.3.0 defaults to `etcd` v3.5.6).

### `etcd` Member ID

Talos now internally handles etcd member removal by member ID instead of member name (hostname).
This resolves the case when member name is not accurate or empty (eg: when `etcd` hasn't fully joined yet).

Command `talosctl etcd remove-member` now accepts member IDs instead of member names.

A new resource can be used to get member ID of the Talos node:

```bash
$ talosctl get etcdmember
NODE         NAMESPACE   TYPE         ID      VERSION   MEMBER ID
10.150.0.4   etcd        EtcdMember   local   1         143fab7c7ccd2577
```

## CRI (containerd)

### CRI Configuration Overrides

Talos no longer supports CRI config overrides placed in `/var/cri/conf.d` directory.

[New way to add configuration overrides]({{< relref "../talos-guides/configuration/containerd/" >}}) correctly handles merging of containerd/CRI plugin configuration.

### Registry Mirrors

Talos had an inconsistency in the way registry mirror endpoints are handled when compared with `containerd` implementation:

```yaml
machine:
  registries:
    mirrors:
      docker.io:
        endpoints:
          - "https://mirror-registry/v2/mirror.docker.io"
```

Talos would use endpoint `https://mirror-registry/v2/mirror.docker.io`, while `containerd` would use `https://mirror-registry/v2/mirror.docker.io/v2`.
This inconsistency is now fixed, and Talos uses same endpoint as `containerd`.

New `overridePath` configuration is introduced to skip appending `/v2` both on Talos and `containerd` side:

```yaml
machine:
  registries:
    mirrors:
      docker.io:
        endpoints:
          - "https://mirror-registry/v2/mirror.docker.io"
        overridePath: true
```

### registry.k8s.io

Talos now uses `registry.k8s.io` instead of `k8s.gcr.io` for Kubernetes container images.

See [Kubernetes documentation](https://kubernetes.io/blog/2022/11/28/registry-k8s-io-faster-cheaper-ga/) for additional details.

If using registry mirrors, or in air-gapped installations you may need to update your configuration.

## Linux

### cgroups v1

Talos always defaults to using `cgroups v2` when Talos doesn't run in a container (when running in a container
Talos follows the host `cgroups` mode).
Talos can now be forced to use `cgroups v1` by setting boot kernel argument `talos.unified_cgroup_hierarchy=0`:

```yaml
machine:
  install:
    extraKernelArgs:
      - "talos.unified_cgroup_hierarchy=0"
```

Current `cgroups` mode can be checked with `talosctl ls /sys/fs/cgroup`:

`cgroups v1`:

```text
blkio
cpu
cpuacct
cpuset
devices
freezer
hugetlb
memory
net_cls
net_prio
perf_event
pids
```

`cgroups v2`:

```text
cgroup.controllers
cgroup.max.depth
cgroup.max.descendants
cgroup.procs
cgroup.stat
cgroup.subtree_control
cgroup.threads
cpu.stat
cpuset.cpus.effective
cpuset.mems.effective
init
io.stat
kubepods
memory.numa_stat
memory.stat
podruntime
system
```

> Note: `cgroupsv1` is deprecated and it should be used only for compatibility with workloads which don't support `cgroupsv2` yet.

### Kernel Command Line `ip=` Argument

Talos now supports referencing interface name via `enxMAC` address notation in the `ip=` argument:

```text
ip=172.20.0.2::172.20.0.1:255.255.255.0::enx7085c2dfbc59
```

Talos correctly handles multiple `ip=` arguments, and also enables forcing DHCP on a specific interface:

```text
vlan=eth0.137:eth0 ip=eth0.137:dhcp
```

### Kernel Module Parameters

Talos now supports settings kernel module parameters.

Example:

```yaml
machine:
  kernel:
    modules:
      - name: "br_netfilter"
        parameters:
          - nf_conntrack_max=131072
```

### BTF Support

Talos Linux kernel now ships with [BTF (BPF Type Format)](https://www.containiq.com/post/btf-bpf-type-format) support enabled:

```bash
$ talosctl -n 10.150.0.4 ls -l /sys/kernel/btf
NODE         MODE         UID   GID   SIZE(B)    LASTMOD           NAME
10.150.0.4   drwxr-xr-x   0     0     0          Dec 13 16:51:19   .
10.150.0.4   -r--r--r--   0     0     11578002   Dec 13 16:51:19   vmlinux
```

This can be used to compile BPF programs against the kernel without kernel sources, or to load relocatable BPF programs.

## Platform Support

### Exocale Platform

Talos adds support for a new platform: [Exoscale](https://www.exoscale.com/).

Exoscale provides a firewall, TCP load balancer and autoscale groups.
It works well with CCM and Kubernetes node autoscaler.

### Nano Pi R4S

Talos now supports the Nano Pi R4S SBC.

### Raspberry Generic Images

The Raspberry Pi 4 specific image has been deprecated and will be removed in the v1.4 release of Talos.
Talos now ships a generic Raspberry Pi image that should support more Raspberry Pi variants.
Refer to the [docs]({{< relref "../talos-guides/install/single-board-computers/rpi_generic/" >}}) to find which ones are supported.

### PlatformMetadata Resource

Talos now publishes information about the platform it is running on in the `PlatformMetadata` resource:

```yaml
# talosctl get platformmetadata -o yaml
spec:
    platform: equinixMetal
    hostname: ci-blue-worker-amd64-0
    region: dc
    zone: dc13
    instanceType: c3.medium.x86
    instanceId: efc0f667-XXX-XXX-XXXX-XXXXXXX
    providerId: equinixmetal://efc0f667-XXX-XXX-XXXX-XXXXXXX
```

## Networking

### KubeSpan

KubeSpan MTU link size is now configurable via `network.kubespan.mtu` setting in the machine configuration.
Default KubeSpan MTU assumes that the underlying network MTU is 1500 bytes, so if the underlying network MTU is different, KubeSpan MTU should be adjusted accordingly.

KubeSpan automatically publishes machine external (public) IP as a machine endpoint (as discovered by connecting to the discovery service), this allows establishing a connection
to a machine behind NAT if the KubeSpan port 51820 is forwarded to the machine.

KubeSpan by default publishes all machine addresses as Wireguard endpoints and finds the set of endpoints that are reachable for each pair of machines.
A set of endpoints can be manually filtered via `machine.network.kubespan.filters.endpoints` setting in the machine configuration.

### Route MTU

Talos now supports setting MTU for a specific route.

## `talosctl`

### Action Tracking

Now action tracking for commands `talosctl reboot`, `talosctl shutdown`, `talosctl reset` and `talosctl upgrade` is enabled by default.
Previous behavior can be restored by setting `--wait=false` flag.

### `talosctl machineconfig patch`

A new subcommand, `machineconfig patch` is added to `talosctl` to allow patching of machine configuration.

It accepts a machineconfig file and a list of patches as input, and outputs the patched machine configuration.

Patches can be sourced from the command line or from a file.
Output can be written to a file or to stdout.

Example:

```bash
talosctl machineconfig patch controlplane.yaml --patch '[{"op":"replace","path":"/cluster/clusterName","value":"patch1"}]' --patch @/path/to/patch2.json
```

Additionally, `talosctl machineconfig gen` subcommand is introduced as an alias to `talosctl gen config`.

### `talosctl gen config`

The command `talosctl gen config` now supports generating a single type of output (e.g. controlplane machine configuration) by specifying the `--output-types` flag,
which is useful with pre-generated secrets bundle, e.g.:

```bash
$ talosctl gen secrets # this outputs secrets bundle to secrets.yaml
$ talosctl gen config mycluster https://mycluster:6443 --with-secrets secrets.yaml --output-types controlplane -o -
version: v1alpha1 # Indicates the schema used to decode the contents.
debug: false # Enable verbose logging to the console.
persist: true # Indicates whether to pull the machine config upon every boot.
# Provides machine specific configuration options.
machine:
...
```

### `talosctl get -o jsonpath`

The command `talosctl get` now supports `jsonpath` output format:

```bash
$ talosctl -n 10.68.182.3 get address -o jsonpath='{.spec.address}
10.68.182.3/31
127.0.0.1/8
::1/128
192.168.11.128/32
```

## Developer Experience

### New Go Module Path

Talos now uses `github.com/siderolabs/talos` and `github.com/siderolabs/talos/pkg/machinery` as a Go module path.
