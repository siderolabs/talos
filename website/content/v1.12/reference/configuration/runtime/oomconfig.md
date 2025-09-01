---
description: OOMConfig is a Out of Memory handler config document.
title: OOMConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: OOMConfig
triggerExpression: memory_full_avg10 > 12.0 && time_since_trigger > duration("500ms") # This expression defines when to trigger OOM action.
cgroupRankingExpression: 'memory_max.hasValue() ? 0.0 : ({Besteffort: 1.0, Burstable: 0.5, Guaranteed: 0.0, Podruntime: 0.0, System: 0.0}[class] * double(memory_current.orValue(0u)) / double(memory_peak.orValue(0u) - memory_current.orValue(0u)))' # This expression defines how to rank cgroups for OOM handler.
sampleInterval: 100ms # How often should the trigger expression be evaluated.
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`triggerExpression` |Expression |This expression defines when to trigger OOM action.<br><br>The expression must evaluate to a boolean value.<br>If the expression returns true, then OOM ranking and killing will be handled.  | |
|`cgroupRankingExpression` |Expression |This expression defines how to rank cgroups for OOM handler.<br><br>The cgroup with the highest rank (score) will be evicted first.<br>The expression must evaluate to a double value.  | |
|`sampleInterval` |Duration |How often should the trigger expression be evaluated.<br><br>This interval determines how often should the OOM controller<br>check for the OOM condition using the provided expression.<br>Adjusting it can help tune the reactivity of the OOM handler.  | |






