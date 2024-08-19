---
title: "Host DNS"
description: "How to configure Talos host DNS caching server."
---

Talos Linux starting with 1.7.0 provides a caching DNS resolver for host workloads (including host networking pods).
Host DNS resolver is enabled by default for clusters created with Talos 1.7, and it can be enabled manually on upgrade.

## Enabling Host DNS

Use the following machine configuration patch to enable host DNS resolver:

```yaml
machine:
  features:
    hostDNS:
      enabled: true
```

Host DNS can be disabled by setting `enabled: false` as well.

## Operations

When enabled, Talos Linux starts a DNS caching server on the host, listening on address `127.0.0.53:53` (both TCP and UDP protocols).
The host `/etc/resolv.conf` file is rewritten to point to the host DNS server:

```shell
$ talosctl read /etc/resolv.conf
nameserver 127.0.0.53
```

All host-based workloads will use the host DNS server for name resolution.
Host DNS server forwards requests to the upstream DNS servers, which are either acquired automatically (DHCP, platform sources, kernel args), or specified in the machine configuration.

The upstream DNS servers can be observed with:

```shell
$ talosctl get resolvers
NODE         NAMESPACE   TYPE             ID          VERSION   RESOLVERS
172.20.0.2   network     ResolverStatus   resolvers   2         ["8.8.8.8","1.1.1.1"]
```

Logs of the host DNS resolver can be queried with:

```shell
talosctl logs dns-resolve-cache
```

Upstream server status can be observed with:

```shell
$ talosctl get dnsupstream
NODE         NAMESPACE   TYPE          ID        VERSION   HEALTHY   ADDRESS
172.20.0.2   network     DNSUpstream   1.1.1.1   1         true      1.1.1.1:53
172.20.0.2   network     DNSUpstream   8.8.8.8   1         true      8.8.8.8:53
```

## Forwarding `kube-dns` to Host DNS

> Note: This feature is enabled by default for new clusters created with Talos 1.8.0 and later.

When host DNS is enabled, by default, `kube-dns` service (`CoreDNS` in Kubernetes) uses host DNS server to resolve external names.
This way the cache is shared between the host DNS and `kube-dns`.

Talos allows forwarding `kube-dns` to the host DNS resolver to be disabled with:

```yaml
machine:
  features:
    hostDNS:
      enabled: true
      forwardKubeDNSToHost: false
```

This configuration should be applied to all nodes in the cluster, if applied after cluster creation, restart `coredns` pods in Kubernetes to pick up changes.

When `forwardKubeDNSToHost` is enabled, Talos Linux allocates IP address `169.254.116.108` for the host DNS server, and `kube-dns` service is configured to use this IP address as the upstream DNS server:
This way `kube-dns` service forwards all DNS requests to the host DNS server, and the cache is shared between the host and `kube-dns`.

## Resolving Talos Cluster Member Names

Host DNS can be configured to resolve Talos cluster member names to IP addresses, so that the host can communicate with the cluster members by name.
Sometimes machine hostnames are already resolvable by the upstream DNS, but this might not always be the case.

Enabling the feature:

```yaml
machine:
  features:
    hostDNS:
      enabled: true
      resolveMemberNames: true
```

When enabled, Talos Linux uses [discovery]({{< relref "../discovery" >}}) data to resolve Talos cluster member names to IP addresses:

```shell
$ talosctl get members
NODE         NAMESPACE   TYPE     ID                             VERSION   HOSTNAME                       MACHINE TYPE   OS                        ADDRESSES
172.20.0.2   cluster     Member   talos-default-controlplane-1   1         talos-default-controlplane-1   controlplane   Talos ({{< release >}})   ["172.20.0.2"]
172.20.0.2   cluster     Member   talos-default-worker-1         1         talos-default-worker-1         worker         Talos ({{< release >}})   ["172.20.0.3"]
```

With the example output above, `talos-default-worker-1` name will resolve to `127.0.0.3`.

Example usage:

```shell
talosctl -n talos-default-worker-1 version
```

When combined with `forwardKubeDNSToHost`, `kube-dns` service will also resolve Talos cluster member names to IP addresses.
