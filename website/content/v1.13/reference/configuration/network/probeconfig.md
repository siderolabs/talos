---
description: ProbeConfig is a config document to configure network connectivity probes.
title: ProbeConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: ProbeConfig
name: proxy-check # Name of the probe.
interval: 1s # Interval between probe attempts.
failureThreshold: 3 # Number of consecutive failures for the probe to be considered failed after having succeeded.
# TCP probe configuration.
tcp:
    endpoint: proxy.example.com:3128 # Endpoint to probe in the format host:port.
    timeout: 10s # Timeout for the probe.
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`name` |string |Name of the probe. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
name: proxy-check
{{< /highlight >}}</details> | |
|`interval` |Duration |Interval between probe attempts.<br>Defaults to 1s. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
interval: 1s
{{< /highlight >}}</details> | |
|`failureThreshold` |int |Number of consecutive failures for the probe to be considered failed after having succeeded.<br>Defaults to 0 (immediately fail on first failure). <details><summary>Show example(s)</summary>{{< highlight yaml >}}
failureThreshold: 3
{{< /highlight >}}</details> | |
|`tcp` |<a href="#ProbeConfig.tcp">TCPProbeConfigV1Alpha1</a> |TCP probe configuration.  | |




## tcp {#ProbeConfig.tcp}

TCPProbeConfigV1Alpha1 describes TCP probe configuration.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`endpoint` |string |Endpoint to probe in the format host:port. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
endpoint: proxy.example.com:3128
{{< /highlight >}}</details> | |
|`timeout` |Duration |Timeout for the probe.<br>Defaults to 10s. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
timeout: 10s
{{< /highlight >}}</details> | |








