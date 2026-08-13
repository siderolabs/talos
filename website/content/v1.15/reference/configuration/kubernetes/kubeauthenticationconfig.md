---
description: KubeAuthenticationConfig configures kube-apiserver authentication.
title: KubeAuthenticationConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: KubeAuthenticationConfig
# Kubernetes API server [authentication](https://kubernetes.io/docs/reference/access-authn-authz/authentication/) configuration.
configuration:
    anonymous:
        conditions:
            - path: /livez
            - path: /readyz
            - path: /healthz
        enabled: true
    jwt: []
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`configuration` |Unstructured |Kubernetes API server [authentication](https://kubernetes.io/docs/reference/access-authn-authz/authentication/) configuration.<br>The value is the literal Kubernetes authentication configuration.  | |






