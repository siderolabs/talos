---
title: "Discovery"
---

## Registries

Peers are aggregated from a number of optional registries.
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

Disabling all registries effectively disables member discovery altogether.

> As of v0.13, Talos supports the `kubernetes` and `discovery` registries.

## Resource Definitions

Talos v0.13 introduces seven new resources that can be used to introspect the new discovery and KubeSpan features.

### Discovery

#### Affiliates

An affiliate is a proposed member attributed to the fact that the node has the same cluster ID and secret.

```sh
talosctl get affiliates
```

#### Members

A member is an affiliate that has been approved to join the cluster.
The members of the cluster can be obtained with:

```sh
talosctl get members
```

#### Identities

The node's unique identity (base62 encoded random 32 bytes) can be obtained with:

> Note: Using base62 allows the ID to be URL encoded without having to use the ambiguous URL-encoding version of base64.

```sh
talosctl get identities
```

