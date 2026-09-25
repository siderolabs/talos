---
description: |
    ExistingVolumeConfig is an existing volume configuration document.
    Existing volumes allow to mount partitions (or whole disks) that were created
    outside of Talos. Volume will be mounted under `/var/mnt/<name>`.
    The name must not be taken by a user or external volume.
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
|`mount` |<a href="#ExistingVolumeConfig.mount">ExistingMountSpec</a> |The mount describes additional mount options.  | |
|`trim` |<a href="#ExistingVolumeConfig.trim">TrimConfig</a> |The trim describes the per-volume filesystem trim (fstrim) configuration.  | |
|`scrub` |<a href="#ExistingVolumeConfig.scrub">ScrubConfig</a> |The scrub describes the per-volume filesystem scrub configuration.  | |




## discovery {#ExistingVolumeConfig.discovery}

VolumeDiscoverySpec describes how the volume is discovered.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`volumeSelector` |<a href="#ExistingVolumeConfig.discovery.volumeSelector">VolumeSelector</a> |The volume selector expression.  | |




### volumeSelector {#ExistingVolumeConfig.discovery.volumeSelector}

VolumeSelector selects an existing volume.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`match` |Expression |The Common Expression Language (CEL) expression to match the volume. <details><summary>Show example(s)</summary>match volumes with partition label MY-DATA:{{< highlight yaml >}}
match: volume.partition_label == "MY-DATA"
{{< /highlight >}}match xfs volume on disk with serial 'SERIAL123':{{< highlight yaml >}}
match: volume.name == "xfs" && disk.serial == "SERIAL123"
{{< /highlight >}}</details> | |








## mount {#ExistingVolumeConfig.mount}

ExistingMountSpec describes how the volume is mounted.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`readOnly` |bool |Mount the volume read-only.  | |
|`disableAccessTime` |bool |If true, disable file access time updates.  | |
|`secure` |bool |Enable secure mount options (nosuid, nodev, noexec).<br><br>Defaults to true for better security.  | |






## trim {#ExistingVolumeConfig.trim}

TrimConfig describes per-volume filesystem trim (fstrim) configuration.

It overrides the global FilesystemTrimConfig for the volume.





| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`enabled` |bool |Enable or disable trimming for this volume.<br><br>If not set, trimming is enabled when the global FilesystemTrimConfig is present.  | |
|`interval` |Duration |The interval at which the volume is trimmed, overriding the global trim interval.  | |
|`chunkSize` |ByteSize |The size of the filesystem range trimmed at once, overriding the global chunk size.<br><br>Setting it explicitly to zero trims the whole filesystem at once.<br>When set to a non-zero value, the chunk size must be at least 1MiB.<br><br>Size is specified in bytes, but can be expressed in human readable format, e.g. 1GiB. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
chunkSize: 1GiB
{{< /highlight >}}</details> | |
|`chunkDelay` |Duration |The delay between trimming consecutive chunks, overriding the global chunk delay.<br><br>Setting it explicitly to zero trims the chunks back-to-back.  | |
|`minLength` |ByteSize |The minimum contiguous free range to discard, overriding the global minimum length.<br><br>The value cannot exceed 128MiB, as ext4 rejects values larger than the block group size.<br><br>Size is specified in bytes, but can be expressed in human readable format, e.g. 1MiB. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
minLength: 1MiB
{{< /highlight >}}</details> | |






## scrub {#ExistingVolumeConfig.scrub}

ScrubConfig describes per-volume filesystem scrub configuration.

It overrides the global FilesystemScrubConfig for the volume.





| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`enabled` |bool |Enable or disable scrubbing for this volume.<br><br>If not set, scrubbing is enabled by default when scrub section is present.  | |
|`interval` |Duration |The interval at which the volume is scrubbed, overriding the global scrub interval.  | |








