# KubeSpan

## Routing

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

## Routing Table

The routing table (number 180 by default) is simple, containing a single route for each family:  send everything through the Wireguard interface.

## NFTables

The logic inside NFTables is fairly simple.
First, everything is compiled into a single table:  `talos_kubespan`.

Next, two chains are set up:  one for the `prerouting` hook (`kubespan_prerouting`)
and the other for the `outgoing` hook (`kubespan_outgoing`).

We define two sets of target IP prefixes:  one for IPv6 (`kubespan_targets_ipv6`)
and the other for IPv4 (`kubespan_targets_ipv4`).

Last, we add rules to each chain which basically specify:

 1. If the packet is marked as _from_ Wireguard (`0x51820` by default), just accept it and terminate
    the chain.
 2. If the packet matches an IP in either of the target IP sets, mark that
    packet with the _to_ Wireguard mark (`0x51821` by default).

## Rules

There are two route rules defined:  one to match IPv6 packets and the other to
match IPv4 packets.

These rules say the same thing for each:  if the packet is marked that it should
go _to_ Wireguard (fwmark `0x51821` by default), send it to the Wireguard
routing table (180 by default).
