---
description: |
    StaticEndpointsConfig is a config document to provide a static list of control plane endpoints.
    Talos discovers control plane endpoints dynamically from the discovery service, the Kubernetes API server endpoints, and the cluster control plane endpoint.
    The control plane endpoints are used by worker nodes to reach trustd, and by control plane nodes to join etcd.
    This document provides a static list of control plane node addresses and hostnames which is merged with the dynamically discovered endpoints.
    It is useful when the discovery service is not available, or in clusters without Kubernetes.

    The list is not trusted: every connection is authenticated with TLS, and unreachable endpoints are skipped.
    Endpoints don't include a port, as the port is defined by the service being accessed.
title: StaticEndpointsConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: StaticEndpointsConfig
# List of control plane node IP addresses.
addresses:
    - 10.0.0.11
    - 10.0.0.12
    - 10.0.0.13
# List of control plane node hostnames.
hostnames:
    - cp.example.com
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`addresses` |[]Addr |List of control plane node IP addresses. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
addresses:
    - 10.0.0.11
    - 10.0.0.12
    - 2001:db8::13
{{< /highlight >}}</details> | |
|`hostnames` |[]string |List of control plane node hostnames.<br>The hostnames are resolved by the services connecting to them. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
hostnames:
    - cp1.example.com
    - cp2.example.com
{{< /highlight >}}</details> | |






