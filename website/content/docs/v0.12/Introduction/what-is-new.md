---
title: What's New in Talos 0.10
weight: 5
---

## Disaster Recovery

Talos now supports `etcd` [snapshots and recovery](../../guides/disaster-recovery/) from the snapshotted state.
Periodic snapshots of `etcd` data can be taken with `talosctl etcd snapshot` command, and in case of catastrophic control plane
failure `etcd` contents can be recovered from the latest snapshot with `talosctl bootstrap --recover-from=` command.

## Time Synchronization

The `timed` service was replaced with a new time sync controller without any machine configuration changes.
There should be no user-visible changes in the way new time synchronization process works, logs are now
available via `talosctl logs controller-runtime`.
Talos now prefers last successful time server (by IP address) on each sync attempt, which improves sync accuracy.

## Single Board Computers

Talos added support for the [Radxa Rock PI 4c](../../single-board-computers/rockpi_4/) board.
`u-boot` version was updated to fix the boot and USB issues on Raspberry Pi 4 8GiB version.

## Optimizations

Multiple optimizations were applied to reduce Talos `initramfs` size and memory footprint.
As a result, we see a reduction of memory usage of around 100 MiB for the core Talos components which leaves more resources available for you workloads.

## Install Disk Selector

Install section of the machine config now has `diskSelector` [field](../../reference/configuration/#installconfig) that allows querying install disk using the list of qualifiers:

```yaml
...
  install:
    diskSelector:
      size: >= 500GB
      model: WDC*
...
```

`talosctl -n <IP> disks -i` can be used to check allowed disk qualifiers when the node is running in the maintenance mode.

## Inline Kubernetes Manifests

Kubernetes manifests can now be submitted in the machine configuration using the `cluster.inlineManifests` [field](../../reference/configuration/#clusterconfig),
which works same way as `cluster.extraManifests` field, but manifest contents are passed inline in the machine configuration.

## Updated Components

Linux: 5.10.19 -> 5.10.29

Kubernetes: 1.20.5 -> 1.21.0

Go: 1.15 -> 1.16
