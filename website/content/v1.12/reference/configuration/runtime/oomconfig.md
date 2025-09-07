---
description: OOMConfig is a Out of Memory handler config document.
title: OOMConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: OOMConfig
cgroupRankingExpression: 'memory_max.hasValue() ? 0.0 : ({Besteffort: 1.0, Guaranteed: 0.0, Burstable: 0.5}[class] * double(memory_current.orValue(0u)) / double(memory_peak.orValue(0u) - memory_current.orValue(0u)))' # This expression defines how to rank cgroups for OOM handler.
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`cgroupRankingExpression` |Expression |This expression defines how to rank cgroups for OOM handler.<br><br>The cgroup with the highest rank (score) will be evicted first.<br>The expression must evaluate to a double value.  | |






