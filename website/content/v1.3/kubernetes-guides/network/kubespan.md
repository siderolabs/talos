---
title: "KubeSpan"
description: "Learn to use KubeSpan to connect Talos Linux machines securely across networks."
aliases:
  - ../../guides/kubespan
---

KubeSpan is a feature of Talos that automates the setup and maintenance of a full mesh [WireGuard](https://www.wireguard.com) network for your cluster, giving you the ability to operate hybrid Kubernetes clusters that can span the edge, datacenter, and cloud.
Management of keys and discovery of peers can be completely automated for a zero-touch experience that makes it simple and easy to create hybrid clusters.

KubeSpan consists of client code in Talos Linux, as well as a discovery service that enables clients to securely find each other.
Sidero Labs operates a free Discovery Service, but the discovery service may be operated by your organization and can be [downloaded here](https://github.com/siderolabs/discovery-service).

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
machine:
    network:
        kubespan:
            enabled: true # Enable the KubeSpan feature.
cluster:
    discovery:
        enabled: true
        # Configure registries used for cluster member discovery.
        registries:
            kubernetes: # Kubernetes registry is problematic with KubeSpan, if the control plane endpoint is routeable itself via KubeSpan.
              disabled: true
            service: {}
```

> The default discovery service is an external service hosted for free by Sidero Labs.
> The default value is `https://discovery.talos.dev/`.
> Contact Sidero Labs if you need to run this service privately.

### Enabling for an Existing Cluster

In order to enable KubeSpan for an existing cluster, enable `kubespan` and `discovery` settings in the machine config for each machine in the cluster (`discovery` is enabled by default):

```yaml
machine:
  network:
    kubespan:
      enabled: true
cluster:
  discovery:
    enabled: true
```

## Configuration

KubeSpan will automatically discovery all cluster members, exchange Wireguard public keys and establish a full mesh network.

There are a few configuration options available to fine-tune the feature:

```yaml
machine:
  network:
    kubespan:
      enabled: false
      advertiseKubernetesNetworks: false
      allowDownPeerBypass: false
      mtu: 1420
      filters:
        endpoints:
          - 0.0.0.0/0
          - ::/0
```

The setting `advertiseKubernetesNetworks` controls whether the node will advertise Kubernetes service and pod networks to other nodes in the cluster over KubeSpan.
It defaults to being disabled, which means KubeSpan only controls the node-to-node traffic, while pod-to-pod traffic is routed and encapsulated by CNI.
This setting should not be enabled with Calico and Cilium CNI plugins, as they do their own pod IP allocation which is not visible to KubeSpan.

The setting `allowDownPeerBypass` controls whether the node will allow traffic to bypass WireGuard if the destination is not connected over KubeSpan.
If enabled, there is a risk that traffic will be routed unencrypted if the destination is not connected over KubeSpan, but it allows a workaround
for the case where a node is not connected to the KubeSpan network, but still needs to access the cluster.

The `mtu` setting configures the Wireguard MTU, which defaults to 1420.
This default value of 1420 is safe to use when the underlying network MTU is 1500, but if the underlying network MTU is smaller, the KubeSpanMTU should be adjusted accordingly:
`KubeSpanMTU = UnderlyingMTU - 80`.

The `filters` setting allows to hide some endpoints from being advertised over KubeSpan.
This is useful when some endpoints are known to be unreachable between the nodes, so that KubeSpan doesn't try to establish a connection to them.
Another use-case is hiding some endpoints if nodes can connect on multiple networks, and some of the networks are more preferable than others.

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

`KubeSpanIdentity` is persisted across reboots and upgrades in [STATE]({{< relref "../../learn-more/architecture/#file-system-partitions" >}}) partition in the file `kubespan-identity.yaml`.

### KubeSpanPeerSpecs

A node's WireGuard peers can be obtained with:

```sh
$ talosctl get kubespanpeerspecs
ID                                             VERSION   LABEL                          ENDPOINTS
06D9QQOydzKrOL7oeLiqHy9OWE8KtmJzZII2A5/FLFI=   2         talos-default-controlplane-2   ["172.20.0.3:51820"]
THtfKtfNnzJs1nMQKs5IXqK0DFXmM//0WMY+NnaZrhU=   2         talos-default-controlplane-3   ["172.20.0.4:51820"]
nVHu7l13uZyk0AaI1WuzL2/48iG8af4WRv+LWmAax1M=   2         talos-default-worker-2         ["172.20.0.6:51820"]
zXP0QeqRo+CBgDH1uOBiQ8tA+AKEQP9hWkqmkE/oDlc=   2         talos-default-worker-1         ["172.20.0.5:51820"]
```

The peer ID is the Wireguard public key.
`KubeSpanPeerSpecs` are built from the cluster discovery data.

### KubeSpanPeerStatuses

The status of a node's WireGuard peers can be obtained with:

```sh
$ talosctl get kubespanpeerstatuses
ID                                             VERSION   LABEL                          ENDPOINT           STATE   RX         TX
06D9QQOydzKrOL7oeLiqHy9OWE8KtmJzZII2A5/FLFI=   63        talos-default-controlplane-2   172.20.0.3:51820   up      15043220   17869488
THtfKtfNnzJs1nMQKs5IXqK0DFXmM//0WMY+NnaZrhU=   62        talos-default-controlplane-3   172.20.0.4:51820   up      14573208   18157680
nVHu7l13uZyk0AaI1WuzL2/48iG8af4WRv+LWmAax1M=   60        talos-default-worker-2         172.20.0.6:51820   up      130072     46888
zXP0QeqRo+CBgDH1uOBiQ8tA+AKEQP9hWkqmkE/oDlc=   60        talos-default-worker-1         172.20.0.5:51820   up      130044     46556
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
