---
title: Getting Started
weight: 3
---

This document will walk you through installing a full Talos Cluster.
You may wish to read through the [Quickstart](../quickstart/) first, to quickly create a local virtual cluster on your workstation.

Regardless of where you run Talos, you will find that there is a pattern to deploying it.

In general you will need to:

- acquire the installation image
- decide on the endpoint for Kubernetes
  - optionally create a load balancer
- configure Talos
- configure `talosctl`
- bootstrap Kubernetes

## Prerequisites

### `talosctl`

The `talosctl` tool provides a CLI tool which interfaces with the Talos API in
an easy manner.
It also includes a number of useful tools for creating and managing your clusters.

You should install `talosctl` before continuing:

```bash
curl -Lo /usr/local/bin/talosctl https://github.com/talos-systems/talos/releases/latest/download/talosctl-$(uname -s | tr "[:upper:]" "[:lower:]")-amd64
chmod +x /usr/local/bin/talosctl
```

## Acquire the installation image

The easiest way to install Talos is to use the ISO image.

The latest ISO image can be found on the Github [Releases](https://github.com/talos-systems/talos/releases) page:

- X86: [https://github.com/talos-systems/talos/releases/download/v0.14.0/talos-amd64.iso](https://github.com/talos-systems/talos/releases/download/v0.14.0/talos-amd64.iso)
- ARM64: [https://github.com/talos-systems/talos/releases/download/v0.14.0/talos-arm64.iso](https://github.com/talos-systems/talos/releases/download/v0.14.0/talos-arm64.iso)

For self-built media and network booting, you can use the kernel and initramfs:

- X86: [vmlinuz-amd64](https://github.com/talos-systems/talos/releases/download/v0.14.0/vmlinuz-amd64) [initramfs-amd64.xz](https://github.com/talos-systems/talos/releases/download/v0.14.0/initramfs-amd64.xz)
- ARM64: [vmlinuz-arm64](https://github.com/talos-systems/talos/releases/download/v0.14.0/vmlinuz-arm64) [initramfs-arm64.xz](https://github.com/talos-systems/talos/releases/download/v0.14.0/initramfs-arm64.xz)

When booted from the ISO, Talos will run in RAM, and it will not install itself
until it is provided a configuration.
Thus, it is safe to boot the ISO onto any machine.

### Alternative Booting

If you wish to use a different boot mechanism (such as network boot or a custom ISO), there
are a number of required kernel parameters.

Please see the [kernel](../../reference/kernel/) docs for more information.

## Decide the Kubernetes Endpoint

In order to configure Kubernetes and bootstrap the cluster, Talos needs to know
what the endpoint (DNS name or IP address) of the Kubernetes API Server will be.

The endpoint should be the fully-qualified HTTP(S) URL for the Kubernetes API
Server, which (by default) runs on port 6443 using HTTPS.

Thus, the format of the endpoint may be something like:

- `https://192.168.0.10:6443`
- `https://kube.mycluster.mydomain.com:6443`
- `https://[2001:db8:1234::80]:6443`

Because the Kubernetes controlplane is meant to be supplied in a high
availability manner, we must also choose how to bind it to the servers
themselves.
There are three common ways to do this.

### Dedicated Load-balancer

If you are using a cloud provider or have your own load-balancer available (such
as HAProxy, nginx reverse proxy, or an F5 load-balancer), using
a dedicated load balancer is a natural choice.
Just create an appropriate frontend matching the endpoint, and point the backends at each of the addresses of the Talos controlplane nodes.

This is convenient if a load-balancer is available, but don't worry if that is
not the case.

### Layer 2 Shared IP

Talos has integrated support for serving Kubernetes from a shared (sometimes
called "virtual") IP address.
This method relies on OSI Layer 2 connectivity between controlplane Talos nodes.

In this case, we may choose an IP address on the same subnet as the Talos
controlplane nodes which is not otherwise assigned to any machine.
For instance, if your controlplane node IPs are:

- 192.168.0.10
- 192.168.0.11
- 192.168.0.12

You could choose the ip `192.168.0.15` as your shared IP address.
Just make sure that `192.168.0.15` is not used by any other machine and that your DHCP
will not serve it to any other machine.

Once chosen, form the full HTTPS URL from this IP:

```url
https://192.168.0.15:6443
```

You are also free to set a DNS record to this IP address instead, but you will
still need to use the IP address to set up the shared IP
(`machine.network.interfaces[].vip.ip`) inside the Talos
configuration.

For more information about using a shared IP, see the related
[Guide](../../guides/vip/)

### DNS records

If neither of the other methods work for you, you can instead use DNS records to
provide a measure of redundancy.
In this case, you would add multiple A or AAAA records for a DNS name.

For instance, you could add:

```dns
kube.cluster1.mydomain.com  IN  A  192.168.0.10
kube.cluster1.mydomain.com  IN  A  192.168.0.11
kube.cluster1.mydomain.com  IN  A  192.168.0.12
```

Then, your endpoint would be:

```url
https://kube.cluster1.mydomain.com:6443
```

## Decide how to access the Talos API

Since Talos is entirely API-driven, it is important to know how you are going to
access that API.
Talos comes with a number of mechanisms to make that easier.

Controlplane nodes can proxy requests for worker nodes.
This means that you only need access to the controlplane nodes in order to access
the rest of the network.
This is useful for security (your worker nodes do not need to have
public IPs or be otherwise connected to the Internet), and it also makes working
with highly-variable clusters easier, since you only need to know the
controlplane nodes in advance.

Even better, the `talosctl` tool will automatically load balance and fail over
between all of your controlplane nodes, so long as it is informed of each of the
controlplane node IPs.

That does, of course, present the problem that you need to know how to talk to
the controlplane nodes.
In some environments, it is easy to be able to forecast, prescribe, or discover
the controlplane node IP addresses.
For others, though, even the controlplane nodes are dynamic, unpredictable, and
undiscoverable.

The dynamic options above for the Kubernetes API endpoint also apply to the
Talos API endpoints.
The difference is that the Talos API runs on port `50000/tcp`.

Whichever way you wish to access the Talos API, be sure to note the IP(s) or
hostname(s) so that you can configure your `talosctl` tool's `endpoints` below.

## Configure Talos

When Talos boots without a configuration, such as when using the Talos ISO, it
enters a limited maintenance mode and waits for a configuration to be provided.

Alternatively, the Talos installer can be booted with the `talos.config` kernel
commandline argument set to an HTTP(s) URL from which it should receive its
configuration.
In cases where a PXE server can be available, this is much more efficient than
manually configuring each node.
If you do use this method, just note that Talos does require a number of other
kernel commandline parameters.
See the [required kernel parameters](../../reference/kernel/) for more information.

In either case, we need to generate the configuration which is to be provided.
Luckily, the `talosctl` tool comes with a configuration generator for exactly
this purpose.

```sh
  talosctl gen config "cluster-name" "cluster-endpoint"
```

Here, `cluster-name` is an arbitrary name for the cluster which will be used
in your local client configuration as a label.
It does not affect anything in the cluster itself.
It is arbitrary, but it should be unique in the configuration on your local workstation.

The `cluster-endpoint` is where you insert the Kubernetes Endpoint you
selected from above.
This is the Kubernetes API URL, and it should be a complete URL, with `https://`
and port, if not `443`.
The default port is `6443`, so the port is almost always required.

When you run this command, you will receive a number of files in your current
directory:

- `controlplane.yaml`
- `worker.yaml`
- `talosconfig`

The three `.yaml` files are what we call Machine Configs.
They are installed onto the Talos servers to act as their complete configuration,
describing everything from what disk Talos should be installed to, to what
sysctls to set, to what network settings it should have.
In the case of the `controlplane.yaml`, it even describes how Talos should form its Kubernetes cluster.

The `talosconfig` file (which is also YAML) is your local client configuration
file.

### Controlplane, Init, and Worker

The three types of Machine Configs correspond to the three roles of Talos nodes.
For our purposes, you can ignore the Init type.
It is a legacy type which will go away eventually.
Its purpose was to self-bootstrap.
Instead, we now use an API call to bootstrap the cluster, which is much more robust.

That leaves us with Controlplane and Worker.

The Controlplane Machine Config describes the configuration of a Talos server on
which the Kubernetes Controlplane should run.
The Worker Machine Config describes everything else: workload servers.

The main difference between Controlplane Machine Config files and Worker Machine
Config files is that the former contains information about how to form the
Kubernetes cluster.

### Templates

The generated files can be thought of as templates.
Individual machines may need specific settings (for instance, each may have a
different static IP address).
When different files are needed for machines of the same type, simply
copy the source template (`controlplane.yaml` or `worker.yaml`) and make whatever
modifications need to be done.

For instance, if you had three controlplane nodes and three worker nodes, you
may do something like this:

```bash
  for i in $(seq 0 2); do
    cp controlplane.yaml cp$i.yaml
  end
  for i in $(seq 0 2); do
    cp worker.yaml w$i.yaml
  end
```

In cases where there is no special configuration needed, you may use the same
file for each machine of the same type.

### Apply Configuration

After you have generated each machine's Machine Config, you need to load them
into the mahines themselves.
For that, you need to know their IP addresses.

If you have access to the console or console logs of the machines, you can read
them to find the IP address(es).
Talos will print them out during the boot process:

```log
[    4.605369] [talos] task loadConfig (1/1): this machine is reachable at:
[    4.607358] [talos] task loadConfig (1/1):   192.168.0.2
[    4.608766] [talos] task loadConfig (1/1): server certificate fingerprint:
[    4.611106] [talos] task loadConfig (1/1):   xA9a1t2dMxB0NJ0qH1pDzilWbA3+DK/DjVbFaJBYheE=
[    4.613822] [talos] task loadConfig (1/1):
[    4.614985] [talos] task loadConfig (1/1): upload configuration using talosctl:
[    4.616978] [talos] task loadConfig (1/1):   talosctl apply-config --insecure --nodes 192.168.0.2 --file <config.yaml>
[    4.620168] [talos] task loadConfig (1/1): or apply configuration using talosctl interactive installer:
[    4.623046] [talos] task loadConfig (1/1):   talosctl apply-config --insecure --nodes 192.168.0.2 --interactive
[    4.626365] [talos] task loadConfig (1/1): optionally with node fingerprint check:
[    4.628692] [talos] task loadConfig (1/1):   talosctl apply-config --insecure --nodes 192.168.0.2 --cert-fingerprint 'xA9a1t2dMxB0NJ0qH1pDzilWbA3+DK/DjVbFaJBYheE=' --file <config.yaml>
```

If you do not have console access, the IP address may also be discoverable from
your DHCP server.

Once you have the IP address, you can then apply the correct configuration.

```sh
  talosctl apply-config --insecure \
    --nodes 192.168.0.2 \
    --file cp0.yaml
```

The insecure flag is necessary at this point because the PKI infrastructure has
not yet been made available to the node.
Note that the connection _will_ be encrypted, it is just unauthenticated.

If you have console access, though, you can extract the server
certificate fingerprint and use it for an additional layer of validation:

```sh
  talosctl apply-config --insecure \
    --nodes 192.168.0.2 \
    --cert-fingerprint xA9a1t2dMxB0NJ0qH1pDzilWbA3+DK/DjVbFaJBYheE= \
    --file cp0.yaml
```

Using the fingerprint allows you to be sure you are sending the configuration to
the right machine, but it is completely optional.

After the configuration is applied to a node, it will reboot.

You may repeat this process for each of the nodes in your cluster.

## Configure your talosctl client

Now that the nodes are running Talos with its full PKI security suite, you need
to use that PKI to talk to the machines.
That means configuring your client, and that is what that `talosconfig` file is for.

### Endpoints

Endpoints are the communication endpoints to which the client directly talks.
These can be load balancers, DNS hostnames, a list of IPs, etc.
In general, it is recommended that these point to the set of control plane
nodes, either directly or through a reverse proxy or load balancer.

Each endpoint will automatically proxy requests destined to another node through
it, so it is not necessary to change the endpoint configuration just because you
wish to talk to a different node within the cluster.

Endpoints _do_, however, need to be members of the same Talos cluster as the
target node, because these proxied connections reply on certificate-based
authentication.

We need to set the `endpoints` in your `talosconfig`.
`talosctl` will automatically load balance and fail over among the endpoints,
so no external load balancer or DNS abstraction is required
(though you are free to use them, if desired).

As an example, if the IP addresses of our controlplane nodes are:

- 192.168.0.2
- 192.168.0.3
- 192.168.0.4

We would set those in the `talosconfig` with:

```sh
  talosctl --talosconfig=./talosconfig \
    config endpoint 192.168.0.2 192.168.0.3 192.168.0.4
```

### Nodes

The node is the target node on which you wish to perform the API call.

Keep in mind, when specifying nodes that their IPs and/or hostnames are as seen by the endpoint servers, not as from the client.
This is because all connections are proxied first through the endpoints.

Some people also like to set a default set of nodes in the `talosconfig`.
This can be done in the same manner, replacing `endpoint` with `node`.
If you do this, however, know that you could easily reboot the wrong machine
by forgetting to declare the right one explicitly.
Worse, if you set several nodes as defaults, you could, with one `talosctl upgrade`
command upgrade your whole cluster all at the same time.
It's a powerful tool, and with that comes great responsibility.

The author of this document generally sets a single controlplane node to be the
default node, which provides the most flexible default operation while limiting
the scope of the disaster should a command be entered erroneously:

```sh
  talosctl --talosconfig=./talosconfig \
    config node 192.168.0.2
```

You may simply provide `-n` or `--nodes` to any `talosctl` command to
supply the node or (comma-delimited) nodes on which you wish to perform the
operation.
Supplying the commandline parameter will override any default nodes
in the configuration file.

To verify default node(s) you're currently configured to use, you can run:

```bash
$ talosctl version
Client:
        ...
Server:
        NODE:        <node>
        ...
```

For a more in-depth discussion of Endpoints and Nodes, please see
[talosctl](../../learn-more/talosctl/).

### Default configuration file

You _can_ reference which configuration file to use directly with the `--talosconfig` parameter:

```sh
  talosctl --talosconfig=./talosconfig \
    --nodes 192.168.0.2 version
```

However, `talosctl` comes with tooling to help you integrate and merge this
configuration into the default `talosctl` configuration file.
This is done with the `merge` option.

```sh
  talosctl config merge ./talosconfig
```

This will merge your new `talosconfig` into the default configuration file
(`$XDG_CONFIG_HOME/talos/config.yaml`), creating it if necessary.
Like Kubernetes, the `talosconfig` configuration files has multiple "contexts"
which correspond to multiple clusters.
The `<cluster-name>` you chose above will be used as the context name.

## Kubernetes Bootstrap

All of your machines are configured, and your `talosctl` client is set up.
Now, you are ready to bootstrap your Kubernetes cluster.
If that sounds daunting, you haven't used Talos before.

Bootstrapping your Kubernetes cluster with Talos is as simple as:

```sh
  talosctl bootstrap --nodes 192.168.0.2
```

**IMPORTANT**: the bootstrap operation should only be called **ONCE** and only on a **SINGLE**
controlplane node!

The IP there can be any of your controlplanes (or the loadbalancer, if you have
one).
It should only be issued once.

At this point, Talos will form an `etcd` cluster, generate all of the core
Kubernetes assets, and start the Kubernetes controlplane components.

After a few moments, you will be able to download your Kubernetes client
configuration and get started:

```sh
  talosctl kubeconfig
```

Running this command will add (merge) you new cluster into you local Kubernetes
configuration in the same way as `talosctl config merge` merged the Talos client
configuration into your local Talos client configuration file.

If you would prefer for the configuration to _not_ be merged into your default
Kubernetes configuration file, simple tell it a filename:

```sh
  talosctl kubeconfig alternative-kubeconfig
```

If all goes well, you should now be able to connect to Kubernetes and see your
nodes:

```sh
  kubectl get nodes
```
