---
description: DHCPv6Config is a config document to configure DHCPv6 on a network link.
title: DHCPv6Config
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: DHCPv6Config
name: enp0s2 # Name of the link (interface).

# # Raw value of the DUID to use as client identifier.
# duidRaw: 00:01:00:01:23:45:67:89:ab:cd:ef:01:23:45
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`name` |string |Name of the link (interface). <details><summary>Show example(s)</summary>{{< highlight yaml >}}
name: enp0s2
{{< /highlight >}}</details> | |
|`routeMetric` |uint32 |An optional metric for the routes received from the DHCP server.<br><br>Lower values indicate higher priority.<br>Default value is 1024.  | |
|`ignoreHostname` |bool |Ignore hostname received from the DHCP server.  | |
|`clientIdentifier` |ClientIdentifier |Client identifier to use when communicating with DHCP servers.<br><br>Defaults to 'mac' if not set.  |`none`<br />`mac`<br />`duid`<br /> |
|`duidRaw` |HardwareAddr |Raw value of the DUID to use as client identifier.<br><br>This field is only used if 'clientIdentifier' is set to 'duid'. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
duidRaw: 00:01:00:01:23:45:67:89:ab:cd:ef:01:23:45
{{< /highlight >}}</details> | |






