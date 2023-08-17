---
title: "KubeSpan"
weight: 100
description: "Understand more about KubeSpan for Talos Linux."
---

## WireGuard Peer Discovery

The key pieces of information needed for WireGuard generally are:

- the public key of the host you wish to connect to
- an IP address and port of the host you wish to connect to

The latter is really only required of _one_ side of the pair.
Once traffic is received, that information is learned and updated by WireGuard automatically.

Kubernetes, though, also needs to know which traffic goes to which WireGuard peer.
Because this information may be dynamic, we need a way to keep this information up to date.

If we already have a connection to Kubernetes, it's fairly easy: we can just keep that information in Kubernetes.
Otherwise, we have to have some way to discover it.

Talos Linux implements a multi-tiered approach to gathering this information.
Each tier can operate independently, but the amalgamation of the mechanisms produces a more robust set of connection criteria.

These mechanisms are:

- an external service
- a Kubernetes-based system

See [discovery service]({{< relref "../talos-guides/discovery" >}}) to learn more about the external service.

The Kubernetes-based system utilizes annotations on Kubernetes Nodes which describe each node's public key and local addresses.

On top of this, KubeSpan can optionally route Pod subnets.
This is usually taken care of by the CNI, but there are many situations where the CNI fails to be able to do this itself, across networks.

## NAT, Multiple Routes, Multiple IPs

One of the difficulties in communicating across networks is that there is often not a single address and port which can identify a connection for each node on the system.
For instance, a node sitting on the same network might see its peer as `192.168.2.10`, but a node across the internet may see it as `2001:db8:1ef1::10`.

We need to be able to handle any number of addresses and ports, and we also need to have a mechanism to _try_ them.
WireGuard only allows us to select one at a time.

KubeSpan implements a controller which continuously discovers and rotates these IP:port pairs until a connection is established.
It then starts trying again if that connection ever fails.

## Packet Routing

After we have established a WireGuard connection, we have to make sure that the right packets get sent to the WireGuard interface.

WireGuard supplies a convenient facility for tagging packets which come from _it_, which is great.
But in our case, we need to be able to allow traffic which both does _not_ come from WireGuard and _also_ is not destined for another Kubernetes node to flow through the normal mechanisms.

Unlike many corporate or privacy-oriented VPNs, we need to allow general internet traffic to flow normally.

Also, as our cluster grows, this set of IP addresses can become quite large and quite dynamic.
This would be very cumbersome and slow in `iptables`.
Luckily, the kernel supplies a convenient mechanism by which to define this arbitrarily large set of IP addresses: IP sets.

Talos collects all of the IPs and subnets which are considered "in-cluster" and maintains these in the kernel as an IP set.

Now that we have the IP set defined, we need to tell the kernel how to use it.

The traditional way of doing this would be to use `iptables`.
However, there is a big problem with IPTables.
It is a common namespace in which any number of other pieces of software may dump things.
We have no surety that what we add will not be wiped out by something else (from Kubernetes itself, to the CNI, to some workload application), be rendered unusable by higher-priority rules, or just generally cause trouble and conflicts.

Instead, we use a three-pronged system which is both more foundational and less centralised.

NFTables offers a separately namespaced, decentralised way of marking packets for later processing based on IP sets.
Instead of a common set of well-known tables, NFTables uses hooks into the kernel's netfilter system, which are less vulnerable to being usurped, bypassed, or a source of interference than IPTables, but which are rendered down by the kernel to the same underlying XTables system.

Our NFTables system is where we store the IP sets.
Any packet which enters the system, either by forward from inside Kubernetes or by generation from the host itself, is compared against a hash table of this IP set.
If it is matched, it is marked for later processing by our next stage.
This is a high-performance system which exists fully in the kernel and which ultimately becomes an eBPF program, so it scales well to hundreds of nodes.

The next stage is the kernel router's route rules.
These are defined as a common ordered list of operations for the whole operating system, but they are intended to be tightly constrained and are rarely used by applications in any case.
The rules we add are very simple: if a packet is marked by our NFTables system, send it to an alternate routing table.

This leads us to our third and final stage of packet routing.
We have a custom routing table with two rules:

- send all IPv4 traffic to the WireGuard interface
- send all IPv6 traffic to the WireGuard interface

So in summary, we:

- mark packets destined for Kubernetes applications or Kubernetes nodes
- send marked packets to a special routing table
- send anything which is sent to that routing table through the WireGuard interface

This gives us an isolated, resilient, tolerant, and non-invasive way to route Kubernetes traffic safely, automatically, and transparently through WireGuard across almost any set of network topologies.

## Design Decisions

### Routing

Routing for Wireguard is a touch complicated when the set of possible peer
endpoints includes at least one member of the set of _destinations_.
That is, packets from Wireguard to a peer endpoint should not be sent to
Wireguard, lest a loop be created.

In order to handle this situation, Wireguard provides the ability to mark
packets which it generates, so their routing can be handled separately.

In our case, though, we actually want the inverse of this:  we want to route
Wireguard packets however the normal networking routes and rules say they should
be routed, while packets destined for the other side of Wireguard Peers should
be forced into Wireguard interfaces.

While IP Rules allow you to invert matches, they do not support matching based
on IP sets.
That means, to use simple rules, we would have to add a rule for
each destination, which could reach into hundreds or thousands of rules to
manage.
This is not really much of a performance issue, but it is a management
issue, since it is expected that we would not be the only manager of rules in
the system, and rules offer no facility to tag for ownership.

IP Sets are supported by IPTables, and we could integrate there.
However, IPTables exists in a global namespace, which makes it fragile having
multiple parties manipulating it.
The newer NFTables replacement for IPTables, though, allows users to
independently hook into various points of XTables, keeping all such rules and
sets independent.
This means that regardless of what CNIs or other user-side routing rules may do,
our KubeSpan setup will not be messed up.

Therefore, we utilise NFTables (which natively supports IP sets and owner
grouping) instead, to mark matching traffic which should be sent to the
Wireguard interface.
This way, we can keep all our KubeSpan set logic in one place, allowing us to
simply use a single `ip rule` match:
for our fwmark, and sending those matched packets to a separate routing table
with one rule: default to the wireguard interface.

So we have three components:

  1. A routing table for Wireguard-destined packets
  2. An NFTables table which defines the set of destinations packets to which will
     be marked with our firewall mark.
      - Hook into PreRouting (type Filter)
      - Hook into Outgoing (type Route)
  3. One IP Rule which sends packets marked with our firewall mark to our Wireguard
     routing table.

### Routing Table

The routing table (number 180 by default) is simple, containing a single route for each family:  send everything through the Wireguard interface.

### NFTables

The logic inside NFTables is fairly simple.
First, everything is compiled into a single table:  `talos_kubespan`.

Next, two chains are set up:  one for the `prerouting` hook (`kubespan_prerouting`)
and the other for the `outgoing` hook (`kubespan_outgoing`).

We define two sets of target IP prefixes:  one for IPv6 (`kubespan_targets_ipv6`)
and the other for IPv4 (`kubespan_targets_ipv4`).

Last, we add rules to each chain which basically specify:

 1. If the packet is marked as _from_ Wireguard, just accept it and terminate
    the chain.
 2. If the packet matches an IP in either of the target IP sets, mark that
    packet with the _to_ Wireguard mark.

### Rules

There are two route rules defined:  one to match IPv6 packets and the other to
match IPv4 packets.

These rules say the same thing for each:  if the packet is marked that it should
go _to_ Wireguard, send it to the Wireguard
routing table.

### Firewall Mark

KubeSpan is using only two bits of the firewall mark with the mask `0x00000060`.

> Note: if other software on the node is using the bits `0x60` of the firewall mark, this
> might cause conflicts and break KubeSpan.
>
> At the moment of the writing, it was confirmed that Calico CNI is using bits `0xffff0000` and
> Cilium CNI is using bits `0xf00`, so KubeSpan is compatible with both.
> Flannel CNI uses `0x4000` mask, so it is also compatible.

In the routing rules table, we match on the mark `0x40` with the mask `0x60`:

```text
32500: from all fwmark 0x40/0x60 lookup 180
```

In the NFTables table, we match with the same mask `0x60` and we set the mask by only modifying
bits from the `0x60` mask:

```text
meta mark & 0x00000060 == 0x00000020 accept
ip daddr @kubespan_targets_ipv4 meta mark set meta mark & 0xffffffdf | 0x00000040 accept
ip6 daddr @kubespan_targets_ipv6 meta mark set meta mark & 0xffffffdf | 0x00000040 accept
```
