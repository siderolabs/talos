---
description: KubePrismConfig configures node-local Kubernetes API load balancer.
title: KubePrismConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: KubePrismConfig
port: 7445 # KubePrism port.
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`port` |int |KubePrism port.<br>The load balancer will be started on `127.0.0.1:<port>` and it will<br>automatically include a controlplane endpoint and direct addresses of<br>all controlplane nodes in the cluster.<br>The KubePrism will pick up the route(s) with the lowest RTT to the controlplane nodes,<br>excluding the unavailable ones, and will automatically update the route list when the controlplane nodes change.  | |
|`tlsServerName` |string |Override the TLS server name (SNI) used by the kubelet when connecting to<br>the KubePrism endpoint.<br><br>KubePrism still listens on `127.0.0.1:<port>` and the kubelet still dials<br>that address, but the generated kubelet kubeconfig will carry<br>`clusters[0].cluster.tls-server-name` set to this value, so the kubelet<br>uses it for SNI and certificate hostname verification.<br><br>This is useful when KubePrism's upstream apiserver is reached through an<br>SNI-routing L4 proxy (for example nginx-ingress in ssl-passthrough mode in<br>front of a Kamaji-hosted apiserver), where SNI=127.0.0.1 doesn't match any<br>route and the proxy serves a fallback certificate.<br><br>When empty (default), no `tls-server-name` is set and behavior is unchanged.  | |
|`readinessCheck` |bool |Use the Kubernetes API server readiness endpoint (`/readyz`) to check the health of the upstreams.<br><br>When enabled, KubePrism establishes a TLS connection to each upstream, verifies the API server<br>certificate against the Kubernetes CA (using `tlsServerName` if set), and considers the upstream<br>healthy only if the API server reports itself ready. This allows KubePrism to stop routing new<br>connections to an API server which still accepts TCP connections, but can't serve requests<br>(e.g. it lost connection to etcd, or it is shutting down).<br><br>The readiness check requires unauthenticated access to the `/readyz` endpoint of the API server<br>(see `KubeAuthenticationConfig`), which is the default since Talos 1.14.<br><br>When disabled (default), the upstream health check is a TCP connection attempt.<br>Machine configuration generated for Talos 1.15+ enables the readiness check.  | |






