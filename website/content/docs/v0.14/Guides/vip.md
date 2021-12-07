---
title: Virtual (shared) IP
---

One of the biggest pain points when building a high-availability controlplane
is giving clients a single IP or URL at which they can reach any of the controlplane nodes.
The most common approaches all require external resources:  reverse proxy, load
balancer, BGP, and DNS.

Using a "Virtual" IP address, on the other hand, provides high availability
without external coordination or resources, so long as the controlplane members
share a layer 2 network.
In practical terms, this means that they are all connected via a switch, with no
router in between them.

The term "virtual" is misleading here.
The IP address is real, and it is assigned to an interface.
Instead, what actually happens is that the controlplane machines vie for
control of the shared IP address.
There can be only one owner of the IP address at any given time, but if that
owner disappears or becomes non-responsive, another owner will be chosen,
and it will take up the mantle: the IP address.

Talos has (as of version 0.9) built-in support for this form of shared IP address,
and it can utilize this for both the Kubernetes API server and the Talos endpoint set.
Talos uses `etcd` for elections and leadership (control) of the IP address.

## Video Walkthrough

To see a live demo of this writeup, see the video below:

<iframe width="560" height="315" src="https://www.youtube.com/embed/BfMGInHtFBc" frameborder="0" allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture" allowfullscreen></iframe>

## Choose your Shared IP

To begin with, you should choose your shared IP address.
It should generally be a reserved, unused IP address in the same subnet as
your controlplane nodes.
It should not be assigned or assignable by your DHCP server.

For our example, we will assume that the controlplane nodes have the following
IP addresses:

- `192.168.0.10`
- `192.168.0.11`
- `192.168.0.12`

We then choose our shared IP to be:

> 192.168.0.15

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

Obviously, for your own environment, the interface and the DHCP setting may
differ.
You are free to use static addressing (`cidr`) instead of DHCP.

## Caveats

In general, the shared IP should just work.
However, since it relies on `etcd` for elections, the shared IP will not come
alive until after you have bootstrapped Kubernetes.
In general, this is not a problem, but it does mean that you cannot use the
shared IP when issuing the `talosctl bootstrap` command.
Instead, that command will need to target one of the controlplane nodes
discretely.
