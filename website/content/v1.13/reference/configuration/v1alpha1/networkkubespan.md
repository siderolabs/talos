---
description: NetworkKubeSpan struct describes KubeSpan configuration.
title: NetworkKubeSpan
---

<!-- markdownlint-disable -->










| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`enabled` |bool |Enable the KubeSpan feature.<br>Cluster discovery should be enabled with .cluster.discovery.enabled for KubeSpan to be enabled.  | |
|`advertiseKubernetesNetworks` |bool |Control whether Kubernetes pod CIDRs are announced over KubeSpan from the node.<br>If disabled, CNI handles encapsulating pod-to-pod traffic into some node-to-node tunnel,<br>and KubeSpan handles the node-to-node traffic.<br>If enabled, KubeSpan will take over pod-to-pod traffic and send it over KubeSpan directly.<br>When enabled, KubeSpan should have a way to detect complete pod CIDRs of the node which<br>is not always the case with CNIs not relying on Kubernetes for IPAM.  | |
|`allowDownPeerBypass` |bool |Skip sending traffic via KubeSpan if the peer connection state is not up.<br>This provides configurable choice between connectivity and security: either traffic is always<br>forced to go via KubeSpan (even if Wireguard peer connection is not up), or traffic can go directly<br>to the peer if Wireguard connection can't be established.  | |
|`harvestExtraEndpoints` |bool |KubeSpan can collect and publish extra endpoints for each member of the cluster<br>based on Wireguard endpoint information for each peer.<br>This feature is disabled by default, don't enable it<br>with high number of peers (>50) in the KubeSpan network (performance issues).  | |
|`mtu` |uint32 |KubeSpan link MTU size.<br>Default value is 1420.  | |
|`filters` |<a href="#NetworkKubeSpan.filters">KubeSpanFilters</a> |KubeSpan advanced filtering of network addresses .<br><br>Settings in this section are optional, and settings apply only to the node.  | |




## filters {#NetworkKubeSpan.filters}

KubeSpanFilters struct describes KubeSpan advanced network addresses filtering.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`endpoints` |[]string |Filter node addresses which will be advertised as KubeSpan endpoints for peer-to-peer Wireguard connections.<br><br>By default, all addresses are advertised, and KubeSpan cycles through all endpoints until it finds one that works.<br><br>Default value: no filtering. <details><summary>Show example(s)</summary>Exclude addresses in 192.168.0.0/16 subnet.:{{< highlight yaml >}}
endpoints:
    - 0.0.0.0/0
    - '!192.168.0.0/16'
    - ::/0
{{< /highlight >}}</details> | |








