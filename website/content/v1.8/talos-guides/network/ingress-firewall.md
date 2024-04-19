---
title: "Ingress Firewall"
description: "Learn to use Talos Linux Ingress Firewall to limit access to the host services."
---

Talos Linux Ingress Firewall is a simple and effective way to limit access to the services running on the host, which includes both Talos standard
services (e.g. `apid` and `kubelet`), and any additional workloads that may be running on the host.
Talos Linux Ingress Firewall doesn't affect the traffic between the Kubernetes pods/services, please use CNI Network Policies for that.

## Configuration

Ingress rules are configured as extra documents [NetworkDefaultActionConfig]({{< relref "../../reference/configuration/network/networkdefaultactionconfig.md" >}}) and
[NetworkRuleConfig]({{< relref "../../reference/configuration/network/networkruleconfig.md" >}}) in the Talos machine configuration:

```yaml
apiVersion: v1alpha1
kind: NetworkDefaultActionConfig
ingress: block
---
apiVersion: v1alpha1
kind: NetworkRuleConfig
name: kubelet-ingress
portSelector:
  ports:
    - 10250
  protocol: tcp
ingress:
  - subnet: 172.20.0.0/24
    except: 172.20.0.1/32
```

The first document configures the default action for the ingress traffic, which can be either `accept` or `block`, with the default being `accept`.
If the default action is set to `accept`, then all the ingress traffic will be allowed, unless there is a matching rule that blocks it.
If the default action is set to `block`, then all the ingress traffic will be blocked, unless there is a matching rule that allows it.

With either `accept` or `block`, the traffic is always allowed on the following network interfaces:

* `lo`
* `siderolink`
* `kubespan`

In the `block` mode:

* ICMP and ICMPv6 traffic is also allowed with a rate limit of 5 packets per second
* traffic between Kubernetes pod/service subnets is allowed (for native routing CNIs)

The second document defines an ingress rule for a set of ports and protocols on the host.
The `NetworkRuleConfig` might be repeated many times to define multiple rules, but each document must have a unique name.

The `ports` field accepts either a single port or a port range:

```yaml
portSelector:
  ports:
    - 10250
    - 10260
    - 10300-10400
```

The `protocol` might be either `tcp` or `udp`.

The `ingress` specifies the list of subnets that are allowed to access the host services, with the optional `except` field to exclude a set of addresses from the subnet.

> Note: incorrect configuration of the ingress firewall might result in the host becoming inaccessible over Talos API.
> The configuration might be [applied]({{< relref "../configuration/editing-machine-configuration" >}}) in `--mode=try` to make sure it gets reverted in case of a mistake.

## Recommended Rules

The following rules improve the security of the cluster and cover only standard Talos services.
If there are additional services running with host networking in the cluster, they should be covered by additional rules.

In the `block` mode, the ingress firewall will also block encapsulated traffic (e.g. VXLAN) between the nodes, which needs to be explicitly allowed for the Kubernetes
networking to function properly.
Please refer to the CNI documentation for the specifics, some default configurations are listed below:

* Flannel, Calico: `vxlan` UDP port 4789
* Cilium: `vxlan` UDP port 8472

In the examples we assume following template variables to describe the cluster:

* `$CLUSTER_SUBNET`, e.g. `172.20.0.0/24` - the subnet which covers all machines in the cluster
* `$CP1`, `$CP2`, `$CP3` - the IP addresses of the controlplane nodes
* `$VXLAN_PORT` - the UDP port used by the CNI for encapsulated traffic

### Controlplane

* `apid` and Kubernetes API are wide open
* `kubelet` and `trustd` API is only accessible within the cluster
* `etcd` API is limited to controlplane nodes

```yaml
apiVersion: v1alpha1
kind: NetworkDefaultActionConfig
ingress: block
---
apiVersion: v1alpha1
kind: NetworkRuleConfig
name: kubelet-ingress
portSelector:
  ports:
    - 10250
  protocol: tcp
ingress:
  - subnet: $CLUSTER_SUBNET
---
apiVersion: v1alpha1
kind: NetworkRuleConfig
name: apid-ingress
portSelector:
  ports:
    - 50000
  protocol: tcp
ingress:
  - subnet: 0.0.0.0/0
  - subnet: ::/0
---
apiVersion: v1alpha1
kind: NetworkRuleConfig
name: trustd-ingress
portSelector:
  ports:
    - 50001
  protocol: tcp
ingress:
  - subnet: $CLUSTER_SUBNET
---
apiVersion: v1alpha1
kind: NetworkRuleConfig
name: kubernetes-api-ingress
portSelector:
  ports:
    - 6443
  protocol: tcp
ingress:
  - subnet: 0.0.0.0/0
  - subnet: ::/0
---
apiVersion: v1alpha1
kind: NetworkRuleConfig
name: etcd-ingress
portSelector:
  ports:
    - 2379-2380
  protocol: tcp
ingress:
  - subnet: $CP1/32
  - subnet: $CP2/32
  - subnet: $CP3/32
---
apiVersion: v1alpha1
kind: NetworkRuleConfig
name: cni-vxlan
portSelector:
  ports:
    - $VXLAN_PORT
  protocol: udp
ingress:
  - subnet: $CLUSTER_SUBNET
```

### Worker

* `kubelet` and `apid` API is only accessible within the cluster

```yaml
apiVersion: v1alpha1
kind: NetworkDefaultActionConfig
ingress: block
---
apiVersion: v1alpha1
kind: NetworkRuleConfig
name: kubelet-ingress
portSelector:
  ports:
    - 10250
  protocol: tcp
ingress:
  - subnet: $CLUSTER_SUBNET
---
apiVersion: v1alpha1
kind: NetworkRuleConfig
name: apid-ingress
portSelector:
  ports:
    - 50000
  protocol: tcp
ingress:
  - subnet: $CLUSTER_SUBNET
---
apiVersion: v1alpha1
kind: NetworkRuleConfig
name: cni-vxlan
portSelector:
  ports:
    - $VXLAN_PORT
  protocol: udp
ingress:
  - subnet: $CLUSTER_SUBNET
```

## Learn More

Talos Linux Ingress Firewall is using the `nftables` to perform the filtering.

With the default action set to `accept`, the following rules are applied (example):

```text
table inet talos {
  chain ingress {
    type filter hook input priority filter; policy accept;
    iifname { "lo", "siderolink", "kubespan" }  accept
    ip saddr != { 172.20.0.0/24 } tcp dport { 10250 } drop
    meta nfproto ipv6 tcp dport { 10250 } drop
  }
}
```

With the default action set to `block`, the following rules are applied (example):

```text
table inet talos {
  chain ingress {
    type filter hook input priority filter; policy drop;
    iifname { "lo", "siderolink", "kubespan" }  accept
    ct state { established, related } accept
    ct state invalid drop
    meta l4proto icmp limit rate 5/second accept
    meta l4proto ipv6-icmp limit rate 5/second accept
    ip saddr { 172.20.0.0/24 } tcp dport { 10250 }  accept
    meta nfproto ipv4 tcp dport { 50000 } accept
    meta nfproto ipv6 tcp dport { 50000 } accept
  }
}
```
