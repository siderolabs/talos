---
description: RoutingRuleConfig is a config document to configure Linux policy routing rules.
title: RoutingRuleConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: RoutingRuleConfig
name: "1000" # Priority of the routing rule.
src: 10.0.0.0/8 # Source address prefix to match.
table: "100" # The routing table to look up if the rule matches.
action: unicast # The action to perform when the rule matches.

# # Destination address prefix to match.
# dst: 192.168.0.0/16

# # Match packets arriving on this interface.
# iifName: eth0

# # Match packets going out on this interface.
# oifName: eth1

# # Match packets with this firewall mark value.
# fwMark: 256

# # Mask for the firewall mark comparison.
# fwMask: 65280
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`name` |string |Priority of the routing rule.<br>Lower values are matched first.<br>Must be between 1 and 32765 (excluding reserved priorities [0 32500 32501 32766 32767]).<br>Must be unique across all routing rules in the configuration. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
name: 1000
{{< /highlight >}}</details> | |
|`src` |Prefix |Source address prefix to match.<br>If empty, matches all sources. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
src: 10.0.0.0/8
{{< /highlight >}}</details> | |
|`dst` |Prefix |Destination address prefix to match.<br>If empty, matches all destinations. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
dst: 192.168.0.0/16
{{< /highlight >}}</details> | |
|`table` |RoutingTable |The routing table to look up if the rule matches. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
table: 100
{{< /highlight >}}</details> | |
|`action` |RoutingRuleAction |The action to perform when the rule matches.<br>Defaults to "unicast" (table lookup).  |`unicast`<br />`blackhole`<br />`unreachable`<br />`prohibit`<br /> |
|`iifName` |string |Match packets arriving on this interface. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
iifName: eth0
{{< /highlight >}}</details> | |
|`oifName` |string |Match packets going out on this interface. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
oifName: eth1
{{< /highlight >}}</details> | |
|`fwMark` |uint32 |Match packets with this firewall mark value. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
fwMark: 256
{{< /highlight >}}</details> | |
|`fwMask` |uint32 |Mask for the firewall mark comparison. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
fwMask: 65280
{{< /highlight >}}</details> | |






