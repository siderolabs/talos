---
description: KubeExternalManifestConfig configures a Kubernetes manifest which is
    downloaded from a URL.
title: KubeExternalManifestConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: KubeExternalManifestConfig
name: example-cni # Name of manifest.
url: https://www.example.com/manifest1.yaml # Kubernetes manifest definition, via the URL to download it from.
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`name` |string |Name of manifest.  | |
|`headers` |map[string]string |Optional HTTP headers to use when downloading the manifest.  | |
|`url` |URL |Kubernetes manifest definition, via the URL to download it from.<br>Please note that Talos does not watch URL contents, and might download<br>the manifest only once, during the boot.  | |






