---
description: |
    DiskSMARTConfig is a disk SMART monitoring configuration document.
    Disk SMART monitoring periodically collects SMART (Self-Monitoring, Analysis and Reporting
    Technology) health information from disks, exposed via the `SMARTStatus` resource
    (`talosctl get smart`).

    SMART collection is performed whenever this document is present in the machine configuration;
    remove the document to disable it. Disks in standby are never spun up just to be probed.
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
|`interval` |Duration |The interval at which disk SMART status is refreshed.  | |






