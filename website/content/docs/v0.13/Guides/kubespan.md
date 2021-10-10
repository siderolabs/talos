---
title: "KubeSpan"
---

KubeSpan is a feature of Talos that automates the setup and maintainance of a full mesh [WireGuard](https://www.wireguard.com) network for your cluster, giving you the ablility to operate hybrid Kuberentes clusters that can span the edge, datacenter, and cloud.
Management of keys and discovery of peers can be completely automated for a zero-touch experience that makes it simple and easy to create hybrid clusters.

## Enabling

### Creating a New Cluster

To generate configuration files for a new cluster, we can use the `--with-kubespan` flag in `talosctl gen config`.
This will enable peer discovery and KubeSpan.

```yaml
...
    # Provides machine specific network configuration options.
    network:
        # Configures KubeSpan feature.
        kubespan:
            enabled: true # Enable the KubeSpan feature.
...
    # Configures cluster member discovery.
    discovery:
        enabled: true # Enable the cluster membership discovery feature.
        # Configure registries used for cluster member discovery.
        registries:
            # Kubernetes registry uses Kubernetes API server to discover cluster members and stores additional information
            kubernetes: {}
            # Service registry is using an external service to push and pull information about cluster members.
            service: {}
...
# Provides cluster specific configuration options.
cluster:
    id: yui150Ogam0pdQoNZS2lZR-ihi8EWxNM17bZPktJKKE= # Globally unique identifier for this cluster.
    secret: dAmFcyNmDXusqnTSkPJrsgLJ38W8oEEXGZKM0x6Orpc= # Shared secret of cluster.
```

> The default discovery service is an external service hosted for free by Sidero Labs.
> The default value is `https://discovery.talos.dev/`.
> Contact Sidero Labs if you need to run this service privately.

### Upgrading an Existing Cluster

In order to enable KubeSpan for an existing cluster, upgrade to the latest v0.13.
Once your cluster is upgraded, the configuration of each node must contain the globally unique identifier, the shared secret for the cluster, and have KubeSpan and discovery enabled.

> Note: Discovery can be used without KubeSpan, but KubeSpan requires at least one discovery registry.

#### Talos v0.11 or Less

If you are migrating from Talos v0.11 or less, we need to generate a cluster ID and secret.

To generate an `id`:

```sh
$ openssl rand -base64 32
EUsCYz+oHNuBppS51P9aKSIOyYvIPmbZK944PWgiyMQ=
```

To generate a `secret`:

```sh
$ openssl rand -base64 32
AbdsWjY9i797kGglghKvtGdxCsdllX9CemLq+WGVeaw=
```

Now, update the configuration of each node with the cluster with the generated `id` and `secret`.
You should end up with the addition of something like this (your `id` and `secret` should be different):

```yaml
cluster:
  id: EUsCYz+oHNuBppS51P9aKSIOyYvIPmbZK944PWgiyMQ=
  secret: AbdsWjY9i797kGglghKvtGdxCsdllX9CemLq+WGVeaw=
```

> Note: This can be applied in immediate mode (no reboot required) by passing `--immediate` to either the `edit machineconfig` or `apply-config` subcommands.

#### Talos v0.12

Enable `kubespan` and `discovery`.

```yaml
machine:
  network:
    kubespan:
      enabled: true
cluster:
  discovery:
    enabled: true
```

## Resource Definitions

### KubeSpanPeerSpecs

A node's WireGuard peers can be obtained with:

```sh
talosctl get kubespanpeerspecs
```

### KubeSpanEndpoints

A node's WireGuard endpoints (peer addresses) can be obtained with:

```sh
talosctl get kubespanendpoints
```

The endpoint ID is the base64 encoded WireGuard public key obtained with `talosctl get kubespanidentities`.

### KubeSpanIdentities

A node's WireGuard identities can be obtained with:

```sh
talosctl get kubespanidentities
```

### KubeSpanPeerStatuses

The status of a node's WireGuard peers can be obtained with:

```sh
talosctl get kubespanpeerstatuses
```
