---
description: BGPPeerConfig configures a native BGP speaker on the host.
title: BGPPeerConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: BGPPeerConfig
localASN: 65001 # Local autonomous system number for the BGP speaker.
# Names of the links whose addresses are originated into BGP as host routes (/32, /128).
advertise:
    - dummy0
multipath: true # Enable ECMP (multipath) for routes learned from multiple neighbors.
# BGP neighbors to peer with.
neighbors:
    - link: enp1s0 # Link name for an unnumbered (IPv6 link-local) session. Mutually exclusive with `address`.
      # BFD (Bidirectional Forwarding Detection) configuration for the neighbor.
      bfd: {}
    - link: enp2s0 # Link name for an unnumbered (IPv6 link-local) session. Mutually exclusive with `address`.
      # BFD (Bidirectional Forwarding Detection) configuration for the neighbor.
      bfd: {}

# # BGP router-id. If not set, it is derived from the first advertised address.
# routerID: 10.0.0.1

# # Preferred source address set on routes installed from BGP (the kernel route `src` / RTA_PREFSRC,
# routeSource: 10.0.0.1
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`localASN` |uint32 |Local autonomous system number for the BGP speaker. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
localASN: 65001
{{< /highlight >}}</details> | |
|`routerID` |Addr |BGP router-id. If not set, it is derived from the first advertised address. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
routerID: 10.0.0.1
{{< /highlight >}}</details> | |
|`routeSource` |Addr |Preferred source address set on routes installed from BGP (the kernel route `src` / RTA_PREFSRC,<br>equivalent to FRR's `ip protocol bgp route-map SETSRC`). Set this to the node's loopback so that<br>traffic following BGP-learned routes is sourced from the node identity even though the unnumbered<br>fabric uplinks carry no address of their own. If not set, the kernel selects the source address. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
routeSource: 10.0.0.1
{{< /highlight >}}</details> | |
|`advertise` |[]string |Names of the links whose addresses are originated into BGP as host routes (/32, /128).<br>Typically a loopback or dummy link holding the node IP. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
advertise:
    - dummy0
{{< /highlight >}}</details> | |
|`multipath` |bool |Enable ECMP (multipath) for routes learned from multiple neighbors.  | |
|`maxPaths` |uint8 |Maximum number of ECMP next-hops to install. Zero uses the implementation default.  | |
|`neighbors` |<a href="#BGPPeerConfig.neighbors.">[]BGPNeighborConfig</a> |BGP neighbors to peer with.  | |




## neighbors[] {#BGPPeerConfig.neighbors.}

BGPNeighborConfig configures a single BGP neighbor.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`address` |Addr |Neighbor IP address for a numbered session. Mutually exclusive with `link`.  | |
|`link` |string |Link name for an unnumbered (IPv6 link-local) session. Mutually exclusive with `address`.<br>Link aliases are supported.  | |
|`peerASN` |uint32 |Expected peer ASN. Zero accepts any ASN advertised by the peer (eBGP "external").  | |
|`holdTime` |Duration |BGP hold time. Zero uses the implementation default.  | |
|`bfd` |<a href="#BGPPeerConfig.neighbors..bfd">BGPBFDConfig</a> |BFD (Bidirectional Forwarding Detection) configuration for the neighbor.  | |




### bfd {#BGPPeerConfig.neighbors..bfd}

BGPBFDConfig configures BFD for a BGP neighbor.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`transmitInterval` |Duration |Desired minimum transmit interval. Zero uses the implementation default.  | |
|`receiveInterval` |Duration |Required minimum receive interval. Zero uses the implementation default.  | |
|`detectMultiplier` |uint8 |BFD detection multiplier. Zero uses the implementation default.  | |










