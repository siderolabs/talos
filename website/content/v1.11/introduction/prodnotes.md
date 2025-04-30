---
title: Production Clusters
weight: 30
description: "Recommendations for setting up a Talos Linux cluster in production."
---

This document explains recommendations for running Talos Linux in production.

## Acquire the installation image

### Alternative Booting

For network booting and self-built media, you can use the published kernel and initramfs images:

- X86: [vmlinuz-amd64](https://github.com/siderolabs/talos/releases/download/{{< release >}}/vmlinuz-amd64) [initramfs-amd64.xz](https://github.com/siderolabs/talos/releases/download/{{< release >}}/initramfs-amd64.xz)
- ARM64: [vmlinuz-arm64](https://github.com/siderolabs/talos/releases/download/{{< release >}}/vmlinuz-arm64) [initramfs-arm64.xz](https://github.com/siderolabs/talos/releases/download/{{< release >}}/initramfs-arm64.xz)

Note that to use alternate booting, there are a number of required kernel parameters.
Please see the [kernel]({{< relref "../reference/kernel" >}}) docs for more information.

## Control plane nodes

For a production, highly available Kubernetes cluster, it is recommended to use three control plane nodes.
Using five nodes can provide greater fault tolerance, but imposes more replication overhead and can result in worse performance.

Boot all three control plane nodes at this point.
They will boot Talos Linux, and come up in maintenance mode, awaiting a configuration.

## Decide the Kubernetes Endpoint

The Kubernetes API Server endpoint, in order to be highly available, should be configured in a way that uses all available control plane nodes.
There are three common ways to do this: using a load-balancer, using Talos Linux's built in VIP functionality, or using multiple DNS records.

### Dedicated Load-balancer

If you are using a cloud provider or have your own load-balancer
(such as HAProxy, Nginx reverse proxy, or an F5 load-balancer), a dedicated load balancer is a natural choice.
Create an appropriate frontend for the endpoint, listening on TCP port 6443, and point the backends at the addresses of each of the Talos control plane nodes.
Your Kubernetes endpoint will be the IP address or DNS name of the load balancer front end, with the port appended (e.g. https://myK8s.mydomain.io:6443).

> Note: an HTTP load balancer can't be used, as Kubernetes API server does TLS termination and mutual TLS authentication.

### Layer 2 VIP Shared IP

Talos has integrated support for serving Kubernetes from a shared/virtual IP address.
This requires Layer 2 connectivity between control plane nodes.

Choose an unused IP address on the same subnet as the control plane nodes for the VIP.
For instance, if your control plane node IPs are:

- 192.168.0.10
- 192.168.0.11
- 192.168.0.12

you could choose the IP `192.168.0.15` as your VIP IP address.
(Make sure that `192.168.0.15` is not used by any other machine and is excluded from DHCP ranges.)

Once chosen, form the full HTTPS URL from this IP:

```url
https://192.168.0.15:6443
```

If you create a DNS record for this IP, note you will need to use the IP address itself, not the DNS name, to configure the shared IP (`machine.network.interfaces[].vip.ip`) in the Talos configuration.

After the machine configurations are generated, you will want to edit the `controlplane.yaml` file to activate the VIP:

```yaml
machine:
  network:
    interfaces:
      - interface: enp2s0
        dhcp: true
        vip:
          ip: 192.168.0.15
```

For more information about using a shared IP, see the related
[Guide]({{< relref "../talos-guides/network/vip" >}})

### DNS records

Add multiple A or AAAA records (one for each control plane node) to a DNS name.

For instance, you could add:

```dns
kube.cluster1.mydomain.com  IN  A  192.168.0.10
kube.cluster1.mydomain.com  IN  A  192.168.0.11
kube.cluster1.mydomain.com  IN  A  192.168.0.12
```

where the IP addresses are those of the control plane nodes.

Then, your endpoint would be:

```url
https://kube.cluster1.mydomain.com:6443
```

## Multihoming

If your machines are multihomed, i.e., they have more than one IPv4 and/or IPv6 addresses other than loopback, then additional configuration is required.
A point to note is that the machines may become multihomed via privileged workloads.

### Multihoming and etcd

The `etcd` cluster needs to establish a mesh of connections among the members.
It is done using the so-called advertised address - each node learns the others' addresses as they are advertised.
It is crucial that these IP addresses are stable, i.e., that each node always advertises the same IP address.
Moreover, it is beneficial to control them to establish the correct routes between the members and, e.g., avoid congested paths.
In Talos, these addresses are controlled using the `cluster.etcd.advertisedSubnets` configuration key.

### Multihoming and kubelets

Stable IP addressing for kubelets (i.e., nodeIP) is not strictly necessary but highly recommended as it ensures that, e.g., kube-proxy and CNI routing take the desired routes.
Analogously to etcd, for kubelets this is controlled via `machine.kubelet.nodeIP.validSubnets`.

### Example

Let's assume that we have a cluster with two networks:

- public network
- private network `192.168.0.0/16`

We want to use the private network for etcd and kubelet communication:

```yaml
machine:
  kubelet:
    nodeIP:
      validSubnets:
        - 192.168.0.0/16
#...
cluster:
  etcd:
    advertisedSubnets: # listenSubnets defaults to advertisedSubnets if not set explicitly
      - 192.168.0.0/16
```

This way we ensure that the `etcd` cluster will use the private network for communication and the kubelets will use the private network for communication with the control plane.

## Load balancing the Talos API

The `talosctl` tool provides built-in client-side load-balancing across control plane nodes, so usually you do not need to configure a load balancer for the Talos API.

However, if the control plane nodes are *not* directly reachable from the workstation where you run `talosctl`, then configure a load balancer to forward TCP port 50000 to the control plane nodes.

> Note: Because the Talos Linux API uses gRPC and mutual TLS, it cannot be proxied by a HTTP/S proxy, but only by a TCP load balancer.

If you create a load balancer to forward the Talos API calls, the load balancer IP or hostname will be used as the `endpoint` for `talosctl`.

Add the load balancer IP or hostname to the `.machine.certSANs` field of the machine configuration file.

> Do *not* use Talos Linux's built in VIP function for accessing the Talos API.
> In the event of an error in `etcd`, the VIP will not function, and you will not be able to access the Talos API to recover.

## Configure Talos

In many installation methods, a configuration can be passed in on boot.

For example, Talos can be booted with the `talos.config` kernel
argument set to an HTTP(s) URL from which it should receive its
configuration.
Where a PXE server is available, this is much more efficient than
manually configuring each node.
If you do use this method, note that Talos requires a number of other
kernel commandline parameters.
See [required kernel parameters]({{< relref "../reference/kernel" >}}).

Similarly, if creating [EC2 kubernetes clusters]({{< relref "../talos-guides/install/cloud-platforms/aws/" >}}), the configuration file can be passed in as `--user-data` to the `aws ec2 run-instances` command.
See generally the [Installation Guide]({{< relref "../talos-guides/install" >}}) for the platform being deployed.

### Separating out secrets

When generating the configuration files for a Talos Linux cluster, it is recommended to start with generating a secrets bundle which should be saved in a secure location.
This bundle can be used to generate machine or client configurations at any time:

```sh
talosctl gen secrets -o secrets.yaml
```

> The `secrets.yaml` can also be extracted from the existing controlplane machine configuration with
> `talosctl gen secrets --from-controlplane-config controlplane.yaml -o secrets.yaml` command.

Now, we can generate the machine configuration for each node:

```sh
talosctl gen config --with-secrets secrets.yaml <cluster-name> <cluster-endpoint>
```

Here, `cluster-name` is an arbitrary name for the cluster, used
in your local client configuration as a label.
It should be unique in the configuration on your local workstation.

The `cluster-endpoint` is the Kubernetes Endpoint you
selected from above.
This is the Kubernetes API URL, and it should be a complete URL, with `https://`
and port.
(The default port is `6443`, but you may have configured your load balancer to forward a different port.)
For example:

```sh
$ talosctl gen config --with-secrets secrets.yaml my-cluster https://192.168.64.15:6443
generating PKI and tokens
created controlplane.yaml
created worker.yaml
created talosconfig
```

### Customizing Machine Configuration

The generated machine configuration provides sane defaults for most cases, but can be modified to fit specific needs.

Some machine configuration options are available as flags for the `talosctl gen config` command,
for example setting a specific Kubernetes version:

```sh
talosctl gen config --with-secrets secrets.yaml --kubernetes-version 1.25.4 my-cluster https://192.168.64.15:6443
```

Other modifications are done with [machine configuration patches]({{< relref "../talos-guides/configuration/patching" >}}).
Machine configuration patches can be applied with `talosctl gen config` command:

```sh
talosctl gen config --with-secrets secrets.yaml --config-patch-control-plane @cni.patch my-cluster https://192.168.64.15:6443
```

> Note: `@cni.patch` means that the patch is read from a file named `cni.patch`.

#### Machine Configs as Templates

Individual machines may need different settings: for instance, each may have a
different [static IP address]({{< relref "../advanced/advanced-networking/#static-addressing" >}}).

When different files are needed for machines of the same type, there are two supported flows:

1. Use the `talosctl gen config` command to generate a template, and then patch
   the template for each machine with `talosctl machineconfig patch`.
2. Generate each machine configuration file separately with `talosctl gen config` while applying patches.

For example, given a machine configuration patch which sets the static machine hostname:

```yaml
# worker1.patch
machine:
  network:
    hostname: worker1
```

Either of the following commands will generate a worker machine configuration file with the hostname set to `worker1`:

```bash
$ talosctl gen config --with-secrets secrets.yaml my-cluster https://192.168.64.15:6443
created /Users/taloswork/controlplane.yaml
created /Users/taloswork/worker.yaml
created /Users/taloswork/talosconfig
$ talosctl machineconfig patch worker.yaml --patch @worker1.patch --output worker1.yaml
```

```sh
talosctl gen config --with-secrets secrets.yaml --config-patch-worker @worker1.patch --output-types worker -o worker1.yaml my-cluster https://192.168.64.15:6443
```

### Apply Configuration while validating the node identity

If you have console access you can extract the server certificate fingerprint and use it for an additional layer of validation:

```sh
  talosctl apply-config --insecure \
    --nodes 192.168.0.2 \
    --cert-fingerprint xA9a1t2dMxB0NJ0qH1pDzilWbA3+DK/DjVbFaJBYheE= \
    --file cp0.yaml
```

Using the fingerprint allows you to be sure you are sending the configuration to the correct machine, but is completely optional.
After the configuration is applied to a node, it will reboot.
Repeat this process for each of the nodes in your cluster.

## Further details about talosctl, endpoints and nodes

### Endpoints

When passed multiple endpoints, `talosctl` will automatically load balance requests to, and fail over between, all endpoints.

You can pass in `--endpoints <IP Address1>,<IP Address2>` as a comma separated list of IP/DNS addresses to the current `talosctl` command.
You can also set the `endpoints` in your `talosconfig`, by calling `talosctl config endpoint <IP Address1> <IP Address2>`.
Note: these are space separated, not comma separated.

As an example, if the IP addresses of our control plane nodes are:

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

It is possible to set a default set of nodes in the `talosconfig` file, but our recommendation is to explicitly pass in the node or nodes to be operated on with each `talosctl` command.
For a more in-depth discussion of Endpoints and Nodes, please see [talosctl]({{< relref "../learn-more/talosctl" >}}).

### Default configuration file

You can reference which configuration file to use directly with the `--talosconfig` parameter:

```sh
  talosctl --talosconfig=./talosconfig \
    --nodes 192.168.0.2 version
```

However, `talosctl` comes with tooling to help you integrate and merge this configuration into the default `talosctl` configuration file.
This is done with the `merge` option.

```sh
  talosctl config merge ./talosconfig
```

This will merge your new `talosconfig` into the default configuration file (`$XDG_CONFIG_HOME/talos/config.yaml`), creating it if necessary.
Like Kubernetes, the `talosconfig` configuration files has multiple "contexts" which correspond to multiple clusters.
The `<cluster-name>` you chose above will be used as the context name.

## Kubernetes Bootstrap

Bootstrapping your Kubernetes cluster by simply calling the `bootstrap` command against any of your control plane nodes (or the loadbalancer, if used for the Talos API endpoint).:

```sh
  talosctl bootstrap --nodes 192.168.0.2
```

>The bootstrap operation should only be called **ONCE** and only on a **SINGLE** control plane node!

At this point, Talos will form an `etcd` cluster, generate all of the core Kubernetes assets, and start the Kubernetes control plane components.

After a few moments, you will be able to download your Kubernetes client configuration and get started:

```sh
  talosctl kubeconfig
```

Running this command will add (merge) you new cluster into your local Kubernetes configuration.

If you would prefer the configuration to *not* be merged into your default Kubernetes configuration file, pass in a filename:

```sh
  talosctl kubeconfig alternative-kubeconfig
```

You should now be able to connect to Kubernetes and see your nodes:

```sh
  kubectl get nodes
```

And use talosctl to explore your cluster:

```sh
  talosctl -n <NODEIP> dashboard
```

For a list of all the commands and operations that `talosctl` provides, see the [CLI reference]({{< relref "../reference/cli/#talosctl" >}}).
