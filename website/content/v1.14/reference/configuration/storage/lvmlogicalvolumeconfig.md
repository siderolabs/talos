---
description: |
    LVMLogicalVolumeConfig is an LVM logical volume config document.
    Defines a logical volume provisioned inside a volume group.
title: LVMLogicalVolumeConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: LVMLogicalVolumeConfig
name: lv-data # Logical volume name.
type: linear # Logical volume layout.
# Describes how the logical volume is provisioned.
provisioning:
    volumeGroup: vg-pool # Name of the volume group that backs the logical volume.
    maxSize: 50GiB # The maximum size of the volume.
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`name` |string |Logical volume name.<br><br>Must be 1-63 chars: ASCII letters, digits, hyphens, underscores.  | |
|`type` |LVMLogicalVolumeType |Logical volume layout.  |`linear`<br />`raid0`<br />`raid1`<br />`raid10`<br /> |
|`mirrors` |uint32 |Number of mirror copies for `raid1` / `raid10` layouts.<br><br>Defaults to 1 (a two-way mirror) when unset. Not valid for `linear`<br>or `raid0`.  | |
|`stripes` |uint32 |Number of stripes for `raid0` / `raid10` layouts.<br><br>Defaults to all available physical volumes when unset. Must be at<br>least 2. Not valid for `linear` or `raid1`.  | |
|`provisioning` |<a href="#LVMLogicalVolumeConfig.provisioning">LVMLogicalVolumeProvisioningSpec</a> |Describes how the logical volume is provisioned.  | |




## provisioning {#LVMLogicalVolumeConfig.provisioning}

LVMLogicalVolumeProvisioningSpec describes how an LV is provisioned.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`volumeGroup` |string |Name of the volume group that backs the logical volume.  | |
|`minSize` |ByteSize |The minimum size of the volume.<br><br>Size is specified in bytes, but can be expressed in human readable format, e.g. 100MB.  | |
|`maxSize` |Size |The maximum size of the volume.<br><br>Size is specified in bytes or in percents of the volume group.<br>It can be expressed in human readable format, e.g. 100MB or 80%.  | |








