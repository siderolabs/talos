---
description: |
    FilesystemScrubConfig is a filesystem scrub configuration document.
    Filesystem scrub periodically checks mounted filesystems which support online scrubbing
    (currently XFS, via `xfs_scrub`) for metadata errors.

    Scrubbing is enabled by default with a interval of one week; this document adjusts the default
    interval or disables scrubbing globally. Individual volumes can override the global settings
    via the `scrub` section of the volume configuration.

    Each volume is scrubbed at a stable, hash-derived time within the interval, which is different
    for each volume and each node, so that scrubs are spread out over time.
title: FilesystemScrubConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: FilesystemScrubConfig
interval: 168h0m0s # The interval at which the filesystems are scrubbed.
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`enabled` |bool |Enable or disable periodic filesystem scrubbing.<br><br>If not set, scrubbing is enabled by default.  | |
|`interval` |Duration |The interval at which the filesystems are scrubbed.<br><br>Default value is 1 week, minimum value is 10 seconds.  | |






