---
title: "Disk Management"
description: "Guide on managing disks"
aliases:
  - ../../guides/disk-management
---

This guide provides an overview of the disk management features in Talos Linux.

## Listing Disks

To obtain a list of all available block devices (disks) on the machine, you can use the following command:

```bash
$ talosctl get disks
NODE         NAMESPACE   TYPE   ID        VERSION   SIZE    READ ONLY   TRANSPORT   ROTATIONAL   WWID                                                               MODEL            SERIAL
172.20.0.5   runtime     Disk   loop0     1         75 MB   true
172.20.0.5   runtime     Disk   nvme0n1   1         10 GB   false       nvme                     nvme.1b36-6465616462656566-51454d55204e564d65204374726c-00000001   QEMU NVMe Ctrl   deadbeef
172.20.0.5   runtime     Disk   sda       1         10 GB   false       virtio      true                                                                            QEMU HARDDISK
172.20.0.5   runtime     Disk   sdb       1         10 GB   false       sata        true         t10.ATA     QEMU HARDDISK                           QM00013        QEMU HARDDISK
172.20.0.5   runtime     Disk   sdc       1         10 GB   false       sata        true         t10.ATA     QEMU HARDDISK                           QM00001        QEMU HARDDISK
172.20.0.5   runtime     Disk   vda       1         13 GB   false       virtio      true
```

To obtain detailed information about a specific disk, execute the following command:

```yaml
# talosctl get disk sda -o yaml
node: 172.20.0.5
metadata:
    namespace: runtime
    type: Disks.block.talos.dev
    id: sda
    version: 1
    owner: block.DisksController
    phase: running
    created: 2024-08-29T13:06:43Z
    updated: 2024-08-29T13:06:43Z
spec:
    dev_path: /dev/sda
    size: 10485760000
    human_size: 10 GB
    io_size: 512
    sector_size: 512
    readonly: false
    cdrom: false
    model: QEMU HARDDISK
    modalias: scsi:t-0x00
    bus_path: /pci0000:00/0000:00:07.0/virtio4/host1/target1:0:0/1:0:0:0
    sub_system: /sys/class/block
    transport: virtio
    rotational: true
    symlinks:
        - /dev/disk/by-diskseq/10
        - /dev/disk/by-path/pci-0000:00:07.0
        - /dev/disk/by-path/virtio-pci-0000:00:07.0
```

## Discovered Volumes

Talos Linux monitors all block devices and partitions on the machine.
Details about these devices, including their type, can be found in the `DiscoveredVolume` resource.

```bash
$ talosctl get discoveredvolumes
NODE         NAMESPACE   TYPE               ID        VERSION   TYPE        SIZE     DISCOVERED   LABEL       PARTITIONLABEL
172.20.0.5   runtime     DiscoveredVolume   dm-0      1         disk        88 MB    xfs          STATE
172.20.0.5   runtime     DiscoveredVolume   loop0     1         disk        75 MB    squashfs
172.20.0.5   runtime     DiscoveredVolume   nvme0n1   1         disk        10 GB
172.20.0.5   runtime     DiscoveredVolume   sda       1         disk        10 GB
172.20.0.5   runtime     DiscoveredVolume   sdb       1         disk        10 GB
172.20.0.5   runtime     DiscoveredVolume   sdc       1         disk        2.1 GB   gpt
172.20.0.5   runtime     DiscoveredVolume   sdc1      1         partition   957 MB   xfs
172.20.0.5   runtime     DiscoveredVolume   sdc2      1         partition   957 MB   xfs
172.20.0.5   runtime     DiscoveredVolume   sdd       1         disk        1.0 GB   gpt
172.20.0.5   runtime     DiscoveredVolume   sdd1      1         partition   957 MB   xfs
172.20.0.5   runtime     DiscoveredVolume   sde       1         disk        10 GB
172.20.0.5   runtime     DiscoveredVolume   vda       1         disk        13 GB    gpt
172.20.0.5   runtime     DiscoveredVolume   vda1      1         partition   105 MB   vfat                     EFI
172.20.0.5   runtime     DiscoveredVolume   vda2      1         partition   1.0 MB                            BIOS
172.20.0.5   runtime     DiscoveredVolume   vda3      1         partition   982 MB   xfs          BOOT        BOOT
172.20.0.5   runtime     DiscoveredVolume   vda4      2         partition   1.0 MB   talosmeta                META
172.20.0.5   runtime     DiscoveredVolume   vda5      1         partition   105 MB   luks                     STATE
172.20.0.5   runtime     DiscoveredVolume   vda6      1         partition   12 GB    xfs          EPHEMERAL   EPHEMERAL
```

Talos Linux has built-in automatic detection for various filesystem types and GPT partition tables.
Currently, the following filesystem types are supported:

- `bluestore` (Ceph)
- `ext2`, `ext3`, `ext4`
- `iso9660`
- `luks` (LUKS encrypted partition)
- `lvm2`
- `squashfs`
- `swap`
- `talosmeta` (Talos Linux META partition)
- `vfat`
- `xfs`
- `zfs`

The discovered volumes can include both Talos-managed volumes and any other volumes present on the machine, such as Ceph volumes.

## Disk Layout

The default disk layout for Talos installation is as follows:

```text
+-----------------------------------------------------------------------------+
| Physical Disk (1TB)                                                         |
+=============+===========+==========+========================================+
| EFI (boot)  | META      | STATE    | EPHEMERAL (spans the rest of the disk) |
| [~1GB]      | [~1MB]    | [~100MB] | [~998GB]                               |
+-------------+-----------+----------+----------------------------------------+
```

In this diagram:

- `EFI`: the EFI partition used for booting the system.
- `META`: the partition used for storing Talos metadata.
- `STATE`: the partition used for storing the system state, including machine configuration.
- `EPHEMERAL`: the partition used for storing container data, downloaded images, logs, `etcd` data (for controlplane nodes) etc.

Talos Linux hardcodes the partition layout for the `EFI`, `META`, and `STATE` partitions.
The `EPHEMERAL` partition by default consumes all unallocated space, but it can be created on another disk or resized.

The `EPHEMERAL` partition is a catch-all location for storing data, while it might be desired to segregate the data into different partitions.
Talos supports creating additional user volumes to be used for different purposes: e.g. local storage for various applications, specific volumes per applications, etc.

### Single Disk Layout

```text
+-------------------------------------------------------------------------------------------------------+
| Physical Disk (1TB)                                                                                   |
+=============+===========+==========+============+===========+===============+=========================+
| EFI (boot)  | META      | STATE    | EPHEMERAL  | ceph-data | local-storage | << Unallocated Space >> |
| [~1GB]      | [~1MB]    | [~100MB] | [~200GB]   | [~100GB]  | [~200GB]      | [~500GB]                |
+-------------+-----------+----------+------------+-----------+---------------+-------------------------+
```

In this layout, the `EPHEMERAL` partition was limited to 200GB, and two additional partitions were created for `ceph-data` and `local-storage`.

### Multiple Disk Layout

```text
+---------------------------------------------------------------------------------------+
| Physical Disk 1 (1TB)                                                                 |
+=============+===========+==========+============+===========+=========================+
| EFI (boot)  | META      | STATE    | EPHEMERAL  | ceph-data | << Unallocated Space >> |
| [~1GB]      | [~1MB]    | [~100MB] | [~500GB]   | [~100GB]  | [~400GB]                |
+-------------+-----------+----------+------------+-----------+-------------------------+
| Physical Disk 2 (1TB)                                                                 |
+===============+=======================================================================+
| local-storage | << Unallocated Space >>                                               |
| [~500GB]      |  [~500GB]                                                             |
+---------------+-----------------------------------------------------------------------+
```

In this layout, the `EPHEMERAL` partition was limited to 500GB, and two additional partitions were created for `ceph-data` and `local-storage`,
and they were created on different disks.

## Volume Management

Talos Linux implements disk management through the concept of volumes.
A volume represents a provisioned, located, mounted, or unmounted entity, such as a disk, partition, or a directory/overlay mount.

Talos Linux provides volume configuration using the following machine configuration documents:

- `VolumeConfig`: used to configure system volumes (override default values)
- `UserVolumeConfig`: used to configure user volumes (extra volumes created by the user)

## System Volume Configuration

### `EPHEMERAL` Volume

> Note: The volume configuration in the machine configuration is only applied when the volume has not been provisioned yet.
> So applying changes after the initial provisioning will not have any effect.

To configure the `EPHEMERAL` (`/var`) volume, append the following [document]({{< relref "../../reference/configuration/block/volumeconfig" >}}) to the machine configuration:

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

> Note: Currently, encryption for `EPHEMERAL` and `STATE` volumes is configured using [another config document]({{< relref "../../reference/configuration/v1alpha1/config#Config.machine.systemDiskEncryption" >}}).

### `IMAGECACHE` Volume

This system volume is not provisioned by default, and it only gets created if the [Image Cache]({{< relref "image-cache" >}}) feature is enabled.

See [Image Cache configuration]({{< relref "image-cache#configuration" >}}) for more details.

## User Volumes

User Volumes allow to treat available disk space as a pool of allocatable resource, which can be dynamically allocated to different applications.
The user volumes are supposed to be used mostly for `hostPath` mounts in Kubernetes, but they can be used for other purposes as well.

When a user volume configuration is applied, Talos Linux will either locate an existing volume or provision a new one.
The volume will be created on the disk which satisfies the `diskSelector` expression and has enough free space to satisfy the `minSize` requirement.

The user volume is identified by a unique name, which is used both as a mount location and as a label for the volume.
The volume name must be unique across all user volumes, and it should be between 1 and 34 characters long, and can only contain ASCII letters, digits, and `-` (dash) characters.

The volume label is derived from the volume name as `u-<volume-name>`, and it is used to identify the volume on the disk after initial provisioning.
The volume mount location is `/var/mnt/<volume-name>`, and it gets automatically propagated into the `kubelet` container to provide additional features like `subPath` mounts.

Disk encryption can be optionally enabled for user volumes.

### Creating User Volumes

To create a user volume, append the following [document]({{< relref "../../reference/configuration/block/uservolumeconfig" >}}) to the machine configuration:

```yaml
apiVersion: v1alpha1
kind: UserVolumeConfig
name: ceph-data
provisioning:
  diskSelector:
    match: disk.transport == 'nvme'
  minSize: 100GB
  maxSize: 200GB
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

### Removing User Volumes

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

## Common Configuration

### Disk Selector

The `diskSelector` field is utilized to choose the disk where the volume will be provisioned.
It is a [Common Expression Language (CEL)](https://cel.dev/) expression that evaluates against the available disks.
The volume will be provisioned on the first disk that matches the expression and has sufficient free space for the volume.

The expression is evaluated in the following context:

- `system_disk` (`bool`) - indicates if the disk is the system disk
- `disk` (`Disks.block.talos.dev`) - the disk resource being evaluated

> Note: The `system_disk` variable is only populated after Talos installation, so you might see an error about `system_disk` not being defined
> before installation finishes.

For the disk resource, any field available in the resource specification can be used (use `talosctl get disks -o yaml` to see the output for your machine):

```yaml
dev_path: /dev/nvme0n1
size: 10485760000
pretty_size: 10 GB
io_size: 512
sector_size: 512
readonly: false
cdrom: false
model: QEMU NVMe Ctrl
serial: deadbeef
wwid: nvme.1b36-6465616462656566-51454d55204e564d65204374726c-00000001
bus_path: /pci0000:00/0000:00:09.0/nvme
sub_system: /sys/class/block
transport: nvme
symlinks:
    - /dev/disk/by-diskseq/11
    - /dev/disk/by-id/ata-QEMU_HARDDISK_QM00001
    - /dev/disk/by-path/pci-0000:00:1f.2-ata-1
    - /dev/disk/by-path/pci-0000:00:1f.2-ata-1.0
```

Additionally, constants for disk size multipliers are available:

- `KiB`, `MiB`, `GiB`, `TiB`, `PiB`, `EiB` - binary size multipliers (1024)
- `kB`, `MB`, `GB`, `TB`, `PB`, `EB` - decimal size multipliers (1000)

The disk expression is evaluated against each available disk, and the expression should either return `true` or `false`.
If the expression returns `true`, the disk is selected for provisioning.

> Note: In CEL, signed and unsigned integers are not interchangeable.
> Disk sizes are represented as unsigned integers, so suffix `u` should be used in constants to avoid type mismatch, e.g. `disk.size > 10u * GiB`.

Examples of disk selector expressions:

- `disk.transport == 'nvme'`: select the NVMe disks only
- `disk.transport == 'scsi' && disk.size < 2u * TiB`: select SCSI disks smaller than 2 TiB
- `disk.serial.startsWith('deadbeef') && !cdrom`: select disks with serial number starting with `deadbeef` and not of CD-ROM type
- `'/dev/disk/by-path/pci-0000:00:1f.2-ata-1' in disk.symlinks`: select disks with a specific stable symlink

### Minimum and Maximum Size

The `minSize` and `maxSize` fields define the minimum and maximum size of the volume, respectively.
Talos Linux will always ensure that the volume is at least `minSize` in size and will not exceed `maxSize`.
If `maxSize` is not set, the volume will grow to utilize the maximum available space on the disk.

If `grow` is set to `true`, the volume will automatically grow to utilize the maximum available space on the disk on each boot.

Setting `minSize` might influence disk selection - if the disk does not have enough free space to satisfy the minimum size requirement, it will not be selected for provisioning.

## Resources

The configuration of volumes is defined using the `VolumeConfig` resource, while the current state of volumes is stored in the `VolumeStatus` resource.

### Configuration

The volume configuration is managed by Talos Linux based on machine configuration.
To see configured volumes, use the following command:

```bash
$ talosctl get volumeconfigs
NODE         NAMESPACE   TYPE           ID                                  VERSION
172.20.0.2   runtime     VolumeConfig   /etc/cni                            2
172.20.0.2   runtime     VolumeConfig   /var/run                            2
172.20.0.2   runtime     VolumeConfig   EPHEMERAL                           2
172.20.0.2   runtime     VolumeConfig   ETCD                                2
172.20.0.2   runtime     VolumeConfig   META                                2
172.20.0.2   runtime     VolumeConfig   STATE                               3
172.20.0.2   runtime     VolumeConfig   u-extra                             2
172.20.0.2   runtime     VolumeConfig   u-p1                                2
172.20.0.2   runtime     VolumeConfig   u-p2                                2
```

In the provided output, the volumes `EPHEMERAL`, `META`, and `STATE` are system volumes managed by Talos, while `u-extra`, `u-p1` and `u-p2` are user configured volumes.

To get details about a specific volume configuration, use the following command:

```yaml
# talosctl get volumeconfig STATE -o yaml
node: 172.20.0.5
metadata:
    namespace: runtime
    type: VolumeConfigs.block.talos.dev
    id: STATE
    version: 4
    owner: block.VolumeConfigController
    phase: running
    created: 2024-08-29T13:22:04Z
    updated: 2024-08-29T13:22:17Z
    finalizers:
        - block.VolumeManagerController
spec:
    type: partition
    provisioning:
        wave: -1
        diskSelector:
            match: system_disk
        partitionSpec:
            minSize: 104857600
            maxSize: 104857600
            grow: false
            label: STATE
            typeUUID: 0FC63DAF-8483-4772-8E79-3D69D8477DE4
        filesystemSpec:
            type: xfs
            label: STATE
    encryption:
        provider: luks2
        keys:
            - slot: 0
              type: nodeID
    locator:
        match: volume.partition_label == "STATE"
    mount:
        targetPath: /system/state
```

### Status

Current volume status can be obtained using the following command:

```bash
$ talosctl get volumestatus
NODE         NAMESPACE   TYPE           ID                                  VERSION   TYPE        PHASE   LOCATION    SIZE
172.20.0.2   runtime     VolumeStatus   /etc/cni                            3         overlay     ready
172.20.0.2   runtime     VolumeStatus   EPHEMERAL                           6         partition   ready   /dev/vda4   5.2 GB
172.20.0.2   runtime     VolumeStatus   ETCD                                2         directory   ready
172.20.0.2   runtime     VolumeStatus   META                                3         partition   ready   /dev/vda2   1.0 MB
172.20.0.2   runtime     VolumeStatus   STATE                               6         partition   ready   /dev/vda3   105 MB
172.20.0.2   runtime     VolumeStatus   u-extra                             2         partition   ready   /dev/sda1   350 MB
172.20.0.2   runtime     VolumeStatus   u-p1                                2         partition   ready   /dev/sdb1   350 MB
172.20.0.2   runtime     VolumeStatus   u-p2                                2         partition   ready   /dev/sdb2   350 MB
```

Each volume goes through different phases during its lifecycle:

- `waiting`: the volume is waiting to be provisioned
- `missing`: all disks have been discovered, but the volume cannot be found
- `located`: the volume is found without prior provisioning
- `provisioned`: the volume has been provisioned (e.g., partitioned, resized if necessary)
- `prepared`: the encrypted volume is open
- `ready`: the volume is formatted and ready to be mounted
- `closed`: the encrypted volume is closed, and ready to be unmounted
