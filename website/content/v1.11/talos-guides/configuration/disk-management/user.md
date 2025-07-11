---
title: "User Volumes"
description: "Configuring user volumes to allocate local storage for Kubernetes workloads."
weight: 30
---

User Volumes allow to treat available disk space as a pool of allocatable resource, which can be dynamically allocated to different applications.
The user volumes are supposed to be used mostly for `hostPath` mounts in Kubernetes, but they can be used for other purposes as well.

When a user volume configuration is applied, Talos Linux will either locate an existing volume or provision a new one.
The volume will be created on the disk which satisfies the `diskSelector` expression and has enough free space to satisfy the `minSize` requirement.

The user volume is identified by a unique name, which is used both as a mount location and as a label for the volume.
The volume name must be unique across all user volumes, and it should be between 1 and 34 characters long, and can only contain ASCII letters, digits, and `-` (dash) characters.

The volume label is derived from the volume name as `u-<volume-name>`, and it is used to identify the volume on the disk after initial provisioning.
The volume mount location is `/var/mnt/<volume-name>`, and it gets automatically propagated into the `kubelet` container to provide additional features like `subPath` mounts.

Disk encryption can be optionally enabled for user volumes.

## Creating User Volumes

To create a user volume, append the following [document]({{< relref "../../../reference/configuration/block/uservolumeconfig" >}}) to the machine configuration:

```yaml
# user-volume.patch.yaml
apiVersion: v1alpha1
kind: UserVolumeConfig
name: ceph-data
provisioning:
  diskSelector:
    match: disk.transport == 'nvme'
  minSize: 100GB
  maxSize: 200GB
```

For example, this machine configuration patch can be applied using the following command:

```bash
talosctl --nodes <NODE> patch mc --patch @user-volume.patch.yaml
```

In this example, a user volume named `ceph-data` is created on the first NVMe disk which has `100GB` of disk space available, and it will be created as maximum
of `200GB` if that space is available.

The status of the volume can be checked using the following command:

```bash
$ talosctl get volumestatus u-ceph-data # note u- prefix
NAMESPACE   TYPE           ID            VERSION   TYPE        PHASE   LOCATION         SIZE
runtime     VolumeStatus   u-ceph-data   2         partition   ready   /dev/nvme0n1p2   200 GB
```

If the volume fails to be provisioned, use the `-o yaml` flag to get additional details.

The volume is immediately mounted to `/var/mnt/ceph-data`:

```bash
$ talosctl get mountstatus
NAMESPACE   TYPE          ID           VERSION   SOURCE           TARGET               FILESYSTEM   VOLUME
runtime     MountStatus   u-ceph-data  2         /dev/nvme0n1p2   /var/mnt/ceph-data   xfs          u-ceph-data
```

It can be used in a Kubernetes pod as a `hostPath` mount:

```yaml
kind: Pod
spec:
  containers:
    - name: ceph
      volumeMounts:
        - mountPath: /var/lib/ceph
          name: ceph-data
  volumes:
    - name: ceph-data
      hostPath:
        path: /var/mnt/ceph-data
```

Please note, the path inside the container can be different from the path on the host.

## Removing User Volumes

Before removing a user volume, ensure that it is not mounted in any Kubernetes pod.

In order to remove a user volume, first remove the configuration document from the machine configuration.
The `VolumeStatus` and `MountStatus` resources will be removed automatically by Talos Linux.

> Note: The actual disk data hasn't been removed yet, so you can re-apply the user volume configuration back
> and it will be re-provisioned on the same disk.

To wipe the disk data, and make it allocatable again, use the following command:

```bash
talosctl wipe disk nvme0n1p2 --drop-partition
```

The `nvme0n1p2` is the partition name, and it can be obtained from the `VolumeStatus` resource before the user volume is removed,
or from the `DiscoveredVolume` resource any time later.

> Note: If the `wipe disk` command fails with `blockdevice is in use by volume`, it means the user volume has not been removed from the machine configuration.
