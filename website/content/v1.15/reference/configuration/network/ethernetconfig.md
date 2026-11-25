---
description: EthernetConfig is a config document to configure Ethernet interfaces.
title: EthernetConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: EthernetConfig
name: enp0s2 # Name or alias of the link to configure.
# Configuration for Ethernet features.
features:
    tx-tcp-segmentation: false
# Configuration for Ethernet link rings.
rings:
    rx: 256 # Number of RX rings.
# Configuration for Ethernet link channels.
channels:
    rx: 4 # Number of RX channels.
# Configuration for Ethernet link flow control (802.3x pause frames).
flowControl:
    rx: true # Whether to enable RX flow control (accepting pause frames from the link peer).
    tx: false # Whether to enable TX flow control (sending pause frames to the link peer).

# # Wake-on-LAN modes to enable.
# wakeOnLan:
#     - unicast
#     - magic
{{< /highlight >}}

{{< highlight yaml >}}
apiVersion: v1alpha1
kind: EthernetConfig
name: virtio-features # Name or alias of the link to configure.
# Selector to match the links to configure.
selector:
    match: link.driver == "virtio_net" # The Common Expression Language (CEL) expression to match the link.
# Configuration for Ethernet features.
features:
    tx-nocache-copy: true

# # Wake-on-LAN modes to enable.
# wakeOnLan:
#     - unicast
#     - magic
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`name` |string |Name or alias of the link to configure.<br><br>If `selector` is set, a descriptive name of the config instead.  | |
|`selector` |<a href="#EthernetConfig.selector">LinkSelector</a> |Selector to match the links to configure.<br>If omitted, the link is matched by `name`.<br><br>Only physical links are matched.<br>If several configs match the same link, their settings are merged in this order, later ones overriding earlier ones:<br>configs matching by a selector, then by link alias, then by exact link name.  | |
|`features` |map[string]bool |Configuration for Ethernet features.<br><br>Set of features available and whether they can be enabled or disabled is driver specific.<br>Use `talosctl get ethernetstatus <link> -o yaml` to get the list of available features and<br>their current status.  | |
|`rings` |<a href="#EthernetConfig.rings">EthernetRingsConfig</a> |Configuration for Ethernet link rings.<br><br>This is similar to `ethtool -G` command.  | |
|`channels` |<a href="#EthernetConfig.channels">EthernetChannelsConfig</a> |Configuration for Ethernet link channels.<br><br>This is similar to `ethtool -L` command.  | |
|`wakeOnLan` |[]WOLMode |Wake-on-LAN modes to enable.<br><br>If this field is omitted, Wake-on-LAN configuration is not changed.<br>An empty list disables Wake-on-LAN.<br><br>This is similar to `ethtool -s <link> wol <options>` command. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
wakeOnLan:
    - unicast
    - magic
{{< /highlight >}}</details> |`phy`<br />`unicast`<br />`multicast`<br />`broadcast`<br />`arp`<br />`magic`<br />`magicsecure`<br />`filter`<br /> |
|`flowControl` |<a href="#EthernetConfig.flowControl">EthernetFlowControlConfig</a> |Configuration for Ethernet link flow control (802.3x pause frames).<br><br>If this field is omitted, flow control configuration is not changed.<br><br>This is similar to `ethtool -A <link>` command.  | |




## selector {#EthernetConfig.selector}

LinkSelector selects links using a CEL expression.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`match` |Expression |The Common Expression Language (CEL) expression to match the link. <details><summary>Show example(s)</summary>match links with a specific MAC address:{{< highlight yaml >}}
match: mac(link.permanent_addr) == "00:1a:2b:3c:4d:5e"
{{< /highlight >}}match links by MAC address prefix:{{< highlight yaml >}}
match: glob("00:1a:2b:*", mac(link.permanent_addr))
{{< /highlight >}}match links by driver name:{{< highlight yaml >}}
match: link.driver == "e1000"
{{< /highlight >}}</details> | |






## rings {#EthernetConfig.rings}

EthernetRingsConfig is a configuration for Ethernet link rings.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`rx` |uint32 |Number of RX rings.  | |
|`tx` |uint32 |Number of TX rings.  | |
|`rx-mini` |uint32 |Number of RX mini rings.  | |
|`rx-jumbo` |uint32 |Number of RX jumbo rings.  | |
|`rx-buf-len` |uint32 |RX buffer length.  | |
|`cqe-size` |uint32 |CQE size.  | |
|`tx-push` |bool |TX push enabled.  | |
|`rx-push` |bool |RX push enabled.  | |
|`tx-push-buf-len` |uint32 |TX push buffer length.  | |
|`tcp-data-split` |bool |TCP data split enabled.  | |






## channels {#EthernetConfig.channels}

EthernetChannelsConfig is a configuration for Ethernet link channels.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`rx` |uint32 |Number of RX channels.  | |
|`tx` |uint32 |Number of TX channels.  | |
|`other` |uint32 |Number of other channels.  | |
|`combined` |uint32 |Number of combined channels.  | |






## flowControl {#EthernetConfig.flowControl}

EthernetFlowControlConfig is a configuration for Ethernet link flow control.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`rx` |bool |Whether to enable RX flow control (accepting pause frames from the link peer).  | |
|`tx` |bool |Whether to enable TX flow control (sending pause frames to the link peer).  | |
|`autoneg` |bool |Whether to enable flow control autonegotiation.  | |








