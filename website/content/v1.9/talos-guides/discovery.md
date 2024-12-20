---
title: "Discovery Service"
description: "Talos Linux Node discovery services"
aliases:
  - ../../guides/discovery
---

Talos Linux includes node-discovery capabilities that depend on a discovery registry.
This allows you to see the members of your cluster, and the associated IP addresses of the nodes.

```bash
talosctl get members
NODE       NAMESPACE   TYPE     ID                             VERSION   HOSTNAME                       MACHINE TYPE   OS               ADDRESSES
10.5.0.2   cluster     Member   talos-default-controlplane-1   1         talos-default-controlplane-1   controlplane   Talos (v1.2.3)   ["10.5.0.2"]
10.5.0.2   cluster     Member   talos-default-worker-1         1         talos-default-worker-1         worker         Talos (v1.2.3)   ["10.5.0.3"]
```

There are currently two supported discovery services: a Kubernetes registry (which stores data in the cluster's etcd service) and an external registry service.
Sidero Labs runs a public external registry service, which is enabled by default.
The Kubernetes registry service is disabled by default.
The advantage of the external registry service is that it is not dependent on etcd, and thus can inform you of cluster membership even when Kubernetes is down.

> Note: Kubernetes registry is deprecated as it is not compatible with Kubernetes 1.32 and later versions in the default configuration.

## Video Walkthrough

To see a live demo of Cluster Discovery, see the video below:

<iframe width="560" height="315" src="https://www.youtube.com/embed/GCBTrHhjawY" frameborder="0" allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture" allowfullscreen></iframe>

## Registries

Peers are aggregated from enabled registries.
By default, Talos will use the `service` registry, while the `kubernetes` registry is disabled.
To disable a registry, set `disabled` to `true` (this option is the same for all registries):
For example, to disable the `service` registry:

```yaml
cluster:
  discovery:
    enabled: true
    registries:
      service:
        disabled: true
```

Disabling all registries effectively disables member discovery.

> Note: An enabled discovery service is required for [KubeSpan]({{< relref "../talos-guides/network/kubespan/" >}}) to function correctly.

### Kubernetes Registry

The `Kubernetes` registry uses Kubernetes `Node` resource data and additional Talos annotations:

```sh
$ kubectl describe node <nodename>
Annotations:        cluster.talos.dev/node-id: Utoh3O0ZneV0kT2IUBrh7TgdouRcUW2yzaaMl4VXnCd
                    networking.talos.dev/assigned-prefixes: 10.244.0.0/32,10.244.0.1/24
                    networking.talos.dev/self-ips: 172.20.0.2,fd83:b1f7:fcb5:2802:8c13:71ff:feaf:7c94
...
```

> Note: Starting with Kubernetes 1.32, the feature gate `AuthorizeNodeWithSelectors` enables additional authorization for `Node` resource read access via `system:node:*` role.
> This prevents Talos Kubernetes registry from functioning correctly.
> The workaround is to disable the feature gate on the API server, but it's not recommended as it disables also other important security protections.
> For this reason, the Kubernetes registry is deprecated and disabled by default.

### Discovery Service Registry

The `Service` registry by default uses a public external Discovery Service to exchange encrypted information about cluster members.

> Note: Talos supports operations when Discovery Service is disabled, but some features will rely on Kubernetes API availability to discover
> controlplane endpoints, so in case of a failure disabled Discovery Service makes troubleshooting much harder.

Sidero Labs maintains a public discovery service at `https://discovery.talos.dev/` whereby cluster members use a shared key that is globally unique to coordinate basic connection information (i.e. the set of possible "endpoints", or IP:port pairs).
We call this data "affiliate data."
This data is encrypted by Talos Linux before being sent to the discovery service, and it can only be decrypted by the cluster members.

> Note: If KubeSpan is enabled the data has the addition of the WireGuard public key.

Data sent to the discovery service is encrypted with AES-GCM encryption and endpoint data is separately encrypted with AES in ECB mode so that endpoints coming from different sources can be deduplicated server-side.
Each node submits its own data, plus the endpoints it sees from other peers, to the discovery service.
The discovery service aggregates the data, deduplicates the endpoints, and sends updates to each connected peer.
Each peer receives information back from the discovery service, decrypts it and uses it to drive KubeSpan and cluster discovery.

Data is stored in memory only (and snapshotted to disk in encrypted way to facilitate quick recovery on restarts).
The cluster ID is used as a key to select the affiliates (so that different clusters see different affiliates).

To summarize, the discovery service knows the client version, cluster ID, the number of affiliates, some encrypted data for each affiliate, and a list of encrypted endpoints.
The discovery service doesn’t see actual node information – it only stores and updates encrypted blobs.
Discovery data is encrypted/decrypted by the clients – the cluster members.
The discovery service does not have the encryption key.

The discovery service may, with a commercial license, be operated by your organization and can be [downloaded here](https://github.com/siderolabs/discovery-service).
In order for nodes to communicate to the discovery service, they must be able to connect to it on TCP port 443.

## Resource Definitions

Talos provides resources that can be used to introspect the discovery and KubeSpan features.

### Discovery

#### Identities

The node's unique identity (base62 encoded random 32 bytes) can be obtained with:

> Note: Using base62 allows the ID to be URL encoded without having to use the ambiguous URL-encoding version of base64.

```sh
$ talosctl get identities -o yaml
...
spec:
    nodeId: Utoh3O0ZneV0kT2IUBrh7TgdouRcUW2yzaaMl4VXnCd
```

Node identity is used as the unique `Affiliate` identifier.

Node identity resource is preserved in the [STATE]({{< relref "../learn-more/architecture/#file-system-partitions" >}}) partition in `node-identity.yaml` file.
Node identity is preserved across reboots and upgrades, but it is regenerated if the node is reset (wiped).

#### Affiliates

An affiliate is a proposed member: the node has the same cluster ID and secret.

```sh
$ talosctl get affiliates
ID                                             VERSION   HOSTNAME                       MACHINE TYPE   ADDRESSES
2VfX3nu67ZtZPl57IdJrU87BMjVWkSBJiL9ulP9TCnF    2         talos-default-controlplane-2   controlplane   ["172.20.0.3","fd83:b1f7:fcb5:2802:986b:7eff:fec5:889d"]
6EVq8RHIne03LeZiJ60WsJcoQOtttw1ejvTS6SOBzhUA   2         talos-default-worker-1         worker         ["172.20.0.5","fd83:b1f7:fcb5:2802:cc80:3dff:fece:d89d"]
NVtfu1bT1QjhNq5xJFUZl8f8I8LOCnnpGrZfPpdN9WlB   2         talos-default-worker-2         worker         ["172.20.0.6","fd83:b1f7:fcb5:2802:2805:fbff:fe80:5ed2"]
Utoh3O0ZneV0kT2IUBrh7TgdouRcUW2yzaaMl4VXnCd    4         talos-default-controlplane-1   controlplane   ["172.20.0.2","fd83:b1f7:fcb5:2802:8c13:71ff:feaf:7c94"]
b3DebkPaCRLTLLWaeRF1ejGaR0lK3m79jRJcPn0mfA6C   2         talos-default-controlplane-3   controlplane   ["172.20.0.4","fd83:b1f7:fcb5:2802:248f:1fff:fe5c:c3f"]
```

One of the `Affiliates` with the `ID` matching node identity is populated from the node data, other `Affiliates` are pulled from the registries.
Enabled discovery registries run in parallel and discovered data is merged to build the list presented above.

Details about data coming from each registry can be queried from the `cluster-raw` namespace:

```sh
$ talosctl get affiliates --namespace=cluster-raw
ID                                                     VERSION   HOSTNAME                       MACHINE TYPE   ADDRESSES
k8s/2VfX3nu67ZtZPl57IdJrU87BMjVWkSBJiL9ulP9TCnF        3         talos-default-controlplane-2   controlplane   ["172.20.0.3","fd83:b1f7:fcb5:2802:986b:7eff:fec5:889d"]
k8s/6EVq8RHIne03LeZiJ60WsJcoQOtttw1ejvTS6SOBzhUA       2         talos-default-worker-1         worker         ["172.20.0.5","fd83:b1f7:fcb5:2802:cc80:3dff:fece:d89d"]
k8s/NVtfu1bT1QjhNq5xJFUZl8f8I8LOCnnpGrZfPpdN9WlB       2         talos-default-worker-2         worker         ["172.20.0.6","fd83:b1f7:fcb5:2802:2805:fbff:fe80:5ed2"]
k8s/b3DebkPaCRLTLLWaeRF1ejGaR0lK3m79jRJcPn0mfA6C       3         talos-default-controlplane-3   controlplane   ["172.20.0.4","fd83:b1f7:fcb5:2802:248f:1fff:fe5c:c3f"]
service/2VfX3nu67ZtZPl57IdJrU87BMjVWkSBJiL9ulP9TCnF    23        talos-default-controlplane-2   controlplane   ["172.20.0.3","fd83:b1f7:fcb5:2802:986b:7eff:fec5:889d"]
service/6EVq8RHIne03LeZiJ60WsJcoQOtttw1ejvTS6SOBzhUA   26        talos-default-worker-1         worker         ["172.20.0.5","fd83:b1f7:fcb5:2802:cc80:3dff:fece:d89d"]
service/NVtfu1bT1QjhNq5xJFUZl8f8I8LOCnnpGrZfPpdN9WlB   20        talos-default-worker-2         worker         ["172.20.0.6","fd83:b1f7:fcb5:2802:2805:fbff:fe80:5ed2"]
service/b3DebkPaCRLTLLWaeRF1ejGaR0lK3m79jRJcPn0mfA6C   14        talos-default-controlplane-3   controlplane   ["172.20.0.4","fd83:b1f7:fcb5:2802:248f:1fff:fe5c:c3f"]
```

Each `Affiliate` ID is prefixed with `k8s/` for data coming from the Kubernetes registry and with `service/` for data coming from the discovery service.

#### Members

A member is an affiliate that has been approved to join the cluster.
The members of the cluster can be obtained with:

```sh
$ talosctl get members
ID                             VERSION   HOSTNAME                       MACHINE TYPE   OS                ADDRESSES
talos-default-controlplane-1   2         talos-default-controlplane-1   controlplane   Talos ({{< release >}})   ["172.20.0.2","fd83:b1f7:fcb5:2802:8c13:71ff:feaf:7c94"]
talos-default-controlplane-2   1         talos-default-controlplane-2   controlplane   Talos ({{< release >}})   ["172.20.0.3","fd83:b1f7:fcb5:2802:986b:7eff:fec5:889d"]
talos-default-controlplane-3   1         talos-default-controlplane-3   controlplane   Talos ({{< release >}})   ["172.20.0.4","fd83:b1f7:fcb5:2802:248f:1fff:fe5c:c3f"]
talos-default-worker-1         1         talos-default-worker-1         worker         Talos ({{< release >}})   ["172.20.0.5","fd83:b1f7:fcb5:2802:cc80:3dff:fece:d89d"]
talos-default-worker-2         1         talos-default-worker-2         worker         Talos ({{< release >}})   ["172.20.0.6","fd83:b1f7:fcb5:2802:2805:fbff:fe80:5ed2"]
```
