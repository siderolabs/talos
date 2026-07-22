---
description: KubeCoreDNSConfig configures CoreDNS deployment.
title: KubeCoreDNSConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: KubeCoreDNSConfig
enabled: true # By default, CoreDNS deployment is enabled.
image: registry.k8s.io/coredns/coredns:v1.14.6 # The container image used to run the CoreDNS.
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`enabled` |bool |By default, CoreDNS deployment is enabled.<br>Set to false to disable the CoreDNS deployment.  | |
|`image` |string |The container image used to run the CoreDNS.<br><br>If the value is not set, the default image will be used.  | |






