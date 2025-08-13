---
title: "Virtual (shared) IP"
description: "Using Talos Linux to set up a floating virtual IP address for cluster access."
aliases:
  - ../../guides/vip
---

One of the pain points when building a high-availability controlplane
is giving clients a single IP or URL at which they can reach any of the controlplane nodes.
The most common approaches - reverse proxy, load
balancer, BGP, and DNS - all require external resources, and add complexity in setting up Kubernetes.

To simplify cluster creation, Talos Linux supports a "Virtual" IP (VIP) address to access the Kubernetes API server, providing high availability with no other resources required.

What happens is that the controlplane machines vie for control of the shared IP address using etcd elections.
There can be only one owner of the IP address at any given time.
If that owner disappears or becomes non-responsive, another owner will be chosen,
and it will take up the IP address.

### Requirements

The controlplane nodes must share a layer 2 network, and the virtual IP must be assigned from that shared network subnet.
In practical terms, this means that they are all connected via a switch, with no router in between them.
Note that the virtual IP election depends on `etcd` being up, as Talos uses `etcd` for elections and leadership (control) of the IP address.

The virtual IP is not restricted by ports - you can access any port that the control plane nodes are listening on, on that IP address.
Thus it *is* possible to access the Talos API over the VIP, but it is *not recommended*, as you cannot access the VIP when etcd is down - and then you could not access the Talos API to recover etcd.

## Video Walkthrough

To see a live demo of this writeup, see the video below:

<iframe width="560" height="315" src="https://www.youtube.com/embed/BfMGInHtFBc" frameborder="0" allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture" allowfullscreen></iframe>

## Choose your Shared IP

The Virtual IP should be a reserved, unused IP address in the same subnet as
your controlplane nodes.
It should not be assigned or assignable by your DHCP server.

For our example, we will assume that the controlplane nodes have the following
IP addresses:

- `192.168.0.10`
- `192.168.0.11`
- `192.168.0.12`

We then choose our shared IP to be:

- `192.168.0.15`

## Configure your Talos Machines

The shared IP setting is only valid for controlplane nodes.

For the example above, each of the controlplane nodes should have the following
Machine Config snippet:

```yaml
machine:
  network:
    interfaces:
    - interface: eth0
      dhcp: true
      vip:
        ip: 192.168.0.15
```

Virtual IP's can also be configured on a VLAN interface.

```yaml
machine:
  network:
    interfaces:
    - interface: eth0
      dhcp: true
      vip:
        ip: 192.168.0.15
      vlans:
        - vlanId: 100
          dhcp: true
          vip:
            ip: 192.168.1.15
```

For your own environment, the interface and the DHCP setting may differ, or you may
use static addressing (`adresses`) instead of DHCP.

When using [predictable interface names]({{< relref "./predictable-interface-names" >}}), the interface name might not be `eth0`.

If the machine has a single network interface, it can be selected using a dummy device selector:

```yaml
machine:
  network:
    interfaces:
      - deviceSelector:
          physical: true # should select any hardware network device, if you have just one, it will be selected
        dhcp: true
        vip:
          ip: 192.168.0.15
```

## Caveats

Since VIP functionality relies on `etcd` for elections, the shared IP will not come
alive until after you have bootstrapped Kubernetes.

Don't use the VIP as the `endpoint` in the `talosconfig`, as the VIP is bound to `etcd` and `kube-apiserver` health, and you will not be able to recover from a failure of either of those components using Talos API.
