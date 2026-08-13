---
description: RegistryMirrorConfig configures an image registry mirror.
title: RegistryMirrorConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: RegistryMirrorConfig
name: registry.k8s.io # Registry name to apply the mirror configuration to.
# List of mirror endpoints for the registry.
endpoints:
    - url: https://my-private-registry.local:5000 # The URL of the registry mirror endpoint.
    - url: http://my-harbor/v2/registry-k8s.io/ # The URL of the registry mirror endpoint.
      overridePath: true # Use endpoint path as supplied, without adding `/v2/` suffix.
skipFallback: true # Skip fallback to the original registry if none of the mirrors are available
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`name` |string |Registry name to apply the mirror configuration to.<br><br>Registry name is the first segment of image identifier, with 'docker.io'<br>being default one.<br><br>A special name '*' can be used to define mirror configuration<br>that applies to all registries.  | |
|`endpoints` |<a href="#RegistryMirrorConfig.endpoints.">[]RegistryEndpoint</a> |List of mirror endpoints for the registry.<br>Mirrors will be used in the order they are specified,<br>falling back to the default registry is `skipFallback` is not set to true.  | |
|`skipFallback` |bool |Skip fallback to the original registry if none of the mirrors are available<br>or contain the requested image.  | |




## endpoints[] {#RegistryMirrorConfig.endpoints.}

RegistryEndpoint defines a registry mirror endpoint.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`url` |URL |The URL of the registry mirror endpoint. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
url: https://my-registry-mirror.local:5000
{{< /highlight >}}</details> | |
|`overridePath` |bool |Use endpoint path as supplied, without adding `/v2/` suffix.  | |








