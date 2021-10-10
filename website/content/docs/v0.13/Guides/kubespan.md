---
title: "KubeSpan"
---


KubeSpan is a feature of Talos that automates the setup and maintainance of a full mesh [WireGuard](https://www.wireguard.com) network for your cluster, giving you the ablility to operate hybrid Kuberentes clusters that can span the edge, datacenter, and cloud.

Designed to be simple and impact-free, KubeSpan explicitly avoids iptables, and manipulation of main routes.
Management of keys, and discovery of peers can be completely automated for a zero-touch experience that makes X simple and easy.
Peers can amalgmated through a number of optional registries.

## Enabling

### Creating a New Cluster

To generate configuration files for a new cluster, we can use the `--with-kubespan` flag in `talosctl gen config`.
This will

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

> The discovery service is an external service hosted for free by Sidero Labs.
> The default value is `https://discovery.talos.dev/`.
> Contact Sidero Labs if you need to run this service privately.

### Upgrading an Existing Cluster

In order to enable KubeSpan for an existing cluster, first upgrade to the latest v0.13.
Once your cluster is upgraded, the configuration of each node must contain the globally unique identifier, and the shared secret for the cluster.

To generate an `id`:

```sh
openssl rand -base64 32
EUsCYz+oHNuBppS51P9aKSIOyYvIPmbZK944PWgiyMQ=
```

To generate a `secret`:

```sh
openssl rand -base64 32
AbdsWjY9i797kGglghKvtGdxCsdllX9CemLq+WGVeaw=
```

Now, update the configuration of each node with the cluster `id` and `secret`, and enable `kubespan` and `discovery`.

```sh
talosctl edit mc --immediate
```

```yaml
cluster:
  id: EUsCYz+oHNuBppS51P9aKSIOyYvIPmbZK944PWgiyMQ=
  secret: AbdsWjY9i797kGglghKvtGdxCsdllX9CemLq+WGVeaw=
```

Enable `kubespan`:

```yaml
machine:
  network:
    kubespan:
      enabled: true
```

Enable `discovery`:

```yaml
cluster:
  discovery:
    enabled: true
```

## Registries

By default, Talos will use the `kubernetes` and `discovery` registries.
Either one can be disabled.
To disable a registry, set `disabled` to `true` (this options is the same for all registries):
For example, to disable the `discovery` registry:

```yaml
cluster:
  discovery:
    enabled: true
    registries:
      discovery:
        disabled: true
```

Disabling all registries effectively disables cluster discovery altogether.

> As of v0.13, Talos supports the `kubernetes` and `discovery` registries.

TODO: What is the use case for enabling KubeSpan but _not_ discovery? Are there any unallowed combinations (e.g. discovery enabled but kubespan disabled)?

## Resource Definitions

Talos v0.13 introduces seven new resources that can be used to introspect the new discovery and KubeSpan features.

### Discovery

#### Affiliates

An affiliate is a proposed member.

```sh
talosctl get affiliates
```

#### Members

The members of the mesh network can be obtained with:

```sh
talosctl get members
```

#### Identities

The node's unique identity (base62 encoded random 32 bytes) can be obtained with:
TODO: Why base62?

```sh
talosctl get identities
```

### KubeSpan

#### Kubespanpeerspecs

A node's wireguard peers can be obtained with:

```sh
talosctl get kubespanpeerspecs
```

#### Kubespanendpoints

A node's wireguard endpoints (peer addresses) can be obtained with:

```sh
talosctl get kubespanendpoints
```

The endpoint ID is the base64 encoded WireGuard public key obtained with `talosctl get kubespanidentities`.

#### Kubespanidentities

A node's wireguard identities can be obtained with:

```sh
talosctl get kubespanidentities
```

#### Kubespanpeerstatuses

The status of WireGuard peers for a given node can be obtained with:

```sh
talosctl get kubespanpeerstatuses
```

## Limitations

TODO: @rsmitty what were some of the limitations you saw?
