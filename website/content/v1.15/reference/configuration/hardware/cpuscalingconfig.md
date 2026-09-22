---
description: CPUScalingConfig configures Linux CPU frequency scaling.
title: CPUScalingConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: CPUScalingConfig
name: all-cores # Name of the config document.
# Selector to match the cpufreq policies to configure.
selector:
    match: "true" # The Common Expression Language (CEL) expression to match the cpufreq policy.
governor: performance # Scaling governor to set, e.g. `performance`, `powersave` or `schedutil`.

# # Energy performance preference to set, e.g. `performance`, `balance_performance`,
# energyPerformancePreference: balance_performance

# # Lower bound of the frequency window the governor may use, in kHz.
# minFrequencyKhz: 1200000

# # Upper bound of the frequency window the governor may use, in kHz.
# maxFrequencyKhz: 3300000
{{< /highlight >}}

{{< highlight yaml >}}
apiVersion: v1alpha1
kind: CPUScalingConfig
name: efficiency-cores # Name of the config document.
# Selector to match the cpufreq policies to configure.
selector:
    match: cpu.core_type == "efficiency" # The Common Expression Language (CEL) expression to match the cpufreq policy.
governor: powersave # Scaling governor to set, e.g. `performance`, `powersave` or `schedutil`.

# # Energy performance preference to set, e.g. `performance`, `balance_performance`,
# energyPerformancePreference: balance_performance

# # Lower bound of the frequency window the governor may use, in kHz.
# minFrequencyKhz: 1200000

# # Upper bound of the frequency window the governor may use, in kHz.
# maxFrequencyKhz: 3300000
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`name` |string |Name of the config document.<br><br>It is used to tell apart several scaling policies, and is reported on the<br>`CPUScalingSpec` resources the document produces.  | |
|`selector` |<a href="#CPUScalingConfig.selector">CPUScalingSelector</a> |Selector to match the cpufreq policies to configure.<br><br>If several documents match the same policy, the first one (in document order) wins.  | |
|`governor` |string |Scaling governor to set, e.g. `performance`, `powersave` or `schedutil`.<br><br>The governors a machine offers depend on its cpufreq driver, and are reported per policy<br>in `availableGovernors` of the `CPUScalingStatus` resource. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
governor: performance
{{< /highlight >}}</details> | |
|`energyPerformancePreference` |string |Energy performance preference to set, e.g. `performance`, `balance_performance`,<br>`balance_power` or `power`.<br><br>Only some drivers implement this (HWP-enabled `intel_pstate`, `amd-pstate` in active<br>mode); the values a machine offers are reported per policy in `availableEPPs` of the<br>`CPUScalingStatus` resource. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
energyPerformancePreference: balance_performance
{{< /highlight >}}</details> | |
|`minFrequencyKhz` |uint64 |Lower bound of the frequency window the governor may use, in kHz.<br><br>Left alone when not set. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
minFrequencyKhz: 1200000
{{< /highlight >}}</details> | |
|`maxFrequencyKhz` |uint64 |Upper bound of the frequency window the governor may use, in kHz.<br><br>Left alone when not set. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
maxFrequencyKhz: 3300000
{{< /highlight >}}</details> | |




## selector {#CPUScalingConfig.selector}

CPUScalingSelector selects the cpufreq policies to configure.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`match` |Expression |The Common Expression Language (CEL) expression to match the cpufreq policy.<br><br>The `cpu` variable is a cpufreq policy as reported by the `CPUScalingStatus` resource. <details><summary>Show example(s)</summary>match every policy:{{< highlight yaml >}}
match: "true"
{{< /highlight >}}match the performance cores of a hybrid CPU:{{< highlight yaml >}}
match: cpu.core_type == "performance"
{{< /highlight >}}match policies whose hardware tops out above 3 GHz:{{< highlight yaml >}}
match: cpu.cpu_info_max_frequency_khz > 3000000u
{{< /highlight >}}match policies by scaling driver:{{< highlight yaml >}}
match: cpu.driver == "intel_pstate"
{{< /highlight >}}</details> | |








