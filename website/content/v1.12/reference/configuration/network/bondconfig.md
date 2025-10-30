---
description: BondConfig is a config document to create a bond (link aggregation) over a set of links.
title: BondConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: BondConfig
name: bond.int # Name of the bond link (interface) to be created.
# Names of the links (interfaces) on which the bond will be created.
links:
    - enp1s2
    - enp1s2
bondMode: 802.3ad # Bond mode.
miimon: 100 # Link monitoring frequency in milliseconds.
updelay: 200 # The time, in milliseconds, to wait before enabling a slave after a link recovery has been detected.
downdelay: 200 # The time, in milliseconds, to wait before disabling a slave after a link failure has been detected.
xmitHashPolicy: layer3+4 # Selects the transmit hash policy to use for slave selection.
lacpRate: slow # LACPDU frames periodic transmission rate.
adActorSysPrio: 65535 # Actor system priority for 802.3ad.
resendIGMP: 1 # The number of times IGMP packets should be resent.
packetsPerSlave: 1 # The number of packets to transmit through a slave before moving to the next one.
# Configure addresses to be statically assigned to the link.
addresses:
    - address: 10.15.0.3/16 # IP address to be assigned to the link.
# Configure routes to be statically created via the link.
routes:
    - destination: 10.0.0.0/8 # The route's destination as an address prefix.
      gateway: 10.15.0.1 # The route's gateway (if empty, creates link scope route).

# # Override the hardware (MAC) address of the link.
# hardwareAddr: 2e:3c:4d:5e:6f:70

# # ARP link monitoring frequency in milliseconds.
# arpInterval: 1000

# # The list of IPv4 addresses to use for ARP link monitoring when arpInterval is set.
# arpIpTargets:
#     - 10.15.0.1

# # The list of IPv6 addresses to use for NS link monitoring when arpInterval is set.
# nsIp6Targets:
#     - fd00::1

# # Specifies whether or not ARP probes and replies should be validated.
# arpValidate: active

# # Specifies whether ARP probes should be sent to any or all targets.
# arpAllTargets: all

# # Specifies whether active-backup mode should set all slaves to the same MAC address
# failOverMac: active

# # Aggregate selection policy for 802.3ad.
# adSelect: stable

# # Whether to send LACPDU frames periodically.
# adLACPActive: on

# # Policy under which the primary slave should be reselected.
# primaryReselect: always

# # Whether dynamic shuffling of flows is enabled in tlb or alb mode.
# tlbLogicalLb: 1
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`name` |string |Name of the bond link (interface) to be created. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
name: bond.ext
{{< /highlight >}}</details> | |
|`hardwareAddr` |HardwareAddr |Override the hardware (MAC) address of the link. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
hardwareAddr: 2e:3c:4d:5e:6f:70
{{< /highlight >}}</details> | |
|`links` |[]string |Names of the links (interfaces) on which the bond will be created.<br>Link aliases can be used here as well. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
links:
    - enp0s3
    - enp0s8
{{< /highlight >}}</details> | |
|`bondMode` |BondMode |Bond mode. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
bondMode: 802.3ad
{{< /highlight >}}</details> |`balance-rr`<br />`active-backup`<br />`balance-xor`<br />`broadcast`<br />`802.3ad`<br />`balance-tlb`<br />`balance-alb`<br /> |
|`miimon` |uint32 |Link monitoring frequency in milliseconds. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
miimon: 200
{{< /highlight >}}</details> | |
|`updelay` |uint32 |The time, in milliseconds, to wait before enabling a slave after a link recovery has been detected. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
updelay: 300
{{< /highlight >}}</details> | |
|`downdelay` |uint32 |The time, in milliseconds, to wait before disabling a slave after a link failure has been detected. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
downdelay: 100
{{< /highlight >}}</details> | |
|`useCarrier` |bool |Specifies whether or not miimon should use MII or ETHTOOL.  | |
|`xmitHashPolicy` |BondXmitHashPolicy |Selects the transmit hash policy to use for slave selection. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
xmitHashPolicy: layer2
{{< /highlight >}}</details> |`layer2`<br />`layer3+4`<br />`layer2+3`<br />`encap2+3`<br />`encap3+4`<br /> |
|`arpInterval` |uint32 |ARP link monitoring frequency in milliseconds. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
arpInterval: 1000
{{< /highlight >}}</details> | |
|`arpIpTargets` |[]Addr |The list of IPv4 addresses to use for ARP link monitoring when arpInterval is set.<br>Maximum of 16 targets are supported. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
arpIpTargets:
    - 10.15.0.1
{{< /highlight >}}</details> | |
|`nsIp6Targets` |[]Addr |The list of IPv6 addresses to use for NS link monitoring when arpInterval is set.<br>Maximum of 16 targets are supported. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
nsIp6Targets:
    - fd00::1
{{< /highlight >}}</details> | |
|`arpValidate` |ARPValidate |Specifies whether or not ARP probes and replies should be validated. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
arpValidate: active
{{< /highlight >}}</details> |`none`<br />`active`<br />`backup`<br />`all`<br />`filter`<br />`filter-active`<br />`filter-backup`<br /> |
|`arpAllTargets` |ARPAllTargets |Specifies whether ARP probes should be sent to any or all targets. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
arpAllTargets: all
{{< /highlight >}}</details> |`any`<br />`all`<br /> |
|`lacpRate` |LACPRate |LACPDU frames periodic transmission rate. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
lacpRate: fast
{{< /highlight >}}</details> |`slow`<br />`fast`<br /> |
|`failOverMac` |FailOverMAC |Specifies whether active-backup mode should set all slaves to the same MAC address<br>at enslavement, when enabled, or perform special handling. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
failOverMac: active
{{< /highlight >}}</details> |`none`<br />`active`<br />`follow`<br /> |
|`adSelect` |ADSelect |Aggregate selection policy for 802.3ad. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
adSelect: stable
{{< /highlight >}}</details> |`stable`<br />`bandwidth`<br />`count`<br /> |
|`adActorSysPrio` |uint16 |Actor system priority for 802.3ad. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
adActorSysPrio: 65535
{{< /highlight >}}</details> | |
|`adUserPortKey` |uint16 |User port key (upper 10 bits) for 802.3ad. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
adUserPortKey: 0
{{< /highlight >}}</details> | |
|`adLACPActive` |ADLACPActive |Whether to send LACPDU frames periodically. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
adLACPActive: on
{{< /highlight >}}</details> |`on`<br />`off`<br /> |
|`primaryReselect` |PrimaryReselect |Policy under which the primary slave should be reselected. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
primaryReselect: always
{{< /highlight >}}</details> |`always`<br />`better`<br />`failure`<br /> |
|`resendIGMP` |uint32 |The number of times IGMP packets should be resent.  | |
|`minLinks` |uint32 |The minimum number of active links required for the bond to be considered active.  | |
|`lpInterval` |uint32 |The number of seconds between instances where the bonding driver sends learning packets to each slave's peer switch.  | |
|`packetsPerSlave` |uint32 |The number of packets to transmit through a slave before moving to the next one.  | |
|`numPeerNotif` |uint8 |The number of peer notifications (gratuitous ARPs and unsolicited IPv6 Neighbor Advertisements)<br>to be issued after a failover event.  | |
|`tlbLogicalLb` |uint8 |Whether dynamic shuffling of flows is enabled in tlb or alb mode. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
tlbLogicalLb: 1
{{< /highlight >}}</details> | |
|`allSlavesActive` |uint8 |Whether duplicate frames (received on inactive ports) should be dropped (0) or delivered (1). <details><summary>Show example(s)</summary>{{< highlight yaml >}}
allSlavesActive: 0
{{< /highlight >}}</details> | |
|`peerNotifDelay` |uint32 |The delay, in milliseconds, between each peer notification.  | |
|`missedMax` |uint8 |The number of arpInterval monitor checks that must fail in order for an interface to be marked down by the ARP monitor.  | |
|`up` |bool |Bring the link up or down.<br><br>If not specified, the link will be brought up.  | |
|`mtu` |uint32 |Configure LinkMTU (Maximum Transmission Unit) for the link.<br><br>If not specified, the system default LinkMTU will be used (usually 1500).  | |
|`addresses` |<a href="#BondConfig.addresses.">[]AddressConfig</a> |Configure addresses to be statically assigned to the link.  | |
|`routes` |<a href="#BondConfig.routes.">[]RouteConfig</a> |Configure routes to be statically created via the link.  | |




## addresses[] {#BondConfig.addresses.}

AddressConfig represents a network address configuration.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`address` |Prefix |IP address to be assigned to the link.<br><br>This field must include the network prefix length (e.g. /24 for IPv4, /64 for IPv6). <details><summary>Show example(s)</summary>{{< highlight yaml >}}
address: 192.168.1.100/24
{{< /highlight >}}{{< highlight yaml >}}
address: fd00::1/64
{{< /highlight >}}</details> | |
|`routePriority` |uint32 |Configure the route priority (metric) for routes created for this address.<br><br>If not specified, the system default route priority will be used.  | |






## routes[] {#BondConfig.routes.}

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








