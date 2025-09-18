---
description: DummyLinkConfig is a config document to create a dummy (virtual) network link.
title: DummyLinkConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: DummyLinkConfig
name: dummy1 # Name of the dummy link (interface).
# Configure addresses to be statically assigned to the link.
addresses:
    - address: 192.168.1.100/24 # IP address to be assigned to the link.

# # Override the hardware (MAC) address of the link.
# hardwareAddr: 2e:3c:4d:5e:6f:70
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`name` |string |Name of the dummy link (interface). <details><summary>Show example(s)</summary>{{< highlight yaml >}}
name: dummy1
{{< /highlight >}}</details> | |
|`hardwareAddr` |HardwareAddr |Override the hardware (MAC) address of the link. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
hardwareAddr: 2e:3c:4d:5e:6f:70
{{< /highlight >}}</details> | |
|`up` |bool |Bring the link up or down.<br><br>If not specified, the link will be brought up.  | |
|`mtu` |uint32 |Configure LinkMTU (Maximum Transmission Unit) for the link.<br><br>If not specified, the system default LinkMTU will be used (usually 1500).  | |
|`addresses` |<a href="#DummyLinkConfig.addresses.">[]AddressConfig</a> |Configure addresses to be statically assigned to the link.  | |
|`routes` |<a href="#DummyLinkConfig.routes.">[]RouteConfig</a> |Configure routes to be statically created via the link.  | |




## addresses[] {#DummyLinkConfig.addresses.}

AddressConfig represents a network address configuration.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`address` |Prefix |IP address to be assigned to the link.<br><br>This field must include the network prefix length (e.g. /24 for IPv4, /64 for IPv6). <details><summary>Show example(s)</summary>{{< highlight yaml >}}
address: 192.168.1.100/24
{{< /highlight >}}{{< highlight yaml >}}
address: fd00::1/64
{{< /highlight >}}</details> | |
|`routePriority` |uint32 |Configure the route priority (metric) for routes created for this address.<br><br>If not specified, the system default route priority will be used.  | |






## routes[] {#DummyLinkConfig.routes.}

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








