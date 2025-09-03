---
title: "Multihoming"
description: "How to handle multihomed machines"
---

When a machine has multiple IPv4 or IPv6 addresses, it is important to control which addresses Talos components use for communication.
Without explicit configuration, services like etcd or the kubelet may select different addresses across reboots or workloads, leading to unstable networking.

A point to note is that the machines may become multihomed via privileged workloads.

## Multihoming and etcd

The `etcd` cluster needs to establish a mesh of connections among the members.
It is done using the so-called advertised address - each node learns the others’ addresses as they are advertised.
It is crucial that these IP addresses are stable, i.e., that each node always advertises the same IP address.
Moreover, it is beneficial to control them to establish the correct routes between the members and, e.g., avoid congested paths.
In Talos, these addresses are controlled using the `cluster.etcd.advertisedSubnets` configuration key.

## Multihoming and kubelets

Stable IP addressing for kubelets (i.e., nodeIP) is not strictly necessary but highly recommended as it ensures that, e.g., kube-proxy and CNI routing take the desired routes.
Analogously to etcd, for kubelets this is controlled via `machine.kubelet.nodeIP.validSubnets`.

For example, let’s assume that we have a cluster with two networks:

* public network
* private network `192.168.0.0/16`

We want to use the private network for etcd and kubelet communication:

```yaml
machine:
  kubelet:
    nodeIP:
      validSubnets:
        - 192.168.0.0/16
#...
cluster:
  etcd:
    advertisedSubnets: # listenSubnets defaults to advertisedSubnets if not set explicitly
      - 192.168.0.0/16`
```

This way we ensure that the `etcd` cluster will use the private network for communication and the kubelets will use the private network for communication with the control plane.
