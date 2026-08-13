---
description: BlackholeRouteConfig is a config document to configure blackhole routes.
title: BlackholeRouteConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: BlackholeRouteConfig
name: 10.0.0.0/12 # Route destination as an address prefix.
metric: 100 # The optional metric for the route.
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`name` |string |Route destination as an address prefix. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
name: 10.0.0.0/12
{{< /highlight >}}</details> | |
|`metric` |uint32 |The optional metric for the route.  | |






