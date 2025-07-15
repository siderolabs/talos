---
description: |
    ExistingVolumeConfig is an existing volume configuration document.
    Existing volumes allow to mount partitions (or whole disks) that were created
    outside of Talos. Volume will be mounted under `/var/mnt/<name>`.
    The existing volume config name should not conflict with user volume names.
title: ExistingVolumeConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: ExistingVolumeConfig
name: my-existing-volume # Name of the volume.
# The discovery describes how to find a volume.
discovery:
    # The volume selector expression.
    volumeSelector:
        match: volume.partition_label == "MY-DATA" # The Common Expression Language (CEL) expression to match the volume.
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`name` |string |Name of the volume.<br><br>Name can only contain:<br>lowercase and uppercase ASCII letters, digits, and hyphens.  | |
|`discovery` |<a href="#ExistingVolumeConfig.discovery">VolumeDiscoverySpec</a> |The discovery describes how to find a volume.  | |
|`mount` |<a href="#ExistingVolumeConfig.mount">MountSpec</a> |The mount describes additional mount options.  | |




## discovery {#ExistingVolumeConfig.discovery}

VolumeDiscoverySpec describes how the volume is discovered.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`volumeSelector` |<a href="#ExistingVolumeConfig.discovery.volumeSelector">VolumeSelector</a> |The volume selector expression.  | |




### volumeSelector {#ExistingVolumeConfig.discovery.volumeSelector}

VolumeSelector selects an existing volume.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`match` |Expression |The Common Expression Language (CEL) expression to match the volume. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
match: volume.partition_label == "MY-DATA"
{{< /highlight >}}{{< highlight yaml >}}
match: volume.name == "xfs" && disk.serial == "SERIAL123"
{{< /highlight >}}</details> | |








## mount {#ExistingVolumeConfig.mount}

MountSpec describes how the volume is mounted.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`readOnly` |bool |Mount the volume read-only.  | |








