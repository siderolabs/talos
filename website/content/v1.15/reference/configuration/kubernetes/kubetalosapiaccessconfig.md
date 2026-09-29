---
description: KubeTalosAPIAccessConfig configures access to Talos API from Kubernetes
    pods via service accounts.
title: KubeTalosAPIAccessConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: KubeTalosAPIAccessConfig
# The list of Talos API roles which can be granted for access from Kubernetes pods.
allowedRoles:
    - os:reader
# The list of Kubernetes namespaces Talos API access is available from.
allowedKubernetesNamespaces:
    - kube-system
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`allowedRoles` |[]string |The list of Talos API roles which can be granted for access from Kubernetes pods.<br><br>Empty list means that no roles can be granted, so access is blocked.  | |
|`allowedKubernetesNamespaces` |[]string |The list of Kubernetes namespaces Talos API access is available from.  | |






