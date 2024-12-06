---
description: FilesystemScrubConfig is a filesystem scrubbing config document.
title: FilesystemScrubConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: FilesystemScrubConfig
name: var # Name of the config document.
mountpoint: /var # Mountpoint of the filesystem to be scrubbed.
period: 168h0m0s # Period for running the scrub task for this filesystem.
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`name` |string |Name of the config document.  | |
|`mountpoint` |string |Mountpoint of the filesystem to be scrubbed. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
mountpoint: /var
{{< /highlight >}}</details> | |
|`period` |Duration |<details><summary>Period for running the scrub task for this filesystem.</summary><br />The first run is scheduled randomly within this period from the boot time, later ones follow after the full period.<br /><br />Default value is 1 week, minimum value is 10 seconds.</details>  | |






