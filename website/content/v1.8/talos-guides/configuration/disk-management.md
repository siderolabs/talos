---
title: "Disk Management"
description: "Guide on managing disks"
---

Talos Linux version 1.8.0 introduces a new backend for managing system and user disks.
The machine configuration changes required are minimal, and the new backend is fully compatible with the existing machine configuration.

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
```

## Discovering Volumes

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

## Volume Management

Talos Linux implements disk management through the concept of volumes.
A volume represents a provisioned, located, mounted, or unmounted entity, such as a disk, partition, or `tmpfs` filesystem.

The configuration of volumes is defined using the `VolumeConfig` resource, while the current state of volumes is stored in the `VolumeStatus` resource.

### Configuration

The volume configuration is managed by Talos Linux based on machine configuration.
To see configured volumes, use the following command:

```bash
$ talosctl get volumeconfigs
NODE         NAMESPACE   TYPE           ID                                            VERSION
172.20.0.5   runtime     VolumeConfig   /dev/disk/by-id/ata-QEMU_HARDDISK_QM00001-1   2
172.20.0.5   runtime     VolumeConfig   /dev/disk/by-id/ata-QEMU_HARDDISK_QM00001-2   2
172.20.0.5   runtime     VolumeConfig   /dev/disk/by-id/ata-QEMU_HARDDISK_QM00003-1   2
172.20.0.5   runtime     VolumeConfig   EPHEMERAL                                     2
172.20.0.5   runtime     VolumeConfig   META                                          2
172.20.0.5   runtime     VolumeConfig   STATE                                         4
```

In the provided output, the volumes `EPHEMERAL`, `META`, and `STATE` are system volumes managed by Talos, while the remaining volumes are based on the machine configuration for `machine.disks`.

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
NODE         NAMESPACE   TYPE           ID                                            VERSION   PHASE   LOCATION         SIZE
172.20.0.5   runtime     VolumeStatus   /dev/disk/by-id/ata-QEMU_HARDDISK_QM00001-1   1         ready   /dev/sdc1        957 MB
172.20.0.5   runtime     VolumeStatus   /dev/disk/by-id/ata-QEMU_HARDDISK_QM00001-2   1         ready   /dev/sdc2        957 MB
172.20.0.5   runtime     VolumeStatus   /dev/disk/by-id/ata-QEMU_HARDDISK_QM00003-1   1         ready   /dev/sdd1        957 MB
172.20.0.5   runtime     VolumeStatus   EPHEMERAL                                     1         ready   /dev/nvme0n1p1   10 GB
172.20.0.5   runtime     VolumeStatus   META                                          2         ready   /dev/vda4        524 kB
172.20.0.5   runtime     VolumeStatus   STATE                                         2         ready   /dev/vda5        92 MB
```

Each volume goes through different phases during its lifecycle:

- `waiting`: the volume is waiting to be provisioned
- `missing`: all disks have been discovered, but the volume cannot be found
- `located`: the volume is found without prior provisioning
- `provisioned`: the volume has been provisioned (e.g., partitioned, resized if necessary)
- `prepared`: the encrypted volume is open
- `ready`: the volume is formatted and ready to be mounted
- `closed`: the encrypted volume is closed

## Machine Configuration

> Note: In Talos Linux 1.8, only `EPHEMERAL` system volume configuration can be managed through the machine configuration.
>
> Note: The volume configuration in the machine configuration is only applied when the volume has not been provisioned yet.
> So applying changes after the initial provisioning will not have any effect.

To configure the `EPHEMERAL` (`/var`) volume, add the following [document]({{< relref "../../reference/configuration/block/volumeconfig" >}}) to the machine configuration:

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

### Disk Selector

The `diskSelector` field is utilized to choose the disk where the volume will be provisioned.
It is a [Common Expression Language (CEL)](https://cel.dev/) expression that evaluates against the available disks.
The volume will be provisioned on the first disk that matches the expression and has sufficient free space for the volume.

The expression is evaluated in the following context:

- `system_disk` (`bool`) - indicates if the disk is the system disk
- `disk` (`Disks.block.talos.dev`) - the disk resource being evaluated

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

### Minimum and Maximum Size

The `minSize` and `maxSize` fields define the minimum and maximum size of the volume, respectively.
Talos Linux will always ensure that the volume is at least `minSize` in size and will not exceed `maxSize`.
If `maxSize` is not set, the volume will grow to utilize the maximum available space on the disk.

If `grow` is set to `true`, the volume will automatically grow to utilize the maximum available space on the disk on each boot.

Setting `minSize` might influence disk selection - if the disk does not have enough free space to satisfy the minimum size requirement, it will not be selected for provisioning.
