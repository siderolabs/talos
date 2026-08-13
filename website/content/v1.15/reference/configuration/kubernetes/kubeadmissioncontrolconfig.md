---
description: KubeAdmissionControlConfig configures kube-apiserver admission control
    plugins.
title: KubeAdmissionControlConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: KubeAdmissionControlConfig
name: PodSecurity # Admission control plugin name, should be a valid Kubernetes admission control plugin name.
# Kubernetes API server [admission control plugins](https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/).
configuration:
    apiVersion: pod-security.admission.config.k8s.io/v1alpha1
    defaults:
        audit: restricted
        audit-version: latest
        enforce: baseline
        enforce-version: latest
        warn: restricted
        warn-version: latest
    exemptions:
        namespaces:
            - kube-system
        runtimeClasses: []
        usernames: []
    kind: PodSecurityConfiguration
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`name` |string |Admission control plugin name, should be a valid Kubernetes admission control plugin name.  | |
|`configuration` |Unstructured |Kubernetes API server [admission control plugins](https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/).<br>The value is the literal Kubernetes admission control configuration.  | |






