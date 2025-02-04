---
description: EthernetConfig is a config document to configure Ethernet interfaces.
title: EthernetConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: EthernetConfig
name: enp0s2 # Name of the link (interface).
# Configuration for Ethernet link rings.
rings:
    rx: 256 # Number of RX rings.
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`name` |string |Name of the link (interface).  | |
|`rings` |<a href="#EthernetConfig.rings">EthernetRingsConfig</a> |<details><summary>Configuration for Ethernet link rings.</summary><br />This is similar to `ethtool -G` command.</details>  | |




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








