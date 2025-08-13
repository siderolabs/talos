---
title: "KubePrism"
description: "Enabling in-cluster highly-available controlplane endpoint."
---

Kubernetes pods running in CNI mode can use the `kubernetes.default.svc` service endpoint to access the Kubernetes API server,
however pods running in host networking mode can only use the external cluster endpoint to access the Kubernetes API server.

Because Kubernetes controlplane and CNI components run in host networking mode, they can only use the external cluster endpoint to access the Kubernetes API server.
If the external cluster endpoint is unavailable (due to misconfiguration, network issues, etc), this will cause issues in the cluster: pods will not be scheduled, service IPs stop working, etc.

KubePrism solves this problem by enabling an in-cluster highly-available controlplane endpoint on every node in the cluster.

## Video Walkthrough

To see a live demo of this writeup, see the video below:

<iframe width="560" height="315" src="https://www.youtube.com/embed/VNRE64R5akM" frameborder="0" allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture" allowfullscreen></iframe>

## Enabling KubePrism

> As of Talos 1.6, KubePrism is enabled by default with port 7445.

To enable KubePrism, apply the following machine config patch either during the machine config generation, or to a running cluster (the patch should be applied to all nodes):

```yaml
machine:
  features:
    kubePrism:
      enabled: true
      port: 7445
```

> Note: the `port` specified should be available on every node in the cluster.

## How it works

Talos spins up a TCP loadbalancer on every machine on the `localhost` on the specified port which automatically picks up one of the endpoints:

* the external cluster endpoint as specified in the machine configuration
* for controlplane machines: `https://localhost:<api-server-local-port>` (`http://localhost:6443` in the default configuration)
* `https://<controlplane-address>:<api-server-port>` for every controlplane machine (based on the information from [Cluster Discovery]({{< relref "../../talos-guides/discovery" >}}))

KubePrism automatically filters out unhealthy (or unreachable) endpoints, and prefers lower-latency endpoints over higher-latency endpoints.

Talos automatically reconfigures `kubelet`, `kube-scheduler` and `kube-controller-manager` to use the KubePrism endpoint.
The `kube-proxy` manifest is also reconfigured to use the KubePrism endpoint by default, but when enabling KubePrism for a running cluster the manifest should be updated
with `talosctl upgrade-k8s` command.

When using CNI components that require access to the Kubernetes API server, the KubePrism endpoint should be passed to the CNI configuration (e.g. Cilium, Calico CNIs).

## Notes

As the list of endpoints for KubePrism includes the external cluster endpoint, KubePrism in the worst case scenario will behave the same as the external cluster endpoint.
For controlplane nodes, the KubePrism should pick up the `localhost` endpoint of the `kube-apiserver`, minimizing the latency.
Worker nodes might use direct address of the controlplane endpoint if the latency is lower than the latency of the external cluster endpoint.

KubePrism listen endpoint is bound to `localhost` address, so it can't be used outside the cluster.
