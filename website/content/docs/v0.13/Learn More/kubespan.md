---
title: "KubeSpan"
weight: 12
---

## WireGuard Peer Discovery

The key pieces of information needed for WireGuard generally are:

- the public key of the host you wish to connect to
- an IP address and port of the host you wish to connect to

The latter is really only required of _one_ side of the pair.
Once traffic is received, that information is known and updated by WireGuard automatically and internally.

For Kubernetes, though, this is not quite sufficient.
Kubernetes also needs to know which traffic goes to which WireGuard peer.
Because this information may be dynamic, we need a way to be able to constantly keep this information up to date.

If we have a functional connection to Kubernetes otherwise, it's fairly easy: we can just keep that information in Kubernetes.
Otherwise, we have to have some way to discover it.

In our solution, we have a multi-tiered approach to gathering this information.
Each tier can operate independently, but the amalgamation of the tiers produces a more robust set of connection criteria.

For this discussion, we will point out two of these tiers:

- an external service
- a Kubernetes-based system

See [discovery service](../discovery) to learn more about the external service.

The Kubernetes-based system utilises annotations on Kubernetes Nodes which describe each node's public key and local addresses.

On top of this, we also route Pod subnets.
This is often (maybe even usually) taken care of by the CNI, but there are many situations where the CNI fails to be able to do this itself, across networks.
So we also scrape the Kubernetes Node resource to discover its `podCIDRs`.

## NAT, Multiple Routes, Multiple IPs

One of the difficulties in communicating across networks is that there is often not a single address and port which can identify a connection for each node on the system.
For instance, a node sitting on the same network might see its peer as `192.168.2.10`, but a node across the internet may see it as `2001:db8:1ef1::10`.

We need to be able to handle any number of addresses and ports, and we also need to have a mechanism to _try_ them.
WireGuard only allows us to select one at a time.

For our implementation, then, we have built a controller which continuously discovers and rotates these IP:port pairs until a connection is established.
It then starts trying again if that connection ever fails.

## Packet Routing

After we have established a WireGuard connection, our work is not done.
We still have to make sure that the right packets get sent to the WireGuard interface.

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
