---
description: VLANConfig is a config document to create a VLAN (virtual LAN) over a parent link.
title: VLANConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: VLANConfig
name: enp0s3.34 # Name of the VLAN link (interface) to be created.
vlanID: 34 # VLAN ID to be used for the VLAN link.
parent: enp0s3 # Name of the parent link (interface) on which the VLAN link will be created.
# Configure addresses to be statically assigned to the link.
addresses:
    - address: 192.168.1.100/24 # IP address to be assigned to the link.
# Configure routes to be statically created via the link.
routes:
    - destination: 192.168.0.0/16 # The route's destination as an address prefix.
      gateway: 192.168.1.1 # The route's gateway (if empty, creates link scope route).

# # Set the VLAN mode to use.
# vlanMode: 802.1q
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`name` |string |Name of the VLAN link (interface) to be created. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
name: enp0s3.34
{{< /highlight >}}</details> | |
|`vlanID` |uint16 |VLAN ID to be used for the VLAN link. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
vlanID: 34
{{< /highlight >}}</details> | |
|`vlanMode` |VLANProtocol |Set the VLAN mode to use.<br>If not set, defaults to '802.1q'. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
vlanMode: 802.1q
{{< /highlight >}}</details> |`802.1q`<br />`802.1ad`<br /> |
|`parent` |string |Name of the parent link (interface) on which the VLAN link will be created.<br>Link aliases can be used here as well. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
parent: enp0s3
{{< /highlight >}}</details> | |
|`up` |bool |Bring the link up or down.<br><br>If not specified, the link will be brought up.  | |
|`mtu` |uint32 |Configure LinkMTU (Maximum Transmission Unit) for the link.<br><br>If not specified, the system default LinkMTU will be used (usually 1500).  | |
|`addresses` |<a href="#VLANConfig.addresses.">[]AddressConfig</a> |Configure addresses to be statically assigned to the link.  | |
|`routes` |<a href="#VLANConfig.routes.">[]RouteConfig</a> |Configure routes to be statically created via the link.  | |
|`multicast` |bool |Set the multicast capability of the link.  | |




## addresses[] {#VLANConfig.addresses.}

AddressConfig represents a network address configuration.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`address` |Prefix |IP address to be assigned to the link.<br><br>This field must include the network prefix length (e.g. /24 for IPv4, /64 for IPv6). <details><summary>Show example(s)</summary>{{< highlight yaml >}}
address: 192.168.1.100/24
{{< /highlight >}}{{< highlight yaml >}}
address: fd00::1/64
{{< /highlight >}}</details> | |
|`routePriority` |uint32 |Configure the route priority (metric) for routes created for this address.<br><br>If not specified, the system default route priority will be used.  | |






## routes[] {#VLANConfig.routes.}

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
|`type` |RouteType |The route type.<br><br>If not specified, the route type will be unicast (or multicast for multicast destinations).<br>Common types: unicast, local, broadcast, blackhole, unreachable, prohibit.  |`local`<br />`broadcast`<br />`unicast`<br />`multicast`<br />`blackhole`<br />`unreachable`<br />`prohibit`<br />`throw`<br />`nat`<br />`xresolve`<br /> |








