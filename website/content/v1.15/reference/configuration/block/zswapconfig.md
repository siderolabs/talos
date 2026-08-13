---
description: |
    ZswapConfig is a zswap (compressed memory) configuration document.
    When zswap is enabled, Linux kernel compresses pages that would otherwise be swapped out to disk.
    The compressed pages are stored in a memory pool, which is used to avoid writing to disk
    when the system is under memory pressure.
title: ZswapConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: ZswapConfig
maxPoolPercent: 25 # The maximum percent of memory that zswap can use.
shrinkerEnabled: true # Enable the shrinker feature: kernel might move
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`maxPoolPercent` |int |The maximum percent of memory that zswap can use.<br>This is a percentage of the total system memory.<br>The value must be between 0 and 100.  | |
|`shrinkerEnabled` |bool |Enable the shrinker feature: kernel might move<br>cold pages from zswap to swap device to free up memory<br>for other use cases.  | |






