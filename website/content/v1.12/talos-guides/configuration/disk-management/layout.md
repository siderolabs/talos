---
title: "Disk Layout"
description: "Guide on disk layout, observing discovered disks and volumes."
weight: 10
---

Talos Linux provides tools to observe available disks and volumes on the machine.

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
| EFI (boot)  | META      | STATE    | EPHEMERAL  | csi-data  | local-storage | << Unallocated Space >> |
| [~1GB]      | [~1MB]    | [~100MB] | [~200GB]   | [~100GB]  | [~200GB]      | [~500GB]                |
+-------------+-----------+----------+------------+-----------+---------------+-------------------------+
```

In this layout, the `EPHEMERAL` partition was limited to 200GB, and two additional partitions were created for `csi-data` and `local-storage`.

### Multiple Disk Layout

```text
+---------------------------------------------------------------------------------------+
| Physical Disk 1 (1TB)                                                                 |
+=============+===========+==========+============+===========+=========================+
| EFI (boot)  | META      | STATE    | EPHEMERAL  | csi-data  | << Unallocated Space >> |
| [~1GB]      | [~1MB]    | [~100MB] | [~500GB]   | [~100GB]  | [~400GB]                |
+-------------+-----------+----------+------------+-----------+-------------------------+
| Physical Disk 2 (1TB)                                                                 |
+===============+=======================================================================+
| local-storage | << Unallocated Space >>                                               |
| [~500GB]      |  [~500GB]                                                             |
+---------------+-----------------------------------------------------------------------+
```

In this layout, the `EPHEMERAL` partition was limited to 500GB, and two additional partitions were created for `csi-data` and `local-storage`,
and they were created on different disks.
