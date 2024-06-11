---
title: "Disk Management"
description: "Guide on managing disks"
---

Talos Linux in version 1.8.0 provides a new backend to manage system and user disks.
The machine configuration changes are minimal, and the new backend is fully compatible with the existing machine configuration.

## Listing Disks

To list all disks (block devices) available on the machine, use the following command:

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

To get details about a specific disk, use the following command:

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

Talos Linux watches all block devices and partitions on the machine.
The information about all block devices, partitions, their type can be found in the `DiscoveredVolume` resource:

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

Talos Linux automatically detects the filesystem type, GPT partition tables.
The following filesystem types are detected at the moment:

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

The discovered volumes might include Talos-managed volumes, and also any other volumes that are present on the machine (e.g. Ceph volumes).

## Volume Management

Talos Linux disk management is based on the volume concept: a volume is something that can be provisioned,
located, mounted and unmounted.
A volume can be a disk, a partition, a `tmpfs` filesystem, etc.

Volume configuration is described in the `VolumeConfig` resource, while the actual volume state is stored in the `VolumeStatus` resource.

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

In the output above, `EPHEMERAL`, `META`, and `STATE` are Talos-managed system volumes, while the other volumes are based on the machine configuration for `machine.disks`.

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

Current volume status can be checked using the following command:

```bash
$ talosctl get volumestatus
NODE         NAMESPACE   TYPE           ID                                            VERSION   PHASE   LOCATION
172.20.0.5   runtime     VolumeStatus   /dev/disk/by-id/ata-QEMU_HARDDISK_QM00001-1   1         ready   /dev/sdc1
172.20.0.5   runtime     VolumeStatus   /dev/disk/by-id/ata-QEMU_HARDDISK_QM00001-2   1         ready   /dev/sdc2
172.20.0.5   runtime     VolumeStatus   /dev/disk/by-id/ata-QEMU_HARDDISK_QM00003-1   1         ready   /dev/sdd1
172.20.0.5   runtime     VolumeStatus   EPHEMERAL                                     1         ready   /dev/vda6
172.20.0.5   runtime     VolumeStatus   META                                          2         ready   /dev/vda4
172.20.0.5   runtime     VolumeStatus   STATE                                         3         ready   /dev/vda5
```

Each volume transitions through different phases during its lifecycle:

- `waiting`: the volume is waiting for provisioning
- `missing`: all disks were discovered, but the volume is not found
- `located`: the volume is found without prior provisioning
- `provisioned`: the volume is provisioned (e.g. partitioned, grown as needed)
- `prepared`: the encrypted volume is open
- `ready`: the volume is formatted, ready to be mounted
- `closed`: the encrypted volume is closed
