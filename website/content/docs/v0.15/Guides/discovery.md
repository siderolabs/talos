---
title: "Discovery"
---

## Video Walkthrough

To see a live demo of Cluster Discovery, see the video below:

<iframe width="560" height="315" src="https://www.youtube.com/embed/GCBTrHhjawY" frameborder="0" allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture" allowfullscreen></iframe>

## Registries

Peers are aggregated from a number of optional registries.
By default, Talos will use the `kubernetes` and `service` registries.
Either one can be disabled.
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

Disabling all registries effectively disables member discovery altogether.

> As of v0.14, Talos supports the `kubernetes` and `service` registries.

`Kubernetes` registry uses Kubernetes `Node` resource data and additional Talos annotations:

```sh
$ kubectl describe node <nodename>
Annotations:        cluster.talos.dev/node-id: Utoh3O0ZneV0kT2IUBrh7TgdouRcUW2yzaaMl4VXnCd
                    networking.talos.dev/assigned-prefixes: 10.244.0.0/32,10.244.0.1/24
                    networking.talos.dev/self-ips: 172.20.0.2,fd83:b1f7:fcb5:2802:8c13:71ff:feaf:7c94
...
```

`Service` registry uses external [Discovery Service](../../learn-more/discovery/) to exchange encrypted information about cluster members.

## Resource Definitions

Talos v0.14 introduces seven new resources that can be used to introspect the new discovery and KubeSpan features.

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

Node identity resource is preserved in the `STATE` partition in `node-identity.yaml` file.
Node identity is preserved across reboots and upgrades, but it is regenerated if the node is reset (wiped).

#### Affiliates

An affiliate is a proposed member attributed to the fact that the node has the same cluster ID and secret.

```sh
$ talosctl get affiliates
ID                                             VERSION   HOSTNAME                 MACHINE TYPE   ADDRESSES
2VfX3nu67ZtZPl57IdJrU87BMjVWkSBJiL9ulP9TCnF    2         talos-default-master-2   controlplane   ["172.20.0.3","fd83:b1f7:fcb5:2802:986b:7eff:fec5:889d"]
6EVq8RHIne03LeZiJ60WsJcoQOtttw1ejvTS6SOBzhUA   2         talos-default-worker-1   worker         ["172.20.0.5","fd83:b1f7:fcb5:2802:cc80:3dff:fece:d89d"]
NVtfu1bT1QjhNq5xJFUZl8f8I8LOCnnpGrZfPpdN9WlB   2         talos-default-worker-2   worker         ["172.20.0.6","fd83:b1f7:fcb5:2802:2805:fbff:fe80:5ed2"]
Utoh3O0ZneV0kT2IUBrh7TgdouRcUW2yzaaMl4VXnCd    4         talos-default-master-1   controlplane   ["172.20.0.2","fd83:b1f7:fcb5:2802:8c13:71ff:feaf:7c94"]
b3DebkPaCRLTLLWaeRF1ejGaR0lK3m79jRJcPn0mfA6C   2         talos-default-master-3   controlplane   ["172.20.0.4","fd83:b1f7:fcb5:2802:248f:1fff:fe5c:c3f"]
```

One of the `Affiliates` with the `ID` matching node identity is populated from the node data, other `Affiliates` are pulled from the registries.
Enabled discovery registries run in parallel and discovered data is merged to build the list presented above.

Details about data coming from each registry can be queried from the `cluster-raw` namespace:

```sh
$ talosctl get affiliates --namespace=cluster-raw
ID                                                     VERSION   HOSTNAME                 MACHINE TYPE   ADDRESSES
k8s/2VfX3nu67ZtZPl57IdJrU87BMjVWkSBJiL9ulP9TCnF        3         talos-default-master-2   controlplane   ["172.20.0.3","fd83:b1f7:fcb5:2802:986b:7eff:fec5:889d"]
k8s/6EVq8RHIne03LeZiJ60WsJcoQOtttw1ejvTS6SOBzhUA       2         talos-default-worker-1   worker         ["172.20.0.5","fd83:b1f7:fcb5:2802:cc80:3dff:fece:d89d"]
k8s/NVtfu1bT1QjhNq5xJFUZl8f8I8LOCnnpGrZfPpdN9WlB       2         talos-default-worker-2   worker         ["172.20.0.6","fd83:b1f7:fcb5:2802:2805:fbff:fe80:5ed2"]
k8s/b3DebkPaCRLTLLWaeRF1ejGaR0lK3m79jRJcPn0mfA6C       3         talos-default-master-3   controlplane   ["172.20.0.4","fd83:b1f7:fcb5:2802:248f:1fff:fe5c:c3f"]
service/2VfX3nu67ZtZPl57IdJrU87BMjVWkSBJiL9ulP9TCnF    23        talos-default-master-2   controlplane   ["172.20.0.3","fd83:b1f7:fcb5:2802:986b:7eff:fec5:889d"]
service/6EVq8RHIne03LeZiJ60WsJcoQOtttw1ejvTS6SOBzhUA   26        talos-default-worker-1   worker         ["172.20.0.5","fd83:b1f7:fcb5:2802:cc80:3dff:fece:d89d"]
service/NVtfu1bT1QjhNq5xJFUZl8f8I8LOCnnpGrZfPpdN9WlB   20        talos-default-worker-2   worker         ["172.20.0.6","fd83:b1f7:fcb5:2802:2805:fbff:fe80:5ed2"]
service/b3DebkPaCRLTLLWaeRF1ejGaR0lK3m79jRJcPn0mfA6C   14        talos-default-master-3   controlplane   ["172.20.0.4","fd83:b1f7:fcb5:2802:248f:1fff:fe5c:c3f"]
```

Each `Affiliate` ID is prefixed with `k8s/` for data coming from the Kubernetes registry and with `service/` for data coming from the discovery service.

#### Members

A member is an affiliate that has been approved to join the cluster.
The members of the cluster can be obtained with:

```sh
$ talosctl get members
ID                       VERSION   HOSTNAME                 MACHINE TYPE   OS                ADDRESSES
talos-default-master-1   2         talos-default-master-1   controlplane   Talos (v0.15.0)   ["172.20.0.2","fd83:b1f7:fcb5:2802:8c13:71ff:feaf:7c94"]
talos-default-master-2   1         talos-default-master-2   controlplane   Talos (v0.15.0)   ["172.20.0.3","fd83:b1f7:fcb5:2802:986b:7eff:fec5:889d"]
talos-default-master-3   1         talos-default-master-3   controlplane   Talos (v0.15.0)   ["172.20.0.4","fd83:b1f7:fcb5:2802:248f:1fff:fe5c:c3f"]
talos-default-worker-1   1         talos-default-worker-1   worker         Talos (v0.15.0)   ["172.20.0.5","fd83:b1f7:fcb5:2802:cc80:3dff:fece:d89d"]
talos-default-worker-2   1         talos-default-worker-2   worker         Talos (v0.15.0)   ["172.20.0.6","fd83:b1f7:fcb5:2802:2805:fbff:fe80:5ed2"]
```
