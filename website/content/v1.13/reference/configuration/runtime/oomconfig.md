---
description: OOMConfig is a Out of Memory handler config document.
title: OOMConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: OOMConfig
triggerExpression: |- # This expression defines when to trigger OOM action.
    multiply_qos_vectors(d_qos_memory_full_total, {System: 8.0, Podruntime: 4.0}) > 3000.0 ||
    memory_full_avg10 > 75.0 && time_since_trigger > duration("10s")
cgroupRankingExpression: 'memory_max.hasValue() ? 0.0 : ({Besteffort: 1.0, Burstable: 0.5, Guaranteed: 0.0, Podruntime: 0.0, System: 0.0}[class] * double(memory_current.orValue(0u)))' # This expression defines how to rank cgroups for OOM handler.
sampleInterval: 100ms # How often should the trigger expression be evaluated.
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`triggerExpression` |Expression |This expression defines when to trigger OOM action.<br><br>The expression must evaluate to a boolean value.<br>If the expression returns true, then OOM ranking and killing will be handled.<br><br>This expression receives the following parameters:<br>- memory_{some,full}_{avg10,avg60,avg300,total} - double, representing PSI values<br>- time_since_trigger - duration since the last OOM handler trigger event  | |
|`cgroupRankingExpression` |Expression |This expression defines how to rank cgroups for OOM handler.<br><br>The cgroup with the highest rank (score) will be evicted first.<br>The expression must evaluate to a double value.<br><br>This expression receives the following parameters:<br>- memory_max - Optional<uint> - in bytes<br>- memory_current - Optional<uint> - in bytes<br>- memory_peak - Optional<uint> - in bytes<br>- path - string, path to the cgroup<br>- class - int. This represents cgroup QoS class, and matches one of the constants, which are also provided: Besteffort, Burstable, Guaranteed, Podruntime, System  | |
|`sampleInterval` |Duration |How often should the trigger expression be evaluated.<br><br>This interval determines how often should the OOM controller<br>check for the OOM condition using the provided expression.<br>Adjusting it can help tune the reactivity of the OOM handler.  | |






