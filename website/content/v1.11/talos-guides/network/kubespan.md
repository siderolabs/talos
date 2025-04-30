---
title: "KubeSpan"
description: "Learn to use KubeSpan to connect Talos Linux machines securely across networks."
aliases:
  - ../../guides/kubespan
  - ../../kubernetes-guides/network/kubespan
---

KubeSpan is a feature of Talos that automates the setup and maintenance of a full mesh [WireGuard](https://www.wireguard.com) network for your cluster, giving you the ability to operate hybrid Kubernetes clusters that can span the edge, datacenter, and cloud.
Management of keys and discovery of peers can be completely automated, making it simple and easy to create hybrid clusters.

KubeSpan consists of client code in Talos Linux, as well as a [discovery service]({{< relref "../discovery" >}}) that enables clients to securely find each other.
Sidero Labs operates a free Discovery Service, but the discovery service may, with a commercial license, be operated by your organization and can be [downloaded here](https://github.com/siderolabs/discovery-service).

## Video Walkthrough

To see a live demo of KubeSpan, see one the videos below:

<iframe width="560" height="315" src="https://www.youtube.com/embed/RRk8gYzRHJg" frameborder="0" allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture" allowfullscreen></iframe>

<iframe width="560" height="315" src="https://www.youtube.com/embed/sBKIFLhC9MQ" frameborder="0" allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture" allowfullscreen></iframe>

## Network Requirements

KubeSpan uses **UDP port 51820** to carry all KubeSpan encrypted traffic.
Because UDP traversal of firewalls is often lenient, and the Discovery Service communicates the apparent IP address of all peers to all other peers, KubeSpan will often work automatically, even when each nodes is behind their own firewall.
However, when both ends of a KubeSpan connection are behind firewalls, it is possible the connection may not be established correctly - it depends on each end sending out packets in a limited time window.

Thus best practice is to ensure that one end of all possible node-node communication allows UDP port 51820, inbound.

For example, if control plane nodes are running in a corporate data center, behind firewalls, KubeSpan connectivity will work correctly so long as worker nodes on the public Internet can receive packets on UDP port 51820.
(Note the workers will also need to receive TCP port 50000 for initial configuration via `talosctl`).

An alternative topology would be to run control plane nodes in a public cloud, and allow inbound UDP port 51820 to the control plane nodes.
Workers could be behind firewalls, and KubeSpan connectivity will be established.
Note that if workers are in different locations, behind different firewalls, the KubeSpan connectivity between workers *should* be correctly established, but may require opening the KubeSpan UDP port on the local firewall also.

### Caveats

#### Kubernetes API Endpoint Limitations

When the K8s endpoint is an IP address that is **not** part of Kubespan, but is an address that is forwarded on to the Kubespan address of a control plane node, without changing the source address, then worker nodes will fail to join the cluster.
In such a case, the control plane node has no way to determine whether the packet arrived on the private Kubespan address, or the public IP address.
If the source of the packet was a Kubespan member, the reply will be Kubespan encapsulated, and thus not translated to the public IP, and so the control plane will reply to the session with the wrong address.

This situation is seen, for example, when the Kubernetes API endpoint is the public IP of a VM in GCP or Azure for a single node control plane.
The control plane will receive packets on the public IP, but will reply from it's KubeSpan address.
The workaround is to create a load balancer to terminate the Kubernetes API endpoint.

#### Digital Ocean Limitations

Digital Ocean assigns an "Anchor IP" address to each droplet.
Talos Linux correctly identifies this as a link-local address, and configures KubeSpan correctly, but this address will often be selected by Flannel or other CNIs as a node's private IP.
Because this address is not routable, nor advertised via KubeSpan, it will break pod-pod communication between nodes.
This can be worked-around by assigning a non-Anchor private IP:

`kubectl annotate node do-worker flannel.alpha.coreos.com/public-ip-overwrite=10.116.X.X`

Then restarting flannel:
`kubectl delete pods -n kube-system -l k8s-app=flannel`

#### Host Port Limitations

As mentioned in Network Requirements, Kubespan uses **UDP port 51820** to carry all KubeSpan encrypted traffic.
For clusters that make heavy use of host ports for Kubernetes pods, care should be taken to ensure that this port is not given to these pods.
Failure to do so can result in a pod being assigned the 51820 port and conflicting with Kubespan traffic.

## Enabling

### Creating a New Cluster

To enable KubeSpan for a new cluster, we can use the `--with-kubespan` flag in `talosctl gen config`.
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

> The default discovery service is an external service hosted by Sidero Labs at `https://discovery.talos.dev/`.
> Contact Sidero Labs if you need to run this service privately.

### Enabling for an Existing Cluster

In order to enable KubeSpan on an existing cluster, enable `kubespan` and `discovery` settings in the machine config for each machine in the cluster (`discovery` is enabled by default):

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

KubeSpan will automatically discover all cluster members, exchange Wireguard public keys and establish a full mesh network.

There are configuration options available which are not usually required:

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

The `filters` setting allows hiding some endpoints from being advertised over KubeSpan.
This is useful when some endpoints are known to be unreachable between the nodes, so that KubeSpan doesn't try to establish a connection to them.
Another use-case is hiding some endpoints if nodes can connect on multiple networks, and some of the networks are more preferable than others.

To include additional announced endpoints, such as inbound NAT mappings, you can add the [machine config document]({{< relref "../../reference/configuration/network/kubespanendpointsconfig" >}}).

```yaml
apiVersion: v1alpha1
kind: KubespanEndpointsConfig
extraAnnouncedEndpoints:
    - 192.168.101.3:61033
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

The Wireguard private key is generated and never leaves the node, while the public key is published through the cluster discovery.

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
