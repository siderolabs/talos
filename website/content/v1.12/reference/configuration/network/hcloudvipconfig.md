---
description: |
    HCloudVIPConfig is a config document to configure virtual IP using Hetzner Cloud APIs for announcement.
    Virtual IP configuration should be used only on controlplane nodes to provide virtual IP for Kubernetes API server.
    Any other use cases are not supported and may lead to unexpected behavior.
    Virtual IP will be announced from only one node at a time using Hetzner Cloud APIs.
title: HCloudVIPConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: HCloudVIPConfig
name: int0 # IP address to be advertised as a Layer 2 VIP.
link: enp0s2 # Name of the link to assign the VIP to.
apiToken: my-secret-token # Specifies the Hetzner Cloud API Token.
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`name` |string |IP address to be advertised as a Layer 2 VIP. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
name: 192.168.100.1
{{< /highlight >}}{{< highlight yaml >}}
name: fd00::1
{{< /highlight >}}</details> | |
|`link` |string |Name of the link to assign the VIP to.<br><br>Selector must match exactly one link, otherwise an error is returned.<br>If multiple selectors match the same link, the first one is used.  | |
|`apiToken` |string |Specifies the Hetzner Cloud API Token.  | |






