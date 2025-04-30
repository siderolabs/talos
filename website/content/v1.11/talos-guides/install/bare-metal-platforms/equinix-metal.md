---
title: "Equinix Metal"
description: "Creating Talos clusters with Equinix Metal."
aliases:
  - ../../../bare-metal-platforms/equinix-metal
---

You can create a Talos Linux cluster on Equinix Metal in a variety of ways, such as through the EM web UI, or the `metal` command line tool.

Regardless of the method, the process is:

* Create a DNS entry for your Kubernetes endpoint.
* Generate the configurations using `talosctl`.
* Provision your machines on Equinix Metal.
* Push the configurations to your servers (if not done as part of the machine provisioning).
* Configure your Kubernetes endpoint to point to the newly created control plane nodes.
* Bootstrap the cluster.

## Define the Kubernetes Endpoint

There are a variety of ways to create an HA endpoint for the Kubernetes cluster.
Some of the ways are:

* DNS
* Load Balancer
* BGP

Whatever way is chosen, it should result in an IP address/DNS name that routes traffic to all the control plane nodes.
We do not know the control plane node IP addresses at this stage, but we should define the endpoint DNS entry so that we can use it in creating the cluster configuration.
After the nodes are provisioned, we can use their addresses to create the endpoint A records, or bind them to the load balancer, etc.

## Create the Machine Configuration Files

### Generating Configurations

Using the DNS name of the loadbalancer defined above, generate the base configuration files for the Talos machines:

```bash
$ talosctl gen config talos-k8s-em-tutorial https://<load balancer IP or DNS>:<port>
created controlplane.yaml
created worker.yaml
created talosconfig
```

> The `port` used above should be 6443, unless your load balancer maps a different port to port 6443 on the control plane nodes.

### Validate the Configuration Files

```bash
talosctl validate --config controlplane.yaml --mode metal
talosctl validate --config worker.yaml --mode metal
```

> Note: Validation of the install disk could potentially fail as validation
> is performed on your local machine and the specified disk may not exist.

### Passing in the configuration as User Data

You can use the metadata service provide by Equinix Metal to pass in the machines configuration.
It is required to add a shebang to the top of the configuration file.
<!-- textlint-disable one-sentence-per-line -->
The convention we use is `#!talos`.
<!-- textlint-enable one-sentence-per-line -->

## Provision the machines in Equinix Metal

Talos Linux can be PXE-booted on Equinix Metal using [Image Factory]({{< relref "../../../learn-more/image-factory" >}}), using the `equinixMetal` platform: e.g.
`https://pxe.factory.talos.dev/pxe/376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba/{{< release >}}/equinixMetal-amd64` (this URL references the default schematic and `amd64` architecture).

Follow the Image Factory guide to create a custom schematic, e.g. with CPU microcode updates.
The PXE boot URL can be used as the iPXE script URL.

### Using the Equinix Metal UI

Simply select the location and type of machines in the Equinix Metal web interface.
Select 'Custom iPXE' as the Operating System and enter the Image Factory PXE URL as the iPXE script URL, then select the number of servers to create, and name them (in lowercase only.)
Under *optional settings*, you can optionally paste in the contents of `controlplane.yaml` that was generated, above (ensuring you add a first line of `#!talos`).

You can repeat this process to create machines of different types for control plane and worker nodes (although you would pass in `worker.yaml` for the worker nodes, as user data).

If you did not pass in the machine configuration as User Data, you need to provide it to each machine, with the following command:

`talosctl apply-config --insecure --nodes <Node IP> --file ./controlplane.yaml`

### Creating a Cluster via the Equinix Metal CLI

This guide assumes the user has a working API token,and the [Equinix Metal CLI](https://github.com/equinix/metal-cli/) installed.

<!-- textlint-disable one-sentence-per-line -->
> Note: Ensure you have prepended `#!talos` to the `controlplane.yaml` file.
<!-- textlint-enable one-sentence-per-line -->

```bash
metal device create \
  --project-id $PROJECT_ID \
  --metro $METRO \
  --operating-system "custom_ipxe" \
  --ipxe-script-url "https://pxe.factory.talos.dev/pxe/376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba/{{< release >}}/equinixMetal-amd64" \
  --plan $PLAN \
  --hostname $HOSTNAME \
  --userdata-file controlplane.yaml
```

e.g. `metal device create -p <projectID> -f da11 -O custom_ipxe -P c3.small.x86 -H steve.test.11 --userdata-file ./controlplane.yaml --ipxe-script-url "https://pxe.factory.talos.dev/pxe/376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba/{{< release >}}/equinixMetal-amd64"`

Repeat this to create each control plane node desired: there should usually be 3 for a HA cluster.

## Update the Kubernetes endpoint

Now our control plane nodes have been created, and we know their IP addresses, we can associate them with the Kubernetes endpoint.
Configure your load balancer to route traffic to these nodes, or add `A` records to your DNS entry for the endpoint, for each control plane node.
e.g.

```bash
host endpoint.mydomain.com
endpoint.mydomain.com has address 145.40.90.201
endpoint.mydomain.com has address 147.75.109.71
endpoint.mydomain.com has address 145.40.90.177
```

## Bootstrap Etcd

Set the `endpoints` and `nodes` for `talosctl`:

```bash
talosctl --talosconfig talosconfig config endpoint <control plane 1 IP>
talosctl --talosconfig talosconfig config node <control plane 1 IP>
```

Bootstrap `etcd`:

```bash
talosctl --talosconfig talosconfig bootstrap
```

This only needs to be issued to one control plane node.

## Retrieve the `kubeconfig`

At this point we can retrieve the admin `kubeconfig` by running:

```bash
talosctl --talosconfig talosconfig kubeconfig .
```
