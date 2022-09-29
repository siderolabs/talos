---
title: Getting Started
weight: 30
description: "A guide to setting up a Talos Linux cluster on multiple machines."
---

This document will walk you through installing a full Talos Cluster.
If this is your first use of Talos Linux, we recommend the the [Quickstart]({{< relref "quickstart" >}}) first, to quickly create a local virtual cluster on your workstation.

Regardless of where you run Talos, in general you need to:

- acquire the installation image
- decide on the endpoint for Kubernetes
  - optionally create a load balancer
- configure Talos
- configure `talosctl`
- bootstrap Kubernetes

## Prerequisites

### `talosctl`

`talosctl` is a CLI tool which interfaces with the Talos API in
an easy manner.

Install `talosctl` before continuing:

#### `amd64`

```bash
curl -Lo /usr/local/bin/talosctl https://github.com/siderolabs/talos/releases/download/{{< release >}}/talosctl-$(uname -s | tr "[:upper:]" "[:lower:]")-amd64
chmod +x /usr/local/bin/talosctl
```

#### `arm64`

For `linux` and `darwin` operating systems `talosctl` is also available for the `arm64` architecture.

```bash
curl -Lo /usr/local/bin/talosctl https://github.com/siderolabs/talos/releases/download/{{< release >}}/talosctl-$(uname -s | tr "[:upper:]" "[:lower:]")-arm64
chmod +x /usr/local/bin/talosctl
```

## Acquire the installation image

The most general way to install Talos is to use the ISO image (but note there are easer methods for some platforms, such as pre-built AMIs for AWS - check the specific [Installation Guides]({{< relref "../talos-guides/install/" >}}).)

The latest ISO image can be found on the Github [Releases](https://github.com/siderolabs/talos/releases) page:

- X86: [https://github.com/siderolabs/talos/releases/download/{{< release >}}/talos-amd64.iso](https://github.com/siderolabs/talos/releases/download/{{< release >}}/talos-amd64.iso)
- ARM64: [https://github.com/siderolabs/talos/releases/download/{{< release >}}/talos-arm64.iso](https://github.com/siderolabs/talos/releases/download/{{< release >}}/talos-arm64.iso)

When booted from the ISO, Talos will run in RAM, and will not install itself
until it is provided a configuration.
Thus, it is safe to boot the ISO onto any machine.

### Alternative Booting

For network booting and self-built media, you can use the published kernel and initramfs images:

- X86: [vmlinuz-amd64](https://github.com/siderolabs/talos/releases/download/{{< release >}}/vmlinuz-amd64) [initramfs-amd64.xz](https://github.com/siderolabs/talos/releases/download/{{< release >}}/initramfs-amd64.xz)
- ARM64: [vmlinuz-arm64](https://github.com/siderolabs/talos/releases/download/{{< release >}}/vmlinuz-arm64) [initramfs-arm64.xz](https://github.com/siderolabs/talos/releases/download/{{< release >}}/initramfs-arm64.xz)

Note that to use alternate booting, there are a number of required kernel parameters.
Please see the [kernel]({{< relref "../reference/kernel" >}}) docs for more information.

## Decide the Kubernetes Endpoint

In order to configure Kubernetes and bootstrap the cluster, Talos needs to know
what the endpoint (DNS name or IP address) of the Kubernetes API Server will be.

The endpoint should be the fully-qualified HTTP(S) URL for the Kubernetes API
Server, which (by default) runs on port 6443 using HTTPS.

Thus, the format of the endpoint may be something like:

- `https://192.168.0.10:6443`
- `https://kube.mycluster.mydomain.com:6443`
- `https://[2001:db8:1234::80]:6443`

The Kubernetes API Server endpoint, in order to be highly availabile, should be configured in a way that functions off all available control plane nodes.
There are three common ways to do this:

### Dedicated Load-balancer

If you are using a cloud provider or have your own load-balancer available (such
as HAProxy, nginx reverse proxy, or an F5 load-balancer), using
a dedicated load balancer is a natural choice.
Create an appropriate frontend matching the endpoint, and point the backends at each of the addresses of the Talos controlplane nodes.
Note that given we have not yet created the control plane nodes, the IP addresses of the backends may not be known yet.
We can bind the backends to the frontend at a later point.
At this stage, we just need the IP and Port of the frontend to be created, for use later on.

### Layer 2 Shared IP

Talos has integrated support for serving Kubernetes from a shared/virtual IP address.
This method relies on Layer 2 connectivity between controlplane Talos nodes.

In this case, we choose an IP address on the same subnet as the Talos
controlplane nodes which is not otherwise assigned to any machine.
For instance, if your controlplane node IPs are:

- 192.168.0.10
- 192.168.0.11
- 192.168.0.12

you could choose the ip `192.168.0.15` as your shared IP address.
(Make sure that `192.168.0.15` is not used by any other machine and that your DHCP server
will not serve it to any other machine.)

Once chosen, form the full HTTPS URL from this IP:

```url
https://192.168.0.15:6443
```

If you create a DNS record for this IP, note you will need to use the IP address itself, not the DNS name, to configure the shared IP (`machine.network.interfaces[].vip.ip`) in the Talos configuration.

For more information about using a shared IP, see the related
[Guide]({{< relref "../talos-guides/network/vip" >}})

### DNS records

If neither of the other methods work for you, you can use DNS records to
provide a measure of redundancy.
In this case, you would add multiple A or AAAA records (one for each controlpane node) to a DNS name.

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

Much of the power of Talos Linux comes from the API calls that can be made against control plane nodes.

We recommend accessing the control plane nodes directly from the `talosctl` client, if possible (i.e. set your `endpoints` to the IP addresses of the control plane nodes).
This requires your controlplane nodes to be reachable from the client IP.

If your control plane nodes are not directly reachable from your workstation where you run `talosctl`, then configure a load balancer for TCP port 50000 to be forwarded to the controlplane nodes.
Do not use Talos Linux's built in VIP support for accessing the Talos API, as it will not function in the event of an etcd failure, and you will not be able to access the Talos API to fix things.

If you create a load balancer to forward the Talos API calls, make a note of the IP or
hostname so that you can configure your `talosctl` tool's `endpoints` below.

## Configure Talos

Talos Linux needs to be given configuration information to know how to form a Kubernetes cluster.

When Talos boots without a configuration, such as when using the Talos ISO, it
enters a limited maintenance mode and waits for a configuration to be provided.

In other installation methods, a configuration can be passed in on boot.
For example, Talos can be booted with the `talos.config` kernel
commandline argument set to an HTTP(s) URL from which it should receive its
configuration.
Where a PXE server is available, this is much more efficient than
manually configuring each node.
If you do use this method, note that Talos requires a number of other
kernel commandline parameters.
See [required kernel parameters]({{< relref "../reference/kernel" >}}).
If creating [EC2 kubernetes clusters]({{< relref "../talos-guides/install/cloud-platforms/aws/" >}}), the configuration file can be passed in as `--user-data` to the `aws ec2 run-instances` command.

In any case, we need to generate the configuration which is to be provided.
`talosctl` can do exactly this:

```sh
  talosctl gen config "cluster-name" "cluster-endpoint"
```

Here, `cluster-name` is an arbitrary name for the cluster which will be used
in your local client configuration as a label.
It does not affect anything in the cluster itself, but it should be unique in the configuration on your local workstation.

The `cluster-endpoint` is the Kubernetes Endpoint you
selected from above.
This is the Kubernetes API URL, and it should be a complete URL, with `https://`
and port.
(The default port is `6443`, but you may have configured your load balancer to forward a different port.)

When you run this command, you will receive a number of files in your current
directory:

- `controlplane.yaml`
- `worker.yaml`
- `talosconfig`

The `.yaml` files are what we call Machine Configs.
They provide the Talos servers their complete configuration,
describing everything from what disk Talos should be installed to, to what
sysctls to set, to network settings.
The `controlplane.yaml` file describes how Talos should form a Kubernetes cluster.

The `talosconfig` file (which is also YAML) is your local client configuration
file.

### Controlplane and Worker

The two types of Machine Configs correspond to the two roles of Talos nodes, controlplane (which run both the Talos and Kubernetes control planes) and worker nodes (which run the workloads).

The main difference between Controlplane Machine Config files and Worker Machine
Config files is that the former contains information about how to form the
Kubernetes cluster.

### Machine Configs as Templates

The generated files can be thought of as templates.
Individual machines may need specific settings: for instance, each may have a
different [static IP address]({{< relref "../advanced/advanced-networking/#static-addressing" >}}).
By default, interfaces are set to DHCP.
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

Then modify each file as needed.

In cases where there is no special configuration needed, you may use the same
file for all machines of the same type.

### Apply Configuration

If you have booted into maintenance mode, then after you have generated the Machine Configs, you need to load them
into the machines themselves.
For that, you need to know their IP addresses.

If you have access to the console logs of the machines, you can read
them to find the IP address(es).
Talos will print them out during the boot process:

```log
[4.605369] [talos] task loadConfig (1/1): this machine is reachable at:
[4.607358] [talos] task loadConfig (1/1):   192.168.0.2
[4.608766] [talos] task loadConfig (1/1): server certificate fingerprint:
[4.611106] [talos] task loadConfig (1/1):   xA9a1t2dMxB0NJ0qH1pDzilWbA3+DK/DjVbFaJBYheE=
[4.613822] [talos] task loadConfig (1/1):
[4.614985] [talos] task loadConfig (1/1): upload configuration using talosctl:
[4.616978] [talos] task loadConfig (1/1):   talosctl apply-config --insecure --nodes 192.168.0.2 --file <config.yaml>
[4.620168] [talos] task loadConfig (1/1): or apply configuration using talosctl interactive installer:
[4.623046] [talos] task loadConfig (1/1):   talosctl apply-config --insecure --nodes 192.168.0.2 --mode=interactive
[4.626365] [talos] task loadConfig (1/1): optionally with node fingerprint check:
[4.628692] [talos] task loadConfig (1/1):   talosctl apply-config --insecure --nodes 192.168.0.2 --cert-fingerprint 'xA9a1t2dMxB0NJ0qH1pDzilWbA3+DK/DjVbFaJBYheE=' --file <config.yaml>
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
Note: the connection _will_ be encrypted, it is just unauthenticated.

If you have console access you can extract the server
certificate fingerprint and use it for an additional layer of validation:

```sh
  talosctl apply-config --insecure \
    --nodes 192.168.0.2 \
    --cert-fingerprint xA9a1t2dMxB0NJ0qH1pDzilWbA3+DK/DjVbFaJBYheE= \
    --file cp0.yaml
```

Using the fingerprint allows you to be sure you are sending the configuration to
the correct machine, but it is completely optional.

After the configuration is applied to a node, it will reboot.

Repeat this process for each of the nodes in your cluster.

## Configure your talosctl client

Now that the nodes are running Talos with its full PKI security suite, you need
to use that PKI to talk to the machines.
That is what that `talosconfig` file is for.

It is important to understand the conecpt of `endpoints` and `nodes`.

### Endpoints

Endpoints are the IP addresses to which the `talosctl` client directly talks.
It is recommended that these point to the set of control plane
nodes, either directly or through a load balancer.
You tell your client (`talosctl`) to communicate with the controlplane nodes by defining the `endpoints`.

Each endpoint will automatically proxy requests destined to another node, so it is not necessary to change the endpoint configuration just to talk to a different node within the cluster.
This means that you only need access to the controlplane nodes in order to access
the rest of the network.
This is useful for security (worker nodes do not need to have
public IPs, and can still be queried by `talosctl`), and it also makes working
with highly-variable clusters easier, since you only need to know the
controlplane nodes in advance.

Endpoints _do_ need to be members of the same Talos cluster as the
target node, because these proxied connections reply on certificate-based
authentication.

`talosctl` will automatically load balance requests and fail over
between all of your endpoints.

You can pass in `--endpoints <IP Address1>,<IP Address2>` as a comma separated list of IP/DNS addresses to affect the endpoints used by the current `talosctl` command.
You can also set the `endpoints` in your `talosconfig`, by calling `talosctl config endpoint <IP Address1> <IP Address2>`.
Note: these are space separated, not comma separated.

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

The node is the target you wish to perform the API call on.

> When specifying nodes, their IPs and/or hostnames are *as seen by the endpoint servers*, not as from the client.
> This is because all connections are proxied through the endpoints.

Some people also like to set a default set of nodes in the `talosconfig`.
This can be done in the same manner, replacing `endpoint` with `node`.

```sh
  talosctl --talosconfig=./talosconfig \
    config node 192.168.0.2
```

If you do this, however, you could easily reboot the wrong machine, or multiple machines,
by forgetting to declare the right one explicitly.

Our recommendation is to leave `nodes` empty in the talosconfig file, and explicitly pass in the node or nodes to be operated on with each `talosctl` command.
You may provide `-n` or `--nodes` to any `talosctl` command to
supply the node or (comma-delimited) nodes on which you wish to perform the
operation.
Supplying the commandline parameter will override any default nodes
in the configuration file.

For example, to see the containers running on node 192.168.0.200:

```bash
talosctl -n 192.168.0.200 containers
```

To see the etcd logs on *both* nodes 192.168.0.10 and 192.168.0.11:

```bash
talosctl -n 192.168.0.10,192.168.0.11 logs etcd
```

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
[talosctl]({{< relref "../learn-more/talosctl" >}}).

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

Bootstrapping your Kubernetes cluster with Talos is as simple as:

```sh
  talosctl bootstrap --nodes 192.168.0.2
```

The bootstrap operation should only be called **ONCE** and only on a **SINGLE**
controlplane node!

The IP can be any of your controlplanes (or the loadbalancer, if used for the Talos API endpoint).

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
Kubernetes configuration file, tell it a filename:

```sh
  talosctl kubeconfig alternative-kubeconfig
```

You should now be able to connect to Kubernetes and see your
nodes:

```sh
  kubectl get nodes
```

And use talosctl to explore your cluster:

```sh
  talosctl -n <NODEIP> dashboard
```
