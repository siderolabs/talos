---
description: VXLANConfig is a config document to create a VXLAN (Virtual eXtensible
    LAN) link over a parent link.
title: VXLANConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: VXLANConfig
name: vxlan900 # Name of the vxlan link (interface) to be created.
id: 100 # VXLAN network identifier (VNI) to be used for the vxlan link.
local: 10.255.0.1 # Source IP address (IPv4 or IPv6) to use in outgoing packets for the tunnel endpoint.
parent: vtep0 # Name of the parent link (interface) used as the physical device for the tunnel endpoint.
learning: false # Enable learning of source link addresses (MAC learning).

# # Multicast group IP address (IPv4 or IPv6) to join for the tunnel.
# group: 239.1.1.1

# # Destination UDP port for VXLAN traffic.
# port: 4789

# # Override the hardware (MAC) address of the link.
# hardwareAddr: 2e:3c:4d:5e:6f:70
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`name` |string |Name of the vxlan link (interface) to be created. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
name: vxlan900
{{< /highlight >}}</details> | |
|`id` |uint32 |VXLAN network identifier (VNI) to be used for the vxlan link. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
id: 100
{{< /highlight >}}</details> | |
|`local` |Addr |Source IP address (IPv4 or IPv6) to use in outgoing packets for the tunnel endpoint. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
local: 10.255.0.1
{{< /highlight >}}</details> | |
|`group` |Addr |Multicast group IP address (IPv4 or IPv6) to join for the tunnel.<br>Either the group or the local address should be set, not both. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
group: 239.1.1.1
{{< /highlight >}}</details> | |
|`parent` |string |Name of the parent link (interface) used as the physical device for the tunnel endpoint.<br>Link aliases can be used here as well. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
parent: vtep0
{{< /highlight >}}</details> | |
|`port` |uint16 |Destination UDP port for VXLAN traffic.<br>If not set, defaults to 4789. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
port: 4789
{{< /highlight >}}</details> | |
|`learning` |bool |Enable learning of source link addresses (MAC learning).<br>If not set, defaults to true. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
learning: false
{{< /highlight >}}</details> | |
|`hardwareAddr` |HardwareAddr |Override the hardware (MAC) address of the link. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
hardwareAddr: 2e:3c:4d:5e:6f:70
{{< /highlight >}}</details> | |
|`up` |bool |Bring the link up or down.<br><br>If not specified, the link will be brought up.  | |
|`mtu` |uint32 |Configure LinkMTU (Maximum Transmission Unit) for the link.<br><br>If not specified, the system default LinkMTU will be used (usually 1500).  | |
|`addresses` |<a href="#VXLANConfig.addresses.">[]AddressConfig</a> |Configure addresses to be statically assigned to the link.  | |
|`routes` |<a href="#VXLANConfig.routes.">[]RouteConfig</a> |Configure routes to be statically created via the link.  | |
|`multicast` |bool |Set the multicast capability of the link.  | |




## addresses[] {#VXLANConfig.addresses.}

AddressConfig represents a network address configuration.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`address` |Prefix |IP address to be assigned to the link.<br><br>This field must include the network prefix length (e.g. /24 for IPv4, /64 for IPv6). <details><summary>Show example(s)</summary>{{< highlight yaml >}}
address: 192.168.1.100/24
{{< /highlight >}}{{< highlight yaml >}}
address: fd00::1/64
{{< /highlight >}}</details> | |
|`routePriority` |uint32 |Configure the route priority (metric) for routes created for this address.<br><br>If not specified, the system default route priority will be used.  | |






## routes[] {#VXLANConfig.routes.}

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








