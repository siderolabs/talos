---
description: |
    FilesystemTrimConfig is a filesystem trim (fstrim) configuration document.
    Filesystem trim (the equivalent of the `fstrim` command) periodically discards unused blocks
    of mounted filesystems which support trimming.

    When this document is present, Talos builds a stable per-node, per-volume schedule and trims
    eligible volumes at the configured interval. If the document is absent, no automatic trimming
    is performed (unless enabled explicitly on a per-volume basis).
title: FilesystemTrimConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: FilesystemTrimConfig
interval: 168h0m0s # The interval at which the filesystems are trimmed.

# # The size of the filesystem range trimmed at once.
# chunkSize: 1GiB

# # The delay between trimming consecutive chunks.
# chunkDelay: 250ms

# # The minimum contiguous free range to discard.
# minLength: 1MiB
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`interval` |Duration |The interval at which the filesystems are trimmed.<br><br>The trim is performed at a stable, hash-derived time within the interval, which is different<br>for each volume and each node, so that trims are spread out over time.  | |
|`chunkSize` |ByteSize |The size of the filesystem range trimmed at once.<br><br>By default (or when set to zero), the whole filesystem is trimmed at once (same as the `fstrim` command).<br>Trimming a large filesystem at once issues discards for all free space back-to-back,<br>which might cause latency spikes for other workloads using the same disk.<br>Setting the chunk size splits the trim into multiple operations, each covering at most<br>the chunk size of the filesystem. When set, the chunk size must be at least 1MiB.<br><br>Size is specified in bytes, but can be expressed in human readable format, e.g. 1GiB. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
chunkSize: 1GiB
{{< /highlight >}}</details> | |
|`chunkDelay` |Duration |The delay between trimming consecutive chunks.<br><br>Only used when the chunk size is set. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
chunkDelay: 250ms
{{< /highlight >}}</details> | |
|`minLength` |ByteSize |The minimum contiguous free range to discard.<br><br>Free ranges smaller than this value are not discarded, which reduces the number of discard<br>operations at the expense of leaving small free ranges untrimmed.<br>The kernel raises the value to the discard granularity of the device.<br>The value cannot exceed 128MiB, as ext4 rejects values larger than the block group size.<br><br>Size is specified in bytes, but can be expressed in human readable format, e.g. 1MiB. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
minLength: 1MiB
{{< /highlight >}}</details> | |






