---
description: DiscoveryServiceConfig is a config document to configure a discovery
    service.
title: DiscoveryServiceConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: DiscoveryServiceConfig
name: primary # Name of the discovery service configuration.
endpoint: https://discovery.talos.dev/ # Discovery service endpoint to use.
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`name` |string |Name of the discovery service configuration.  | |
|`endpoint` |URL |Discovery service endpoint to use. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
endpoint: https://discovery.talos.dev/
{{< /highlight >}}</details> | |






