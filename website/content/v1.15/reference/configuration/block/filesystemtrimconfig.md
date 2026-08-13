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
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`interval` |Duration |The interval at which the filesystems are trimmed.<br><br>The trim is performed at a stable, hash-derived time within the interval, which is different<br>for each volume and each node, so that trims are spread out over time.  | |






