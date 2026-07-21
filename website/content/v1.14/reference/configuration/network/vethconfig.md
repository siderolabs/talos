---
description: VethConfig is a config document to create a virtual Ethernet device pair.
title: VethConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: VethConfig
name: veth-host # Name of this end of the veth pair.
# Configuration for the peer end of the veth pair.
peer:
    name: veth-router # Name of the peer end of the veth pair.
    # Configure addresses to be statically assigned to the link.
    addresses:
        - address: fda1::/127 # IP address to be assigned to the link.
# Configure addresses to be statically assigned to the link.
addresses:
    - address: fda1::1/127 # IP address to be assigned to the link.
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`name` |string |Name of this end of the veth pair.<br><br>This is a literal kernel interface name. Link aliases are not supported here because<br>the interface is created by this document rather than selected from existing physical links. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
name: veth0
{{< /highlight >}}</details> | |
|`peer` |<a href="#VethConfig.peer">VethPeerConfig</a> |Configuration for the peer end of the veth pair.  | |
|`up` |bool |Bring the link up or down.<br><br>If not specified, the link will be brought up.  | |
|`mtu` |uint32 |Configure LinkMTU (Maximum Transmission Unit) for the link.<br><br>If not specified, the system default LinkMTU will be used (usually 1500).  | |
|`addresses` |<a href="#VethConfig.addresses.">[]AddressConfig</a> |Configure addresses to be statically assigned to the link.  | |
|`routes` |<a href="#VethConfig.routes.">[]RouteConfig</a> |Configure routes to be statically created via the link.  | |
|`multicast` |bool |Set the multicast capability of the link.  | |




## peer {#VethConfig.peer}

VethPeerConfig is the configuration for the peer end of a veth pair.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`name` |string |Name of the peer end of the veth pair.<br><br>This is a literal kernel interface name. Link aliases are not supported here because<br>the interface is created by this document rather than selected from existing physical links.<br><br>Both endpoints are created in the host network namespace. This name can be listed in a<br>VRFConfig document's `links` field to attach the peer endpoint to that VRF. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
name: veth-router
{{< /highlight >}}</details> | |
|`up` |bool |Bring the link up or down.<br><br>If not specified, the link will be brought up.  | |
|`mtu` |uint32 |Configure LinkMTU (Maximum Transmission Unit) for the link.<br><br>If not specified, the system default LinkMTU will be used (usually 1500).  | |
|`addresses` |<a href="#VethConfig.peer.addresses.">[]AddressConfig</a> |Configure addresses to be statically assigned to the link.  | |
|`routes` |<a href="#VethConfig.peer.routes.">[]RouteConfig</a> |Configure routes to be statically created via the link.  | |
|`multicast` |bool |Set the multicast capability of the link.  | |




### addresses[] {#VethConfig.peer.addresses.}

AddressConfig represents a network address configuration.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`address` |Prefix |IP address to be assigned to the link.<br><br>This field must include the network prefix length (e.g. /24 for IPv4, /64 for IPv6). <details><summary>Show example(s)</summary>{{< highlight yaml >}}
address: 192.168.1.100/24
{{< /highlight >}}{{< highlight yaml >}}
address: fd00::1/64
{{< /highlight >}}</details> | |
|`routePriority` |uint32 |Configure the route priority (metric) for routes created for this address.<br><br>If not specified, the system default route priority will be used.  | |






### routes[] {#VethConfig.peer.routes.}

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








## addresses[] {#VethConfig.addresses.}

AddressConfig represents a network address configuration.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`address` |Prefix |IP address to be assigned to the link.<br><br>This field must include the network prefix length (e.g. /24 for IPv4, /64 for IPv6). <details><summary>Show example(s)</summary>{{< highlight yaml >}}
address: 192.168.1.100/24
{{< /highlight >}}{{< highlight yaml >}}
address: fd00::1/64
{{< /highlight >}}</details> | |
|`routePriority` |uint32 |Configure the route priority (metric) for routes created for this address.<br><br>If not specified, the system default route priority will be used.  | |






## routes[] {#VethConfig.routes.}

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








