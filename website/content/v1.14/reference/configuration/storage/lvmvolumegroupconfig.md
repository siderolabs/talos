---
description: |
    LVMVolumeGroupConfig is an LVM volume group config document.
    Defines volume group and selector for backing disks.
title: LVMVolumeGroupConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: LVMVolumeGroupConfig
name: vg-pool # Volume group name.
# The provisioning describes how the Physical Volumes are provisioned.
provisioning:
    # Matches disks to initialize as physical volumes.
    volumeSelector:
        match: volume.partition_label.startsWith("r-lvm") # CEL expression matching a disk or partition to use as a physical volume.
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`name` |string |Volume group name.<br><br>Must be 1-63 chars: ASCII letters, digits, hyphens, underscores.  | |
|`provisioning` |<a href="#LVMVolumeGroupConfig.provisioning">ProvisioningSpec</a> |The provisioning describes how the Physical Volumes are provisioned.  | |




## provisioning {#LVMVolumeGroupConfig.provisioning}

ProvisioningSpec describes how the Physical Volumes are provisioned.





| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`volumeSelector` |<a href="#LVMVolumeGroupConfig.provisioning.volumeSelector">LVMVolumeSelectorSpec</a> |Matches disks to initialize as physical volumes.  | |




### volumeSelector {#LVMVolumeGroupConfig.provisioning.volumeSelector}

LVMVolumeSelectorSpec matches disks with CEL.





| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`match` |Expression |CEL expression matching a disk or partition to use as a physical volume.<br><br>The expression is evaluated against each discovered volume with the<br>`volume` variable (the discovered volume) and, for whole disks, the<br>`disk` variable. Partitions (e.g. raw volumes) can be matched by their<br>partition label via `volume.partition_label`. <details><summary>Show example(s)</summary>match raw volume partitions labeled r-lvm*:{{< highlight yaml >}}
match: volume.partition_label.startsWith("r-lvm")
{{< /highlight >}}</details> | |










