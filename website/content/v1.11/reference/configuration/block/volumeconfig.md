---
description: |
    VolumeConfig is a system volume configuration document.
    Note: at the moment, only `EPHEMERAL` and `IMAGE-CACHE` system volumes are supported.
title: VolumeConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: VolumeConfig
name: EPHEMERAL # Name of the volume.
# The provisioning describes how the volume is provisioned.
provisioning:
    # The disk selector expression.
    diskSelector:
        match: disk.transport == "nvme" # The Common Expression Language (CEL) expression to match the disk.
    maxSize: 50GiB # The maximum size of the volume, if not specified the volume can grow to the size of the

    # # The minimum size of the volume.
    # minSize: 2.5GiB
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`name` |string |Name of the volume.  | |
|`provisioning` |<a href="#VolumeConfig.provisioning">ProvisioningSpec</a> |The provisioning describes how the volume is provisioned.  | |




## provisioning {#VolumeConfig.provisioning}

ProvisioningSpec describes how the volume is provisioned.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`diskSelector` |<a href="#VolumeConfig.provisioning.diskSelector">DiskSelector</a> |The disk selector expression.  | |
|`grow` |bool |Should the volume grow to the size of the disk (if possible).  | |
|`minSize` |ByteSize |The minimum size of the volume.<br><br>Size is specified in bytes, but can be expressed in human readable format, e.g. 100MB. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
minSize: 2.5GiB
{{< /highlight >}}</details> | |
|`maxSize` |ByteSize |The maximum size of the volume, if not specified the volume can grow to the size of the<br>disk.<br><br>Size is specified in bytes, but can be expressed in human readable format, e.g. 100MB. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
maxSize: 50GiB
{{< /highlight >}}</details> | |




### diskSelector {#VolumeConfig.provisioning.diskSelector}

DiskSelector selects a disk for the volume.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`match` |Expression |The Common Expression Language (CEL) expression to match the disk. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
match: disk.size > 120u * GB && disk.size < 1u * TB
{{< /highlight >}}{{< highlight yaml >}}
match: disk.transport == "sata" && !disk.rotational && !system_disk
{{< /highlight >}}</details> | |










