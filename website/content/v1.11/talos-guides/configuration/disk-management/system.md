---
title: "System Volumes"
description: "Configuring Talos Linux system volumes, for example `EPHEMERAL` volume."
weight: 20
---

Talos Linux has a set of system volumes that are used for various purposes, such as storing the system state, ephemeral data, and more.
This guide provides an overview of the system volumes and how to configure them.

## `EPHEMERAL` Volume

The `EPHEMERAL` volume is a system volume that is used for storing ephemeral data, such as container data, downloaded images, logs, and `etcd` data (for controlplane nodes).

> Note: The volume configuration in the machine configuration is only applied when the volume has not been provisioned yet.
> So applying changes after the initial provisioning will not have any effect.

To configure the `EPHEMERAL` (`/var`) volume, append the following [document]({{< relref "../../../reference/configuration/block/volumeconfig" >}}) to the machine configuration:

```yaml
apiVersion: v1alpha1
kind: VolumeConfig
name: EPHEMERAL
provisioning:
  diskSelector:
    match: disk.transport == 'nvme'
  minSize: 2GB
  maxSize: 40GB
  grow: false
```

Every field in the `VolumeConfig` resource is optional, and if a field is not specified, the default value is used.
The default built-in values are:

```yaml
provisioning:
    diskSelector:
        match: system_disk
    minSize: 2GiB
    grow: true
```

By default, the `EPHEMERAL` volume is provisioned on the system disk, which is the disk where Talos Linux is installed.
It has a minimum size of 2 GiB and automatically grows to utilize the maximum available space on the disk.

If you would like to keep the `EPHEMERAL` volume on the system disk but limit its size to 40 GiB, you can set the `maxSize` field to `40GiB`:

```yaml
apiVersion: v1alpha1
kind: VolumeConfig
name: EPHEMERAL
provisioning:
  maxSize: 40GiB
```

If you want to create a separate partition for `EPHEMERAL` on a different disk, you can set the `diskSelector` field to select the desired disk:

```yaml
apiVersion: v1alpha1
kind: VolumeConfig
name: EPHEMERAL
provisioning:
  diskSelector:
    match: disk.transport == 'nvme' && !system_disk
```

> Note: Currently, encryption for `EPHEMERAL` and `STATE` volumes is configured using [another config document]({{< relref "../../../reference/configuration/v1alpha1/config#Config.machine.systemDiskEncryption" >}}).

## `IMAGECACHE` Volume

This system volume is not provisioned by default, and it only gets created if the [Image Cache]({{< relref "../image-cache" >}}) feature is enabled.

See [Image Cache configuration]({{< relref "../image-cache#configuration" >}}) for more details.
