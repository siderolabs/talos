---
description: SysfsConfig configures Linux kernel sysfs values.
title: SysfsConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: SysfsConfig
# Used to configure the machine's sysfs (kernel attributes under `/sys`).
params:
    devices.system.cpu.cpu0.cpufreq.scaling_governor: performance
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`params` |map[string]string |Used to configure the machine's sysfs (kernel attributes under `/sys`).<br>Values from this document are merged with the deprecated v1alpha1 machine.sysfs values (if set),<br>with this document taking precedence on key conflicts. <details><summary>Show example(s)</summary>SysfsConfig usage example.:{{< highlight yaml >}}
params:
    devices.system.cpu.cpu0.cpufreq.scaling_governor: performance
{{< /highlight >}}</details> | |






