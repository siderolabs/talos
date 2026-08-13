---
description: KubeAuditPolicyConfig configures kube-apiserver audit policy.
title: KubeAuditPolicyConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: KubeAuditPolicyConfig
# Kubernetes API server [audit policy](https://kubernetes.io/docs/tasks/debug-application-cluster/audit/) configuration.
configuration:
    apiVersion: audit.k8s.io/v1
    kind: Policy
    rules:
        - level: Metadata
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`configuration` |Unstructured |Kubernetes API server [audit policy](https://kubernetes.io/docs/tasks/debug-application-cluster/audit/) configuration.<br>The value is the literal Kubernetes audit policy configuration.  | |






