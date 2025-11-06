---
description: BridgeConfig is a config document to create a Bridge (link aggregation) over a set of links.
title: BridgeConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: BridgeConfig
name: bridge.3 # Name of the bridge link (interface) to be created.
# Names of the links (interfaces) to be aggregated.
links:
    - eno1
    - eno2
# Bridge STP (Spanning Tree Protocol) configuration.
stp:
    enabled: true # Enable or disable STP on the bridge.

# # Override the hardware (MAC) address of the link.
# hardwareAddr: 2e:3c:4d:5e:6f:70
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`name` |string |Name of the bridge link (interface) to be created. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
name: Bridge.ext
{{< /highlight >}}</details> | |
|`hardwareAddr` |HardwareAddr |Override the hardware (MAC) address of the link. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
hardwareAddr: 2e:3c:4d:5e:6f:70
{{< /highlight >}}</details> | |
|`links` |[]string |Names of the links (interfaces) to be aggregated.<br>Link aliases can be used here as well. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
links:
    - enp1s3
    - enp1s2
{{< /highlight >}}</details> | |
|`stp` |<a href="#BridgeConfig.stp">BridgeSTPConfig</a> |Bridge STP (Spanning Tree Protocol) configuration.  | |
|`vlan` |<a href="#BridgeConfig.vlan">BridgeVLANConfig</a> |Bridge VLAN configuration.  | |
|`up` |bool |Bring the link up or down.<br><br>If not specified, the link will be brought up.  | |
|`mtu` |uint32 |Configure LinkMTU (Maximum Transmission Unit) for the link.<br><br>If not specified, the system default LinkMTU will be used (usually 1500).  | |
|`addresses` |<a href="#BridgeConfig.addresses.">[]AddressConfig</a> |Configure addresses to be statically assigned to the link.  | |
|`routes` |<a href="#BridgeConfig.routes.">[]RouteConfig</a> |Configure routes to be statically created via the link.  | |




## stp {#BridgeConfig.stp}

BridgeSTPConfig is a bridge STP (Spanning Tree Protocol) configuration.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`enabled` |bool |Enable or disable STP on the bridge. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
enabled: true
{{< /highlight >}}</details> | |






## vlan {#BridgeConfig.vlan}

BridgeVLANConfig is a bridge VLAN configuration.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`filtering` |bool |Enable or disable VLAN filtering on the bridge. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
filtering: true
{{< /highlight >}}</details> | |






## addresses[] {#BridgeConfig.addresses.}

AddressConfig represents a network address configuration.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`address` |Prefix |IP address to be assigned to the link.<br><br>This field must include the network prefix length (e.g. /24 for IPv4, /64 for IPv6). <details><summary>Show example(s)</summary>{{< highlight yaml >}}
address: 192.168.1.100/24
{{< /highlight >}}{{< highlight yaml >}}
address: fd00::1/64
{{< /highlight >}}</details> | |
|`routePriority` |uint32 |Configure the route priority (metric) for routes created for this address.<br><br>If not specified, the system default route priority will be used.  | |






## routes[] {#BridgeConfig.routes.}

RouteConfig represents a network route configuration.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`destination` |Prefix |The route's destination as an address prefix.<br><br>If not specified, a default route will be created for the address family of the gateway. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
destination: 10.0.0.0/8
{{< /highlight >}}</details> | |
|`gateway` |Addr |The route's gateway (if empty, creates link scope route). <details><summary>Show example(s)</summary>{{< highlight yaml >}}
gateway: 10.0.0.1
{{< /highlight >}}</details> | |
|`source` |Addr |The route's source address (optional).  | |
|`metric` |uint32 |The optional metric for the route.  | |
|`mtu` |uint32 |The optional MTU for the route.  | |
|`table` |RoutingTable |The routing table to use for the route.<br><br>If not specified, the main routing table will be used.  | |








