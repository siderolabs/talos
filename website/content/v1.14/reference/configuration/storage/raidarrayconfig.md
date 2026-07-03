---
description: |
    RAIDArrayConfig provisions a Linux MD (software RAID) array.
    Provisions a Linux software RAID (MD) array from matching disks.

    The array is exposed at `/dev/disk/by-id/md-name-<name>` and can back a
    user volume. Provisioning is additive: the array and its members are
    created but never destroyed by this document. Use `talosctl wipe md <device>`
    to remove an array.
title: RAIDArrayConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: RAIDArrayConfig
name: talos # Array name, stamped into the md metadata.
level: raid1 # RAID level.
# The provisioning describes how the RAID arrays are provisioned.
provisioning:
    # The volume selector describes how the members of RAID arrays are selected.
    volumeSelector:
        match: disk.transport == "nvme" && disk.size > 100u * GiB # CEL expression matching the member volumes of the array.
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`name` |string |Array name, stamped into the md metadata.<br><br>Must be 1-32 chars: ASCII letters, digits, hyphens, underscores.<br>Exposed as `/dev/disk/by-id/md-name-<name>`.  | |
|`level` |MDLevel |RAID level.  |`raid1`<br /> |
|`provisioning` |<a href="#RAIDArrayConfig.provisioning">RAIDProvisioningSpec</a> |The provisioning describes how the RAID arrays are provisioned.  | |




## provisioning {#RAIDArrayConfig.provisioning}

RAIDProvisioningSpec describes how the RAID arrays are provisioned.





| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`volumeSelector` |<a href="#RAIDArrayConfig.provisioning.volumeSelector">RAIDVolumeSelector</a> |The volume selector describes how the members of RAID arrays are selected.  | |




### volumeSelector {#RAIDArrayConfig.provisioning.volumeSelector}

RAIDVolumeSelector matches member disks with CEL.





| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`match` |Expression |CEL expression matching the member volumes of the array.<br><br>Evaluated against each discovered volume with the `volume` variable;<br>the `disk` variable is bound for whole disks (empty for partitions), so<br>both whole disks and partitions can be selected. The system disk and<br>its partitions are never eligible. <details><summary>Show example(s)</summary>match NVMe disks larger than 100 GiB:{{< highlight yaml >}}
match: disk.transport == "nvme" && disk.size > 100u * GiB
{{< /highlight >}}</details> | |










