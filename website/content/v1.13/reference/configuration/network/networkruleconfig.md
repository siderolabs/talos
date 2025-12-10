---
description: NetworkRuleConfig is a network firewall rule config document.
title: NetworkRuleConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: NetworkRuleConfig
name: ingress-apid # Name of the config document.
# Port selector defines which ports and protocols on the host are affected by the rule.
portSelector:
    # Ports defines a list of port ranges or single ports.
    ports:
        - 50000
    protocol: tcp # Protocol defines traffic protocol (e.g. TCP or UDP).
# Ingress defines which source subnets are allowed to access the host ports/protocols defined by the `portSelector`.
ingress:
    - subnet: 192.168.0.0/16 # Subnet defines a source subnet.
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`name` |string |Name of the config document.  | |
|`portSelector` |<a href="#NetworkRuleConfig.portSelector">RulePortSelector</a> |Port selector defines which ports and protocols on the host are affected by the rule.  | |
|`ingress` |<a href="#NetworkRuleConfig.ingress.">[]IngressRule</a> |Ingress defines which source subnets are allowed to access the host ports/protocols defined by the `portSelector`.  | |




## portSelector {#NetworkRuleConfig.portSelector}

RulePortSelector is a port selector for the network rule.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`ports` |PortRanges |Ports defines a list of port ranges or single ports.<br>The port ranges are inclusive, and should not overlap. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
ports:
    - 80
    - 443
{{< /highlight >}}{{< highlight yaml >}}
ports:
    - 1200-1299
    - 8080
{{< /highlight >}}</details> | |
|`protocol` |Protocol |Protocol defines traffic protocol (e.g. TCP or UDP).  |`tcp`<br />`udp`<br />`icmp`<br />`icmpv6`<br /> |






## ingress[] {#NetworkRuleConfig.ingress.}

IngressRule is a ingress rule.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`subnet` |Prefix |Subnet defines a source subnet. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
subnet: 10.3.4.0/24
{{< /highlight >}}{{< highlight yaml >}}
subnet: 2001:db8::/32
{{< /highlight >}}{{< highlight yaml >}}
subnet: 1.3.4.5/32
{{< /highlight >}}</details> | |
|`except` |Prefix |Except defines a source subnet to exclude from the rule, it gets excluded from the `subnet`.  | |








