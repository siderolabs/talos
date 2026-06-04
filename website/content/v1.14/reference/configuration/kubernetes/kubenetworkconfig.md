---
description: KubeNetworkConfig configures Kubernetes base network settings.
title: KubeNetworkConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: KubeNetworkConfig
dnsDomain: cluster.local # The domain used by Kubernetes DNS.
# The pod subnet (CIDR), this can be a single value or two values for dual-stack clusters.
podSubnets:
    - 10.244.0.0/16
# The service subnet (CIDR), this can be a single value or two values for dual-stack clusters.
serviceSubnets:
    - 10.96.0.0/12
{{< /highlight >}}

{{< highlight yaml >}}
apiVersion: v1alpha1
kind: KubeNetworkConfig
dnsDomain: cluster.local # The domain used by Kubernetes DNS.
# The pod subnet (CIDR), this can be a single value or two values for dual-stack clusters.
podSubnets:
    - fc00:db8:10::/56
# The service subnet (CIDR), this can be a single value or two values for dual-stack clusters.
serviceSubnets:
    - fc00:db8:20::/112
{{< /highlight >}}

{{< highlight yaml >}}
apiVersion: v1alpha1
kind: KubeNetworkConfig
dnsDomain: cluster.local # The domain used by Kubernetes DNS.
# The pod subnet (CIDR), this can be a single value or two values for dual-stack clusters.
podSubnets:
    - 10.244.0.0/16
    - fc00:db8:10::/56
# The service subnet (CIDR), this can be a single value or two values for dual-stack clusters.
serviceSubnets:
    - 10.96.0.0/12
    - fc00:db8:20::/112
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`dnsDomain` |string |The domain used by Kubernetes DNS.<br>The default is `cluster.local`  | |
|`podSubnets` |[]Prefix |The pod subnet (CIDR), this can be a single value or two values for dual-stack clusters.  | |
|`serviceSubnets` |[]Prefix |The service subnet (CIDR), this can be a single value or two values for dual-stack clusters.  | |






