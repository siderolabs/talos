---
description: KubeSpanConfig is a config document to configure KubeSpan.
title: KubeSpanConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: KubeSpanConfig
enabled: true # Enable the KubeSpan feature.
advertiseKubernetesNetworks: false # Control whether Kubernetes pod CIDRs are announced over KubeSpan from the node.
allowDownPeerBypass: false # Skip sending traffic via KubeSpan if the peer connection state is not up.
harvestExtraEndpoints: false # KubeSpan can collect and publish extra endpoints for each member of the cluster
mtu: 1420 # KubeSpan link MTU size.
# KubeSpan advanced filtering of network addresses.
filters:
    # Filter node addresses which will be advertised as KubeSpan endpoints for peer-to-peer Wireguard connections.
    endpoints:
        - 0.0.0.0/0
        - ::/0
    # Filter networks (e.g., host addresses, pod CIDRs if enabled) which will be advertised over KubeSpan.
    excludeAdvertisedNetworks:
        - 192.168.1.0/24
        - 2003::/16
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`enabled` |bool |Enable the KubeSpan feature.<br>Cluster discovery should be enabled with cluster.discovery.enabled for KubeSpan to be enabled.  | |
|`advertiseKubernetesNetworks` |bool |Control whether Kubernetes pod CIDRs are announced over KubeSpan from the node.<br>If disabled, CNI handles pod-to-pod traffic encapsulation.<br>If enabled, KubeSpan takes over pod-to-pod traffic directly.  | |
|`allowDownPeerBypass` |bool |Skip sending traffic via KubeSpan if the peer connection state is not up.<br>This provides configurable choice between connectivity and security.  | |
|`harvestExtraEndpoints` |bool |KubeSpan can collect and publish extra endpoints for each member of the cluster<br>based on Wireguard endpoint information for each peer.<br>Disabled by default. Do not enable with high peer counts (>50).  | |
|`mtu` |uint32 |KubeSpan link MTU size.<br>Default value is 1420.  | |
|`filters` |<a href="#KubeSpanConfig.filters">KubeSpanFiltersConfig</a> |KubeSpan advanced filtering of network addresses.<br>Settings are optional and apply only to this node.  | |




## filters {#KubeSpanConfig.filters}

KubeSpanFiltersConfig configures KubeSpan endpoint filters.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`endpoints` |[]string |Filter node addresses which will be advertised as KubeSpan endpoints for peer-to-peer Wireguard connections.<br><br>By default, all addresses are advertised, and KubeSpan cycles through all endpoints until it finds one that works.<br><br>Default value: no filtering. <details><summary>Show example(s)</summary>Exclude addresses in 192.168.0.0/16 subnet.:{{< highlight yaml >}}
endpoints:
    - 0.0.0.0/0
    - '!192.168.0.0/16'
    - ::/0
{{< /highlight >}}</details> | |
|`excludeAdvertisedNetworks` |[]Prefix |Filter networks (e.g., host addresses, pod CIDRs if enabled) which will be advertised over KubeSpan.<br><br>By default, all networks are advertised.<br>Use this filter to exclude some networks from being advertised.<br><br>Note: excluded networks will not be reachable over KubeSpan, so make sure<br>these networks are still reachable via some other route (e.g., direct connection).<br><br>Default value: no filtering. <details><summary>Show example(s)</summary>Exclude private networks from being advertised.:{{< highlight yaml >}}
excludeAdvertisedNetworks:
    - 192.168.1.0/24
{{< /highlight >}}</details> | |








