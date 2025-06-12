---
description: EthernetConfig is a config document to configure Ethernet interfaces.
title: EthernetConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: EthernetConfig
name: enp0s2 # Name of the link (interface).
# Configuration for Ethernet features.
features:
    tx-tcp-segmentation: false
# Configuration for Ethernet link rings.
rings:
    rx: 256 # Number of RX rings.
# Configuration for Ethernet link channels.
channels:
    rx: 4 # Number of RX channels.
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`name` |string |Name of the link (interface).  | |
|`features` |map[string]bool |Configuration for Ethernet features.<br><br>Set of features available and whether they can be enabled or disabled is driver specific.<br>Use `talosctl get ethernetstatus <link> -o yaml` to get the list of available features and<br>their current status.  | |
|`rings` |<a href="#EthernetConfig.rings">EthernetRingsConfig</a> |Configuration for Ethernet link rings.<br><br>This is similar to `ethtool -G` command.  | |
|`channels` |<a href="#EthernetConfig.channels">EthernetChannelsConfig</a> |Configuration for Ethernet link channels.<br><br>This is similar to `ethtool -L` command.  | |




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








