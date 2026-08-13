---
description: KubeSpanEndpointsConfig is a config document to configure KubeSpan endpoints.
title: KubeSpanEndpointsConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: KubeSpanEndpointsConfig
# A list of extra Wireguard endpoints to announce from this machine.
extraAnnouncedEndpoints:
    - 192.168.13.46:52000
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`extraAnnouncedEndpoints` |[]AddrPort |A list of extra Wireguard endpoints to announce from this machine.<br><br>Talos automatically adds endpoints based on machine addresses, public IP, etc.<br>This field allows to add extra endpoints which are managed outside of Talos, e.g. NAT mapping.  | |






