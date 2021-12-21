---
title: "KubeSpan"
---

KubeSpan is a feature of Talos that automates the setup and maintenance of a full mesh [WireGuard](https://www.wireguard.com) network for your cluster, giving you the ability to operate hybrid Kubernetes clusters that can span the edge, datacenter, and cloud.
Management of keys and discovery of peers can be completely automated for a zero-touch experience that makes it simple and easy to create hybrid clusters.

## Video Walkthrough

To learn more about KubeSpan, see the video below:

<iframe width="560" height="315" src="https://www.youtube.com/embed/lPl3u9BN7j4" frameborder="0" allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture" allowfullscreen></iframe>

To see a live demo of KubeSpan, see one the videos below:

<iframe width="560" height="315" src="https://www.youtube.com/embed/RRk8gYzRHJg" frameborder="0" allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture" allowfullscreen></iframe>

<iframe width="560" height="315" src="https://www.youtube.com/embed/sBKIFLhC9MQ" frameborder="0" allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture" allowfullscreen></iframe>

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

In order to enable KubeSpan for an existing cluster, upgrade to the latest v0.14.
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

### KubeSpanIdentities

A node's WireGuard identities can be obtained with:

```sh
$ talosctl get kubespanidentities -o yaml
...
spec:
    address: fd83:b1f7:fcb5:2802:8c13:71ff:feaf:7c94/128
    subnet: fd83:b1f7:fcb5:2802::/64
    privateKey: gNoasoKOJzl+/B+uXhvsBVxv81OcVLrlcmQ5jQwZO08=
    publicKey: NzW8oeIH5rJyY5lefD9WRoHWWRr/Q6DwsDjMX+xKjT4=
```

Talos automatically configures unique IPv6 address for each node in the cluster-specific IPv6 ULA prefix.

Wireguard private key is generated for the node, private key never leaves the node while public key is published through the cluster discovery.

`KubeSpanIdentity` is persisted across reboots and upgrades in `STATE` partition in the file `kubespan-identity.yaml`.

### KubeSpanPeerSpecs

A node's WireGuard peers can be obtained with:

```sh
$ talosctl get kubespanpeerspecs
ID                                             VERSION   LABEL                    ENDPOINTS
06D9QQOydzKrOL7oeLiqHy9OWE8KtmJzZII2A5/FLFI=   2         talos-default-master-2   ["172.20.0.3:51820"]
THtfKtfNnzJs1nMQKs5IXqK0DFXmM//0WMY+NnaZrhU=   2         talos-default-master-3   ["172.20.0.4:51820"]
nVHu7l13uZyk0AaI1WuzL2/48iG8af4WRv+LWmAax1M=   2         talos-default-worker-2   ["172.20.0.6:51820"]
zXP0QeqRo+CBgDH1uOBiQ8tA+AKEQP9hWkqmkE/oDlc=   2         talos-default-worker-1   ["172.20.0.5:51820"]
```

The peer ID is the Wireguard public key.
`KubeSpanPeerSpecs` are built from the cluster discovery data.

### KubeSpanPeerStatuses

The status of a node's WireGuard peers can be obtained with:

```sh
$ talosctl get kubespanpeerstatuses
ID                                             VERSION   LABEL                    ENDPOINT           STATE   RX         TX
06D9QQOydzKrOL7oeLiqHy9OWE8KtmJzZII2A5/FLFI=   63        talos-default-master-2   172.20.0.3:51820   up      15043220   17869488
THtfKtfNnzJs1nMQKs5IXqK0DFXmM//0WMY+NnaZrhU=   62        talos-default-master-3   172.20.0.4:51820   up      14573208   18157680
nVHu7l13uZyk0AaI1WuzL2/48iG8af4WRv+LWmAax1M=   60        talos-default-worker-2   172.20.0.6:51820   up      130072     46888
zXP0QeqRo+CBgDH1uOBiQ8tA+AKEQP9hWkqmkE/oDlc=   60        talos-default-worker-1   172.20.0.5:51820   up      130044     46556
```

KubeSpan peer status includes following information:

* the actual endpoint used for peer communication
* link state:
  * `unknown`: the endpoint was just changed, link state is not known yet
  * `up`: there is a recent handshake from the peer
  * `down`: there is no handshake from the peer
* number of bytes sent/received over the Wireguard link with the peer

If the connection state goes `down`, Talos will be cycling through the available endpoints until it finds the one which works.

Peer status information is updated every 30 seconds.

### KubeSpanEndpoints

A node's WireGuard endpoints (peer addresses) can be obtained with:

```sh
$ talosctl get kubespanendpoints
ID                                             VERSION   ENDPOINT           AFFILIATE ID
06D9QQOydzKrOL7oeLiqHy9OWE8KtmJzZII2A5/FLFI=   1         172.20.0.3:51820   2VfX3nu67ZtZPl57IdJrU87BMjVWkSBJiL9ulP9TCnF
THtfKtfNnzJs1nMQKs5IXqK0DFXmM//0WMY+NnaZrhU=   1         172.20.0.4:51820   b3DebkPaCRLTLLWaeRF1ejGaR0lK3m79jRJcPn0mfA6C
nVHu7l13uZyk0AaI1WuzL2/48iG8af4WRv+LWmAax1M=   1         172.20.0.6:51820   NVtfu1bT1QjhNq5xJFUZl8f8I8LOCnnpGrZfPpdN9WlB
zXP0QeqRo+CBgDH1uOBiQ8tA+AKEQP9hWkqmkE/oDlc=   1         172.20.0.5:51820   6EVq8RHIne03LeZiJ60WsJcoQOtttw1ejvTS6SOBzhUA
```

The endpoint ID is the base64 encoded WireGuard public key.

The observed endpoints are submitted back to the discovery service (if enabled) so that other peers can try additional endpoints to establish the connection.
