---
description: ImageCacheConfig configures Image Cache feature.
title: ImageCacheConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: ImageCacheConfig
# Local (to the machine) image cache configuration.
local:
    enabled: true # Is the local image cache enabled.
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`local` |<a href="#ImageCacheConfig.local">LocalImageCacheConfig</a> |Local (to the machine) image cache configuration.  | |




## local {#ImageCacheConfig.local}

LocalImageCacheConfig configures local image cache.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`enabled` |bool |Is the local image cache enabled.  | |








