---
title: "Common Configuration"
description: "Common elements of volume configuration."
weight: 100
---

Several configuration documents share common elements for configuring volumes in Talos Linux:

* [`VolumeConfig`]({{< relref "../../../reference/configuration/block/volumeconfig" >}})
* [`UserVolumeConfig`]({{< relref "../../../reference/configuration/block/uservolumeconfig" >}})
* [`RawVolumeConfig`]({{< relref "../../../reference/configuration/block/rawvolumeconfig" >}})
* [`SwapVolumeConfig`]({{< relref "../../../reference/configuration/block/swapvolumeconfig" >}})

## Disk Selector

The `diskSelector` field is utilized to choose the disk where the volume will be provisioned.
It is a [Common Expression Language (CEL)](https://cel.dev/) expression that evaluates against the available disks.
The volume will be provisioned on the first disk that matches the expression and has sufficient free space for the volume.

The expression is evaluated in the following context:

* `system_disk` (`bool`) - indicates if the disk is the system disk
* `disk` (`Disks.block.talos.dev`) - the disk resource being evaluated

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

* `KiB`, `MiB`, `GiB`, `TiB`, `PiB`, `EiB` - binary size multipliers (1024)
* `kB`, `MB`, `GB`, `TB`, `PB`, `EB` - decimal size multipliers (1000)

The disk expression is evaluated against each available disk, and the expression should either return `true` or `false`.
If the expression returns `true`, the disk is selected for provisioning.

> Note: In CEL, signed and unsigned integers are not interchangeable.
> Disk sizes are represented as unsigned integers, so suffix `u` should be used in constants to avoid type mismatch, e.g. `disk.size > 10u * GiB`.

Examples of disk selector expressions:

* `disk.transport == 'nvme'`: select the NVMe disks only
* `disk.transport == 'scsi' && disk.size < 2u * TiB`: select SCSI disks smaller than 2 TiB
* `disk.serial.startsWith('deadbeef') && !cdrom`: select disks with serial number starting with `deadbeef` and not of CD-ROM type
* `'/dev/disk/by-path/pci-0000:00:1f.2-ata-1' in disk.symlinks`: select disks with a specific stable symlink

## Minimum and Maximum Size

The `minSize` and `maxSize` fields define the minimum and maximum size of the volume, respectively.
Talos Linux will always ensure that the volume is at least `minSize` in size and will not exceed `maxSize`.
If `maxSize` is not set, the volume will grow to utilize the maximum available space on the disk.

If `grow` is set to `true`, the volume will automatically grow to utilize the maximum available space on the disk on each boot.

Setting `minSize` might influence disk selection - if the disk does not have enough free space to satisfy the minimum size requirement, it will not be selected for provisioning.
