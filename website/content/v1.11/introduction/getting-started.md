---
title: Getting Started
weight: 30
description: "A guide to setting up a Talos Linux cluster."
---

This document will walk you through installing a simple Talos Cluster with a single control plane node and one or more worker nodes, explaining some of the concepts.

> If this is your first use of Talos Linux, we recommend the [Quickstart]({{< relref "quickstart" >}}) first, to quickly create a local virtual cluster in containers on your workstation.
>
> For a production cluster, extra steps are needed - see [Production Notes]({{< relref "prodnotes" >}}).

Regardless of where you run Talos, the steps to create a Kubernetes cluster are:

- boot machines off the Talos Linux image
- define the endpoint for the Kubernetes API and generate your machine configurations
- configure Talos Linux by applying machine configurations to the machines
- configure `talosctl`
- bootstrap Kubernetes

## Prerequisites

### `talosctl`

`talosctl` is a CLI tool which interfaces with the Talos API.
Talos Linux has no SSH access: `talosctl` is the tool you use to interact with the operating system on the machines.

You can download `talosctl` on MacOS and Linux via:

```bash
brew install siderolabs/tap/talosctl
```

For manual installation and other platforms please see the [talosctl installation guide]({{< relref "../talos-guides/install/talosctl.md" >}}).

> Note: If you boot systems off the ISO, Talos on the ISO image runs in RAM and acts as an installer.
> The version of `talosctl` that is used to create the machine configurations controls the version of Talos Linux that is installed on the machines - NOT the image that the machines are initially booted off.
> For example, booting a machine off the Talos 1.3.7 ISO, but creating the initial configuration with `talosctl` binary of version 1.4.1, will result in a machine running Talos Linux version 1.4.1.
>
> It is advisable to use the same version of `talosctl` as the version of the boot media used.

### Network access

This guide assumes that the systems being installed have outgoing access to the internet, allowing them to pull installer and container images, query NTP, etc.
If needed, see the documentation on [registry proxies]({{< relref "../talos-guides/configuration/pull-through-cache" >}}), local registries, and [airgapped installation]({{< relref "../advanced/air-gapped" >}}).

## Acquire the Talos Linux image and boot machines

The most general way to install Talos Linux is to use the ISO image.

The latest ISO image can be found on the Github [Releases](https://github.com/siderolabs/talos/releases) page:

- X86: [https://github.com/siderolabs/talos/releases/download/{{< release >}}/metal-amd64.iso](https://github.com/siderolabs/talos/releases/download/{{< release >}}/metal-amd64.iso)
- ARM64: [https://github.com/siderolabs/talos/releases/download/{{< release >}}/metal-arm64.iso](https://github.com/siderolabs/talos/releases/download/{{< release >}}/metal-arm64.iso)

When booted from the ISO, Talos will run in RAM and will not install to disk until provided a configuration.
Thus, it is safe to boot any machine from the ISO.

At this point, you should:

- boot one machine off the ISO to be the control plane node
- boot one or more machines off the same ISO to be the workers

### Alternative Booting

For network booting and self-built media, see [Production Notes]({{< relref "prodnotes#alternative-booting" >}}).
There are installation methods specific to specific platforms, such as pre-built AMIs for AWS - check the specific [Installation Guides]({{< relref "../talos-guides/install/" >}}).)

## Define the Kubernetes Endpoint

In order to configure Kubernetes, Talos needs to know
what the endpoint of the Kubernetes API Server will be.

Because we are only creating a single control plane node in this guide, we can use the control plane node directly as the Kubernetes API endpoint.

Identify the IP address or DNS name of the control plane node that was booted above, and convert it to a fully-qualified HTTPS URL endpoint address for the Kubernetes API Server which (by default) runs on port 6443.
The endpoint should be formatted like:

- `https://192.168.0.2:6443`
- `https://kube.mycluster.mydomain.com:6443`

> NOTE: For a production cluster, you should have three control plane nodes, and have the endpoint allocate traffic to all three - see [Production Notes]({{< relref "prodnotes#control-plane-nodes" >}}).

## Configure Talos Linux

When Talos boots without a configuration, such as when booting off the Talos ISO, it
enters maintenance mode and waits for a configuration to be provided.

> NOTE: Talos initially loads the OS to RAM, and only installs to disk after the configuration is applied.
> If you reboot the machine before applying machine config, make sure your boot media is still present.

Unlike traditional Linux, Talos Linux is _not_ configured by SSHing to the server and issuing commands.
Instead, the entire state of the machine is defined by a `machine config` file which is passed to the server.
This allows machines to be managed in a declarative way, and lends itself to GitOps and modern operations paradigms.

The state of a machine is completely defined by, and can be reproduced from, the machine configuration file.

> A configuration can be passed in on boot via kernel parameters or metadata servers.
> See [Production Notes]({{< relref "prodnotes#configure-talos" >}}).

To generate the machine configurations for a cluster, run this command on the workstation where you installed `talosctl`:

```sh
talosctl gen config <cluster-name> <cluster-endpoint>
```

`cluster-name` is an arbitrary name, used as a label in your local client configuration.
It should be unique in the configuration on your local workstation.

`cluster-endpoint` is the Kubernetes Endpoint you constructed from the control plane node's IP address or DNS name above.
It should be a complete URL, with `https://`
and port.

For example:

```sh
$ talosctl gen config mycluster https://192.168.0.2:6443
generating PKI and tokens
created /Users/taloswork/controlplane.yaml
created /Users/taloswork/worker.yaml
created /Users/taloswork/talosconfig
```

When you run this command, three files are created in your current
directory:

- `controlplane.yaml`
- `worker.yaml`
- `talosconfig`

The `.yaml` files are Machine Configs: they describe everything from what disk Talos should be installed on, to network settings.
The `controlplane.yaml` file also describes how Talos should form a Kubernetes cluster.

The `talosconfig` file is your local client configuration file, used to connect to and authenticate access to the cluster.

### Controlplane and Worker

The two types of Machine Configs correspond to the two roles of Talos nodes, control plane nodes (which run both the Talos and Kubernetes control planes) and worker nodes (which run the workloads).

The main difference between Controlplane Machine Config files and Worker Machine Config files is that the former contains information about how to form the
Kubernetes cluster.

### Modifying the Machine configs

The generated Machine Configs have defaults that work for most cases.
They use DHCP for interface configuration, and install to `/dev/sda`.

Sometimes, you will need to modify the generated files to work with your systems.
A common case is needing to change the installation disk.
If you try to to apply the machine config to a node, and get an error like the below, you need to specify a different installation disk:

```sh
$ talosctl apply-config --insecure -n 192.168.0.2 --file controlplane.yaml
error applying new configuration: rpc error: code = InvalidArgument desc = configuration validation failed: 1 error occurred:
    * specified install disk does not exist: "/dev/sda"
```

You can verify which disks your nodes have by using the `talosctl get disks --insecure` command.

> Insecure mode is needed at this point as the PKI infrastructure has not yet been set up.

For example, the `talosctl get disks` command below shows that the system has a `vda` drive, not an `sda`:

```sh
$ talosctl -n 192.168.0.2 get disks --insecure
DEV        MODEL   SERIAL   TYPE   UUID   WWID  MODALIAS                    NAME   SIZE    BUS_PATH
/dev/vda   -       -        HDD    -      -      virtio:d00000002v00001AF4   -      69 GB   /pci0000:00/0000:00:06.0/virtio2/
```

In this case, you would modify the `controlplane.yaml` and `worker.yaml` files and edit the line:

```yaml
install:
  disk: /dev/sda # The disk used for installations.
```

to reflect `vda` instead of `sda`.

> For information on customizing your machine configurations (such as to specify the version of Kubernetes), using [machine configuration patches]({{< relref "../talos-guides/configuration/patching" >}}), or customizing configurations for individual machines (such as setting static IP addresses), see the [Production Notes]({{< relref "prodnotes#customizing-machine-configuration" >}}).

## Accessing the Talos API

Administrative tasks are performed by calling the Talos API (usually with `talosctl`) on Talos Linux control plane nodes, who may forward the requests to other nodes.
Thus:

- ensure your control plane node is directly reachable on TCP port 50000 from the workstation where you run the `talosctl` client.
- until a node is a member of the cluster, it does not have the PKI infrastructure set up, and so will not accept API requests that are proxied through a control plane node.

Thus you will need direct access to the **worker** nodes on port 50000 from the workstation where you run `talosctl`  in order to apply the initial configuration.
Once the cluster is established, you will no longer need port 50000 access to the workers.
(You can avoid requiring such access by passing in the initial configuration in one of other methods, such as by cloud `userdata` or via `talos.config=` kernel argument on a `metal` platform)

This may require changing firewall rules or cloud provider access-lists.

For production configurations, see [Production Notes]({{< relref "prodnotes#decide-the-kubernetes-endpoint" >}}).

## Understand how talosctl treats endpoints and nodes

In short: `endpoints` are where `talosctl` _sends_ commands to, but the command _operates_ on the specified `nodes`.
The endpoint will forward the command to the nodes, if needed.

### Endpoints

Endpoints are the IP addresses of control plane nodes, to which the `talosctl` client directly talks.

Endpoints automatically proxy requests destined to another node in the cluster.
This means that you only need access to the control plane nodes in order to manage the rest of the cluster.

You can pass in `--endpoints <Control Plane IP Address>` or `-e <Control Plane IP Address>` to the current `talosctl` command.

In this tutorial setup, the endpoint will always be the single control plane node.

### Nodes

Nodes are the target(s) you wish to perform the operation on.

> When specifying nodes, the IPs and/or hostnames are _as seen by the endpoint servers_, not as from the client.
> This is because all connections are proxied through the endpoints.

You may provide `-n` or `--nodes` to any `talosctl` command to supply the node or (comma-separated) nodes on which you wish to perform the operation.

For example, to see the containers running on node 192.168.0.200, by routing the `containers` command through the control plane endpoint 192.168.0.2:

```bash
talosctl -e 192.168.0.2 -n 192.168.0.200 containers
```

To see the etcd logs on _both_ nodes 192.168.0.10 and 192.168.0.11:

```bash
talosctl -e 192.168.0.2 -n 192.168.0.10,192.168.0.11 logs etcd
```

For a more in-depth discussion of Endpoints and Nodes, please see [talosctl]({{< relref "../learn-more/talosctl" >}}).

### Apply Configuration

To apply the Machine Configs, you need to know the machines' IP addresses.

Talos prints the IP addresses of the machines on the console during the boot process:

```log
[4.605369] [talos] task loadConfig (1/1): this machine is reachable at:
[4.607358] [talos] task loadConfig (1/1):   192.168.0.2
```

If you do not have console access, the IP address may also be discoverable from your DHCP server.

Once you have the IP address, you can then apply the correct configuration.
Apply the `controlplane.yaml` file to the control plane node, and the `worker.yaml` file to all the worker node(s).

```sh
  talosctl apply-config --insecure \
    --nodes 192.168.0.2 \
    --file controlplane.yaml
```

The `--insecure` flag is necessary because the PKI infrastructure has not yet been made available to the node.
Note: the connection _will_ be encrypted, but not authenticated.

> When using the `--insecure` flag, you cannot specify an endpoint, and must directly access the node on port 50000.

### Default talosconfig configuration file

You reference which configuration file to use by the `--talosconfig` parameter:

```sh
talosctl --talosconfig=./talosconfig \
    --nodes 192.168.0.2 -e 192.168.0.2 version
```

Note that `talosctl` comes with tooling to help you integrate and merge this configuration into the default `talosctl` configuration file.
See [Production Notes]({{< relref "prodnotes#default-configuration-file" >}}) for more information.

While getting started, a common mistake is referencing a configuration context for a different cluster, resulting in authentication or connection failures.
Thus it is recommended to explicitly pass in the configuration file while becoming familiar with Talos Linux.

## Kubernetes Bootstrap

Bootstrapping your Kubernetes cluster with Talos is as simple as calling `talosctl bootstrap` on your control plane node:

```sh
talosctl bootstrap --nodes 192.168.0.2 --endpoints 192.168.0.2 \
  --talosconfig=./talosconfig
```

> The bootstrap operation should only be called **ONCE** on a **SINGLE** control plane node.
> (If you have multiple control plane nodes, it doesn't matter which one you issue the bootstrap command against.)

At this point, Talos will form an `etcd` cluster, and start the Kubernetes control plane components.

After a few moments, you will be able to download your Kubernetes client configuration and get started:

```sh
talosctl kubeconfig --nodes 192.168.0.2 --endpoints 192.168.0.2 \
  --talosconfig=./talosconfig
```

Running this command will add (merge) you new cluster into your local Kubernetes configuration.

If you would prefer the configuration to _not_ be merged into your default Kubernetes configuration file, pass in a filename:

```sh
talosctl kubeconfig alternative-kubeconfig --nodes 192.168.0.2 --endpoints 192.168.0.2 \
  --talosconfig=./talosconfig
```

You should now be able to connect to Kubernetes and see your nodes:

```sh
kubectl get nodes
```

And use talosctl to explore your cluster:

```sh
talosctl --nodes 192.168.0.2 --endpoints 192.168.0.2 health \
   --talosconfig=./talosconfig
talosctl --nodes 192.168.0.2 --endpoints 192.168.0.2 dashboard \
   --talosconfig=./talosconfig
```

For a list of all the commands and operations that `talosctl` provides, see the [CLI reference]({{< relref "../reference/cli/#talosctl" >}}).
