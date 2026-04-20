---
description: NetworkDefaultActionConfig is a ingress firewall default action configuration document.
title: NetworkDefaultActionConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: NetworkDefaultActionConfig
ingress: accept # Default action for all not explicitly configured ingress traffic: accept or block.
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`ingress` |DefaultAction |Default action for all not explicitly configured ingress traffic: accept or block.  |`accept`<br />`block`<br /> |






