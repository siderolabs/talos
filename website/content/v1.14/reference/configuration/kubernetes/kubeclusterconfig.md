---
description: KubeClusterConfig configures Kubernetes cluster base settings.
title: KubeClusterConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: KubeClusterConfig
clusterName: example-cluster # The cluster name.
endpoint: https://example.com:6443/ # The Kubernetes API endpoint.
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`clusterName` |string |The cluster name.<br>It is used mostly for informational purposes, and gets included into kubeconfig.  | |
|`endpoint` |URL |The Kubernetes API endpoint.<br>For a single-node cluster, this can be the same as the node's IP address.<br>For a multi-node cluster, this should be the load balancer's IP address or DNS name,<br>or any other address (VIP, BGP, etc.) that can be used to reach the Kubernetes API server from the nodes.  | |






