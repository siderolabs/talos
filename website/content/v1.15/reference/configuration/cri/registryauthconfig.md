---
description: RegistryAuthConfig configures authentication for a registry endpoint.
title: RegistryAuthConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: RegistryAuthConfig
name: my-private-registry.local:5000 # Registry endpoint to apply the authentication configuration to.
username: my-username # Username/password authentication.
password: my-password # Username/password authentication.
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`name` |string |Registry endpoint to apply the authentication configuration to.<br><br>Registry endpoint is the hostname part of the endpoint URL,<br>e.g. 'my-mirror.local:5000' for 'https://my-mirror.local:5000/v2/'.<br><br>The authentication configuration will apply to all image pulls for this<br>registry endpoint, by Talos or any Kubernetes workloads.  | |
|`username` |string |Username/password authentication.  | |
|`password` |string |Username/password authentication.  | |
|`auth` |string |Raw authentication string.  | |
|`identityToken` |string |Identity token authentication.  | |






