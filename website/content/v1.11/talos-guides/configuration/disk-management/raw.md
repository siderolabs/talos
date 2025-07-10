---
title: "Raw Volumes"
description: "Configuring raw volumes to allocate unformatted storage."
weight: 40
---

Raw Volumes allow allocate an unformatted partition, label it, and make it available for use by advanced storage drivers like Container Storage Interface (CSI) drivers.

When a raw volume configuration is applied, Talos Linux will either locate an existing volume or provision a new one.
The volume will be created on the disk which satisfies the `diskSelector` expression and has enough free space to satisfy the `minSize` requirement.

The raw volume is identified by a unique name, which is used both as a mount location and as a label for the volume.
The volume name must be unique across all raw volumes, and it should be between 1 and 34 characters long, and can only contain ASCII letters, digits, and `-` (dash) characters.

The volume label is derived from the volume name as `r-<volume-name>`, and it is used to identify the volume on the disk after initial provisioning.

Disk encryption can be optionally enabled for raw volumes.

> Note: If you need to allocate a volume to be mounted to a container, please see [User Volumes]({{< relref "user" >}}) guide.

### Creating Raw Volumes

To create a raw volume, append the following [document]({{< relref "../../../reference/configuration/block/rawvolumeconfig" >}}) to the machine configuration:

```yaml
# raw-volume.patch.yaml
apiVersion: v1alpha1
kind: RawVolumeConfig
name: openebs-vol1
provisioning:
  diskSelector:
    match: "!system_disk"
  minSize: 2GB
  maxSize: 2GB
```

For example, this machine configuration patch can be applied using the following command:

```bash
talosctl --nodes <NODE> patch mc --patch @raw-volume.patch.yaml
```

In this example, a raw volume named `openebs-vol1` is created on the first disk which is not the system disk and has `2GB` of disk space available.
The volume will be created as a partition with a size of `2GB`.

The status of the volume can be checked using the following command:

```bash
$ talosctl get volumestatus r-openebs-vol1 # note r- prefix
NODE         NAMESPACE   TYPE           ID               VERSION   TYPE        PHASE   LOCATION    SIZE
172.20.0.5   runtime     VolumeStatus   r-openebs-vol1   1         partition   ready   /dev/sda1   2.0 GB
```

This volume can be referenced using a stable symlink `/dev/disk/by-partlabel/r-openebs-vol1`, which is created automatically by Talos Linux.

### Removing Raw Volumes

Before removing a raw volume, ensure that it is not used anymore.

In order to remove a raw volume, first remove the configuration document from the machine configuration.
The `VolumeStatus` resource will be removed automatically by Talos Linux.

> Note: The actual disk data hasn't been removed yet, so you can re-apply the raw volume configuration back
> and it will be re-provisioned on the same disk.

To wipe the disk data, and make it allocatable again, use the following command:

```bash
talosctl wipe disk sda1 --drop-partition
```

The `sda1` is the partition name, and it can be obtained from the `VolumeStatus` resource before the raw volume is removed,
or from the `DiscoveredVolume` resource any time later.
