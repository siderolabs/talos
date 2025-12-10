---
description: WatchdogTimerConfig is a watchdog timer config document.
title: WatchdogTimerConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: WatchdogTimerConfig
device: /dev/watchdog0 # Path to the watchdog device.
timeout: 2m0s # Timeout for the watchdog.
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`device` |string |Path to the watchdog device. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
device: /dev/watchdog0
{{< /highlight >}}</details> | |
|`timeout` |Duration |Timeout for the watchdog.<br><br>If Talos is unresponsive for this duration, the watchdog will reset the system.<br><br>Default value is 1 minute, minimum value is 10 seconds.  | |






