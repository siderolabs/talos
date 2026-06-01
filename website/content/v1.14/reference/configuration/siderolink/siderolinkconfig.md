---
description: SideroLinkConfig is a SideroLink connection machine configuration document.
title: SideroLinkConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: SideroLinkConfig
apiUrl: https://siderolink.api/jointoken?token=secret # SideroLink API URL to connect to.
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`apiUrl` |URL |SideroLink API URL to connect to. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
apiUrl: https://siderolink.api/?jointoken=secret
{{< /highlight >}}</details> | |
|`uniqueToken` |string |SideroLink unique token to use for the connection (optional).<br><br>This value is overridden with META key UniqueMachineToken.  | |






