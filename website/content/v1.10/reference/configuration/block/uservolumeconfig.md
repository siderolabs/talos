---
description: |
    UserVolumeConfig is a user volume configuration document.
    User volume is automatically allocated as a partition on the specified disk
    and mounted under `/var/mnt/<name>`.
    The partition label is automatically generated as `u-<name>`.
title: UserVolumeConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: UserVolumeConfig
name: ceph-data # Name of the volume.
# The provisioning describes how the volume is provisioned.
provisioning:
    # The disk selector expression.
    diskSelector:
        match: disk.transport == "nvme" # The Common Expression Language (CEL) expression to match the disk.
    maxSize: 50GiB # The maximum size of the volume, if not specified the volume can grow to the size of the

    # # The minimum size of the volume.
    # minSize: 2.5GiB
# The filesystem describes how the volume is formatted.
filesystem:
    type: xfs # Filesystem type. Default is `xfs`.
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`name` |string |<details><summary>Name of the volume.</summary><br />Name might be between 1 and 34 characters long and can only contain:<br />lowercase and uppercase ASCII letters, digits, and hyphens.</details>  | |
|`provisioning` |<a href="#UserVolumeConfig.provisioning">ProvisioningSpec</a> |The provisioning describes how the volume is provisioned.  | |
|`filesystem` |<a href="#UserVolumeConfig.filesystem">FilesystemSpec</a> |The filesystem describes how the volume is formatted.  | |




## provisioning {#UserVolumeConfig.provisioning}

ProvisioningSpec describes how the volume is provisioned.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`diskSelector` |<a href="#UserVolumeConfig.provisioning.diskSelector">DiskSelector</a> |The disk selector expression.  | |
|`grow` |bool |Should the volume grow to the size of the disk (if possible).  | |
|`minSize` |ByteSize |<details><summary>The minimum size of the volume.</summary><br />Size is specified in bytes, but can be expressed in human readable format, e.g. 100MB.</details> <details><summary>Show example(s)</summary>{{< highlight yaml >}}
minSize: 2.5GiB
{{< /highlight >}}</details> | |
|`maxSize` |ByteSize |<details><summary>The maximum size of the volume, if not specified the volume can grow to the size of the</summary>disk.<br /><br />Size is specified in bytes, but can be expressed in human readable format, e.g. 100MB.</details> <details><summary>Show example(s)</summary>{{< highlight yaml >}}
maxSize: 50GiB
{{< /highlight >}}</details> | |




### diskSelector {#UserVolumeConfig.provisioning.diskSelector}

DiskSelector selects a disk for the volume.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`match` |Expression |The Common Expression Language (CEL) expression to match the disk. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
match: disk.size > 120u * GB && disk.size < 1u * TB
{{< /highlight >}}{{< highlight yaml >}}
match: disk.transport == "sata" && !disk.rotational && !system_disk
{{< /highlight >}}</details> | |








## filesystem {#UserVolumeConfig.filesystem}

FilesystemSpec configures the filesystem for the volume.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`type` |FilesystemType |Filesystem type. Default is `xfs`.  |`ext4`<br />`xfs`<br /> |








