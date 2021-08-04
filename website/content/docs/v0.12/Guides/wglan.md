---
title: Wireguard LAN
---

**FEATURE STATE**: alpha

When building hybrid, multi-cloud, or multi-site clusters, the ability to
securely and easily extend a single cluster across multiple locations is greatly
desired.

Talos includes a set of tools which can be simply enabled to automatically form
a secure multi-site network using Wireguard:  WgLAN.

## Configure your Talos Machines

For most installations, you need only enable the relevant flags on a Wireguard
interface:

```yaml
machine:
  network:
    interfaces:
    - interface: wglan0
      wireguard:
        enableAutomaticNodes: true
        enablePodNetworking: true
```

As a configuration patch, this may look like:

```json
[
   {
      "op": "add",
      "path": "/machine/network/interfaces",
      "value": [
         {
            "interface": "eth0",
            "dhcp": true
         },
         {
            "interface": "wglan0",
            "wireguard": {
               "enableAutomaticNodes": true,
               "enablePodNetworking": true
            }
         }
      ]
   }
]
```

Most of the magic if enabled by `enableAutomaticNodes`.
Turning this on will cause each node to:

- generate its own Wireguard IP address
- generate necessary public and private keys
- discover peer public keys through multiple mechanisms
- automatically determine a route to each peer, regardless of NAT
- dynamically sweep through available possibilities to keep peers connected
- dynamically add routes to each peer according to its connected IPs
- generate firewall and routing rules to facilitate transparent communications

The `enablePodNetworking` feature adds routing for Pod-to-Pod networking, too.
While this feature is usually handled by CNI tools, it can be handy to use as a
backup or for cases where the CNI fails or does not sufficiently handle
Pod-to-Pod networking on the host.
It is generally safe to enable this feature even if the CNI handles this job.

In order for this system to work, all nodes in the cluster must have these flags
enabled.

Also, at least one node must have its wireguard port (51820/udp by default) exposed
to all others.
Usually and by recommendation, this is a control plane node.

## Internals

WgLAN is built on the assumption of simplicity and reasonable defaults.
Many critical values are adjustable for specific scenarios, but no settings
beyond the enabling flags for most installations.

WgLAN is constructed from a number of pieces:

- Address generation:  IPv6 EUI64-style address generation based on an
     RFC4193 Unique Local Addressing prefix deterministrically generated from
     a cluster-unique identifier.
     While this is an IPv6 address, you are free to use any combination of IPv4
     and IPv6 addresses in the rest of your cluster.  
     There is no need for you to enable IPv6 in Kubernetes at all.
- Public Key discovery: WgLAN uses any of a set of discovery mechanisms for
     determining its peers, their public keys, and the addresses which should be
     routed to each.
     First is through annotations on the Kubernetes Node.
     Next is a secure external service (secured by a SHA-256 hash of your
     cluster token by default or any other value of your choosing).
     The [external service](https://github.com/talos-systems/wglan-controller)
     is open source, so you may run your own in airgapped environments, as well.
- Peer manager: each Talos nodes regularly watches for changes to the set of
     Nodes and the addresses associated with them.
     When they change, the peer manager will update the Wireguard configuration.
- Rules manager: the Linux routing system has three components which WgLAN uses to
     send packets to the right place.
     These are nftables, a secondary routing table, and routing rules.
