---
description: KubeEtcdEncryptionConfig configures kube-apiserver etcd encryption rules.
title: KubeEtcdEncryptionConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: KubeEtcdEncryptionConfig
# Kubernetes API server [encryption of secret data at rest](https://kubernetes.io/docs/tasks/administer-cluster/encrypt-data/).
config:
    resources:
        - providers:
            - secretbox:
                keys:
                    - name: key2
                      secret: M-EXAMPLE-SECRET-DO-NOT-USE-w=
            - identity: {}
          resources:
            - secrets
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`config` |Unstructured |Kubernetes API server [encryption of secret data at rest](https://kubernetes.io/docs/tasks/administer-cluster/encrypt-data/).<br>Key value should be exact contents of the configuration file, excluding the apiVersion and kind fields.  | |






