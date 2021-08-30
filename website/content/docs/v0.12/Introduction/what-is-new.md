---
title: What's New in Talos 0.12
weight: 5
---

### Security

* `etcd` PKI moved to `/system/secrets`
* `kubelet` bootstrap CSR auto-signing scoped to `kubelet` bootstrap tokens only
* enforce default `seccomp` profile on all system containers
* run system services apid, trustd, and etcd as non-root users

### Performance

* machined uses less memory and CPU time
* more disk encryption options are exposed via the machine configuration
* disk partitions are now aligned properly with minimum I/O size
* Talos system processes are moved under proper cgroups, resource metrics are now available via the kubelet
* OOM score is set on the system processes making sure they are killed last under memory pressure

### etcd

New etcd cluster members are now joined in [learner mode](https://etcd.io/docs/v3.4/learning/design-learner/), which improves cluster resiliency
to member join issues.

### Machine Configuration

Machine configuration is validated now for unsupported keys.
This change allows to catch issues with YAML indentation.

### Networking

* multiple static addresses can be specified for the interface with new `.addresses` field (old `.cidr` field is deprecated now)
* static addresses can be set on interfaces configured with DHCP

### Kubernetes Upgrades

`talosctl upgrade-k8s` now checks if cluster has any resources which are going to be removed or migrated to the new version after upgrade
and shows that as a warning before the upgrade.
Additionally, `upgrade-k8s` command now has `--dry-run` flag that only prints out warnings and upgrade summary.

### Sysctl Configuration

Sysctl Kernel Params configuration was completely rewritten to be based on controllers and resources,
which makes it possible to apply `.machine.sysctls` in immediate mode (without a reboot).
`talosctl get kernelparams` returns merged list of KSPP, Kubernetes and user defined params along with
the default values overwritten by Talos.

### Equinix Metal

Added support for Equinix Metal IPs for the Talos virtual (shared) IP (option `equinixMetal` under `vip` in the machine configuration).
Talos automatically re-assigns IP using the Equinix Metal API when leadership changes.

### Support for Self-hosted Control Plane Dropped

> **Note**: This item only applies to clusters bootstrapped with Talos <= 0.8.

Talos 0.12 completely removes support for self-hosted Kubernetes control plane (bootkube-based).
Talos 0.9 introduced support for Talos-managed control plane and provided migration path to convert self-hosted control plane
to Talos-managed static pods.
Automated and manual conversion process is available in Talos from 0.9.x to 0.11.x.
For clusters bootstrapped with bootkube (Talos <= 0.8), please make sure control plane is converted to Talos-managed
before upgrading to Talos 0.12.
Current control plane status can be checked with `talosctl get bootstrapstatus` before performing upgrade to Talos 0.12.

### Cluster API v0.3.x

Cluster API v0.3.x (v1alpha3) is not compatible with Kubernetes 1.22 used by default in Talos 0.12.
Talos can be configued to use Kubernetes 1.21 or CAPI v0.4.x components can be used instead.

### Join Node Type

Node type `join` was renamed to `worker` for clarity.
The old value is still accepted in the machine configuration but deprecated.
`talosctl gen config` now generates `worker.yaml` instead of `join.yaml`.

### Component Updates

* Linux: 5.10.58
* Kubernetes: 1.22.1
* containerd: 1.5.5
* runc: 1.0.1
* GRUB: 2.06
* Talos is built with Go 1.16.7
