---
description: |
    EncryptionConfiguration allows providing a custom Kubernetes API server encryption configuration.
    When specified, the custom encryption config is applied as-is, bypassing the Talos-generated default.
    The document uses the upstream Kubernetes apiVersion and kind:
    apiVersion: apiserver.config.k8s.io/v1, kind: EncryptionConfiguration.
    See https://kubernetes.io/docs/tasks/administer-cluster/encrypt-data/ for details.
title: EncryptionConfiguration
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: apiserver.config.k8s.io/v1
kind: EncryptionConfiguration
resources:
    - providers:
        - secretbox:
            keys:
                - name: key1
                  secret: dGl0aXRvdG90aXRpdG90b3RpdGl0b3RvdGl0aXRvdG8K
        - identity: {}
      resources:
        - secrets
{{< /highlight >}}







