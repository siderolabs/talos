---
description: |
    DiskSMARTConfig is a disk SMART monitoring configuration document.
    Disk SMART monitoring periodically collects SMART (Self-Monitoring, Analysis and Reporting
    Technology) health information from disks, exposed via the `SMARTStatus` resource
    (`talosctl get smart`).

    SMART collection is enabled by default; this document allows tuning the refresh interval or
    disabling it. Disks in standby are never spun up just to be probed.
title: DiskSMARTConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: DiskSMARTConfig
interval: 30m0s # The interval at which disk SMART status is refreshed.
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`enabled` |bool |Enable or disable disk SMART monitoring.<br><br>Defaults to enabled when this document is present.  | |
|`interval` |Duration |The interval at which disk SMART status is refreshed.  | |






