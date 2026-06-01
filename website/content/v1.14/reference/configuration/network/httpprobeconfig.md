---
description: HTTPProbeConfig is a config document to configure network HTTP connectivity probes.
title: HTTPProbeConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: HTTPProbeConfig
name: http-check # Name of the probe.
interval: 1s # Interval between probe attempts.
failureThreshold: 3 # Number of consecutive failures for the probe to be considered failed after having succeeded.
url: https://example.com # HTTP or HTTPS URL to probe. The probe succeeds if the server responds with a 2xx or 3xx status code.
timeout: 10s # Timeout for the probe.
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`name` |string |Name of the probe. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
name: http-check
{{< /highlight >}}</details> | |
|`interval` |Duration |Interval between probe attempts.<br>Defaults to 1s. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
interval: 1s
{{< /highlight >}}</details> | |
|`failureThreshold` |int |Number of consecutive failures for the probe to be considered failed after having succeeded.<br>Defaults to 0 (immediately fail on first failure). <details><summary>Show example(s)</summary>{{< highlight yaml >}}
failureThreshold: 3
{{< /highlight >}}</details> | |
|`url` |URL |HTTP or HTTPS URL to probe. The probe succeeds if the server responds with a 2xx or 3xx status code.<br>Probe does not follow redirects. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
url: https://example.com
{{< /highlight >}}</details> | |
|`timeout` |Duration |Timeout for the probe.<br>Defaults to 10s. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
timeout: 10s
{{< /highlight >}}</details> | |






