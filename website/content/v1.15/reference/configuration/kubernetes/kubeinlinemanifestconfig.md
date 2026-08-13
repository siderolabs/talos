---
description: KubeInlineManifestConfig configures a Kubernetes manifest to be applied
    to the cluster.
title: KubeInlineManifestConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: KubeInlineManifestConfig
name: namespace-ci # Name of manifest.
manifest: |- # Kubernetes manifest definition, it is supplied as a raw string.
    apiVersion: v1
    kind: Namespace
    metadata:
      name: ci
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`name` |string |Name of manifest.  | |
|`manifest` |string |Kubernetes manifest definition, it is supplied as a raw string.<br>It might contain a set of YAML documents separated by `---`.<br>The format matches what can be supplied as `kubectl apply -f <file>`.  | |






