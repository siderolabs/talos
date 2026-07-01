---
description: NetworkNATRuleConfig is a network NAT rule config document.
title: NetworkNATRuleConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: NetworkNATRuleConfig
name: masquerade # Name of the config document.
# SourceAddress restricts which source addresses are matched.
sourceAddress:
    # IncludeSubnets is the list of CIDRs that match.
    includeSubnets:
        - 10.0.0.0/8
# OutputInterface restricts which egress interfaces trigger the rule.
outputInterface:
    # InterfaceNames is the list of interface names to match against.
    interfaceNames:
        - eth0
{{< /highlight >}}

{{< highlight yaml >}}
apiVersion: v1alpha1
kind: NetworkNATRuleConfig
name: snat-rule # Name of the config document.
type: snat # Type is the kind of NAT operation: masquerade, snat, or dnat.
# SourceAddress restricts which source addresses are matched.
sourceAddress:
    # IncludeSubnets is the list of CIDRs that match.
    includeSubnets:
        - 10.0.0.0/8
# OutputInterface restricts which egress interfaces trigger the rule.
outputInterface:
    # InterfaceNames is the list of interface names to match against.
    interfaceNames:
        - eth0
snatAddress: 203.0.113.1 # SNATAddress is the address to translate the source to.
{{< /highlight >}}

{{< highlight yaml >}}
apiVersion: v1alpha1
kind: NetworkNATRuleConfig
name: dnat-rule # Name of the config document.
type: dnat # Type is the kind of NAT operation: masquerade, snat, or dnat.
# InputInterface restricts which ingress interfaces trigger the rule.
inputInterface:
    # InterfaceNames is the list of interface names to match against.
    interfaceNames:
        - eth0
# DestinationAddress restricts which destination addresses are matched.
destinationAddress:
    # IncludeSubnets is the list of CIDRs that match.
    includeSubnets:
        - 203.0.113.1/32
dnatAddress: 10.0.0.1 # DNATAddress is the address to redirect traffic to.
dnatPort: 8080 # DNATPort is the port to redirect traffic to.
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`name` |string |Name of the config document.  | |
|`type` |NATType |Type is the kind of NAT operation: masquerade, snat, or dnat.<br>Defaults to masquerade when omitted.  |`masquerade`<br />`snat`<br />`dnat`<br /> |
|`sourceAddress` |<a href="#NetworkNATRuleConfig.sourceAddress">NATSubnetConfig</a> |SourceAddress restricts which source addresses are matched.<br>Applies to masquerade, snat, and dnat.  | |
|`outputInterface` |<a href="#NetworkNATRuleConfig.outputInterface">NATInterfaceConfig</a> |OutputInterface restricts which egress interfaces trigger the rule.<br>Applies to masquerade and snat.  | |
|`snatAddress` |Addr |SNATAddress is the address to translate the source to.<br>Required when type is snat.  | |
|`snatPort` |uint16 |SNATPort is the source port to translate to.<br>Optional for snat; when zero, the kernel chooses the source port (default behaviour).  | |
|`inputInterface` |<a href="#NetworkNATRuleConfig.inputInterface">NATInterfaceConfig</a> |InputInterface restricts which ingress interfaces trigger the rule.<br>Applies to dnat.  | |
|`destinationAddress` |<a href="#NetworkNATRuleConfig.destinationAddress">NATSubnetConfig</a> |DestinationAddress restricts which destination addresses are matched.<br>Applies to snat and dnat.  | |
|`dnatAddress` |Addr |DNATAddress is the address to redirect traffic to.<br>Required when type is dnat.  | |
|`dnatPort` |uint16 |DNATPort is the port to redirect traffic to.<br>Optional for dnat; when zero, the original destination port is preserved.  | |




## sourceAddress {#NetworkNATRuleConfig.sourceAddress}

NATSubnetConfig holds a list of subnets to match.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`includeSubnets` |[]Prefix |IncludeSubnets is the list of CIDRs that match.  | |






## outputInterface {#NetworkNATRuleConfig.outputInterface}

NATInterfaceConfig holds a list of interface names to match.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`interfaceNames` |[]string |InterfaceNames is the list of interface names to match against.  | |






## inputInterface {#NetworkNATRuleConfig.inputInterface}

NATInterfaceConfig holds a list of interface names to match.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`interfaceNames` |[]string |InterfaceNames is the list of interface names to match against.  | |






## destinationAddress {#NetworkNATRuleConfig.destinationAddress}

NATSubnetConfig holds a list of subnets to match.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`includeSubnets` |[]Prefix |IncludeSubnets is the list of CIDRs that match.  | |








