---
description: |
    NetworkLinkIngressConfig is a config document to filter out the packets coming into the system based on destination IP and interface.
    Filters incoming packets on a link by destination address: only packets destined to one of
    the node's own addresses are accepted, anything else is dropped.
    This is meant for clusters using an encapsulating CNI, where pod/service CIDR destinations should never
    arrive unencapsulated on an external interface; it is incompatible with native pod IP routing (e.g. BGP).

    The set of accepted destinations can be overridden with the `destinationAddresses` option.
title: NetworkLinkIngressConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: NetworkLinkIngressConfig
name: enp0s2.35 # Name of the link (interface) to filter the incoming packets on.
# Destination addresses to accept on this link, as a list of CIDRs.
destinationAddresses:
    - 1.2.3.4/32
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`name` |string |Name of the link (interface) to filter the incoming packets on. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
name: enp0s2
{{< /highlight >}}{{< highlight yaml >}}
name: enp0s2.35
{{< /highlight >}}</details> | |
|`destinationAddresses` |[]Prefix |Destination addresses to accept on this link, as a list of CIDRs.<br><br>This is an override: when specified, only packets destined to one of these addresses are<br>accepted, and the node's own addresses are not implicitly allowed.<br><br>An empty list allows no destination at all, i.e. all packets arriving on the link are<br>dropped.<br><br>Default value: the node's own addresses. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
destinationAddresses:
    - 1.2.3.4/32
{{< /highlight >}}{{< highlight yaml >}}
destinationAddresses:
    - 192.168.10.0/24
{{< /highlight >}}</details> | |






