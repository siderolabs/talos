---
description: BGPInstanceConfig configures a native BGP routing instance on the host.
title: BGPInstanceConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: BGPInstanceConfig
name: fabric # Name of the BGP routing instance.
localASN: 65001 # Local autonomous system number for the BGP instance.
# Names or aliases of the links whose addresses are originated into BGP as host routes (/32, /128).
advertise:
    - dummy0
multipath: true # Enable ECMP (multipath) for routes learned from multiple neighbors.
# BGP neighbors in this routing instance.
neighbors:
    - link: enp1s0 # Link name or alias for an unnumbered (IPv6 link-local) session. Mutually exclusive with `address`.
      peerASN: 65000 # Expected peer ASN. Zero accepts any ASN advertised by the peer (eBGP "external").
      holdTime: 9s # BGP hold time for this neighbor. Zero uses the implementation default.
      # BFD (Bidirectional Forwarding Detection) settings for this neighbor.
      bfd:
        transmitInterval: 300ms # Desired minimum transmit interval. Zero uses the implementation default.
        receiveInterval: 300ms # Required minimum receive interval. Zero uses the implementation default.
        detectMultiplier: 3 # BFD detection multiplier. Zero uses the implementation default.
    - link: enp2s0 # Link name or alias for an unnumbered (IPv6 link-local) session. Mutually exclusive with `address`.
      peerASN: 65000 # Expected peer ASN. Zero accepts any ASN advertised by the peer (eBGP "external").
      holdTime: 9s # BGP hold time for this neighbor. Zero uses the implementation default.
      # BFD (Bidirectional Forwarding Detection) settings for this neighbor.
      bfd:
        transmitInterval: 300ms # Desired minimum transmit interval. Zero uses the implementation default.
        receiveInterval: 300ms # Required minimum receive interval. Zero uses the implementation default.
        detectMultiplier: 3 # BFD detection multiplier. Zero uses the implementation default.

# # BGP router-id. If not set, it is derived from the first advertised address.
# routerID: 10.0.0.1

# # Preferred source address set on routes installed from BGP (the kernel route `src` / RTA_PREFSRC,
# routeSource: 10.0.0.1
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`name` |string |Name of the BGP routing instance. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
name: fabric
{{< /highlight >}}</details> | |
|`vrf` |string |Linux VRF link used by this routing instance. If unset, the default routing domain is used.  | |
|`localASN` |uint32 |Local autonomous system number for the BGP instance. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
localASN: 65001
{{< /highlight >}}</details> | |
|`routerID` |Addr |BGP router-id. If not set, it is derived from the first advertised address. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
routerID: 10.0.0.1
{{< /highlight >}}</details> | |
|`routeSource` |Addr |Preferred source address set on routes installed from BGP (the kernel route `src` / RTA_PREFSRC,<br>equivalent to FRR's `ip protocol bgp route-map SETSRC`). If not set, the kernel selects the source address. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
routeSource: 10.0.0.1
{{< /highlight >}}</details> | |
|`advertise` |[]string |Names or aliases of the links whose addresses are originated into BGP as host routes (/32, /128). <details><summary>Show example(s)</summary>{{< highlight yaml >}}
advertise:
    - dummy0
{{< /highlight >}}</details> | |
|`multipath` |bool |Enable ECMP (multipath) for routes learned from multiple neighbors.  | |
|`maxPaths` |uint8 |Maximum number of ECMP next-hops to install. Zero uses the implementation default.  | |
|`neighbors` |<a href="#BGPInstanceConfig.neighbors.">[]BGPNeighborConfig</a> |BGP neighbors in this routing instance.  | |




## neighbors[] {#BGPInstanceConfig.neighbors.}

BGPNeighborConfig configures a concrete BGP neighbor.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`address` |Addr |Neighbor IP address for a numbered session. Mutually exclusive with `link`.  | |
|`link` |string |Link name or alias for an unnumbered (IPv6 link-local) session. Mutually exclusive with `address`.  | |
|`peerASN` |uint32 |Expected peer ASN. Zero accepts any ASN advertised by the peer (eBGP "external").  | |
|`localASN` |uint32 |Local ASN override for this neighbor. Zero uses the instance local ASN.  | |
|`passive` |bool |Wait for the neighbor to establish the connection instead of initiating it.  | |
|`holdTime` |Duration |BGP hold time for this neighbor. Zero uses the implementation default.  | |
|`bfd` |<a href="#BGPInstanceConfig.neighbors..bfd">BGPBFDConfig</a> |BFD (Bidirectional Forwarding Detection) settings for this neighbor.<br>The presence of this block enables BFD; an empty block uses the implementation defaults.<br>BFD is supported only when the BGP instance uses the default routing domain, not a VRF.  | |




### bfd {#BGPInstanceConfig.neighbors..bfd}

BGPBFDConfig configures BFD for a BGP neighbor.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`transmitInterval` |Duration |Desired minimum transmit interval. Zero uses the implementation default.  | |
|`receiveInterval` |Duration |Required minimum receive interval. Zero uses the implementation default.  | |
|`detectMultiplier` |uint8 |BFD detection multiplier. Zero uses the implementation default.  | |










