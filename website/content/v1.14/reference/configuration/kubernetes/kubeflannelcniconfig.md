---
description: KubeFlannelCNIConfig deploys Flannel CNI to the cluster.
title: KubeFlannelCNIConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: KubeFlannelCNIConfig
# Extra arguments for 'flanneld'.
extraArgs:
    - --iface-can-reach=192.168.1.1
kubeNetworkPoliciesEnabled: true # Deploys kube-network-policies along with Flannel.
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`extraArgs` |[]string |Extra arguments for 'flanneld'. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
extraArgs:
    - --iface-can-reach=192.168.1.1
{{< /highlight >}}</details> | |
|`kubeNetworkPoliciesEnabled` |bool |Deploys kube-network-policies along with Flannel.<br><br>This enables Kubernetes Network Policies support in the cluster.  | |






