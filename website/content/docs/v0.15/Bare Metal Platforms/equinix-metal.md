---
title: "Equinix Metal"
description: "Creating Talos cluster using Equinix Metal."
---

## Prerequisites

This guide assumes the user has a working API token, the Equinix Metal CLI installed, and some familiarity with the CLI.

## Network Booting

To install Talos to a server a working TFTP and iPXE server are needed.
How this is done varies and is left as an exercise for the user.
In general this requires a Talos kernel vmlinuz and initramfs.
These assets can be downloaded from a given [release](https://github.com/talos-systems/talos/releases).

## Special Considerations

### PXE Boot Kernel Parameters

The following is a list of kernel parameters required by Talos:

- `talos.platform`: set this to `packet`
- `init_on_alloc=1`: required by KSPP
- `slab_nomerge`: required by KSPP
- `pti=on`: required by KSPP

### User Data

<!-- textlint-disable one-sentence-per-line -->

To configure a Talos you can use the metadata service provide by Equinix Metal.
It is required to add a shebang to the top of the configuration file.
The shebang is arbitrary in the case of Talos, and the convention we use is `#!talos`.

<!-- textlint-enable one-sentence-per-line -->

## Creating a Cluster via the Equinix Metal CLI

### Control Plane Endpoint

The strategy used for an HA cluster varies and is left as an exercise for the user.
Some of the known ways are:

- DNS
- Load Balancer
- BPG

### Create the Machine Configuration Files

#### Generating Base Configurations

Using the DNS name of the loadbalancer created earlier, generate the base configuration files for the Talos machines:

```bash
$ talosctl gen config talos-k8s-aws-tutorial https://<load balancer IP or DNS>:<port>
created controlplane.yaml
created worker.yaml
created talosconfig
```

Now add the required shebang (e.g. `#!talos`) at the top of `controlplane.yaml`, and `worker.yaml`
At this point, you can modify the generated configs to your liking.
Optionally, you can specify `--config-patch` with RFC6902 jsonpatch which will be applied during the config generation.

#### Validate the Configuration Files

```bash
talosctl validate --config controlplane.yaml --mode metal
talosctl validate --config worker.yaml --mode metal
```

> Note: Validation of the install disk could potentially fail as the validation
> is performed on you local machine and the specified disk may not exist.

#### Create the Control Plane Nodes

```bash
packet device create \
  --project-id $PROJECT_ID \
  --facility $FACILITY \
  --ipxe-script-url $PXE_SERVER \
  --operating-system "custom_ipxe" \
  --plan $PLAN\
  --hostname $HOSTNAME\
  --userdata-file controlplane.yaml
```

> Note: The above should be invoked at least twice in order for `etcd` to form quorum.

#### Create the Worker Nodes

```bash
packet device create \
  --project-id $PROJECT_ID \
  --facility $FACILITY \
  --ipxe-script-url $PXE_SERVER \
  --operating-system "custom_ipxe" \
  --plan $PLAN\
  --hostname $HOSTNAME\
  --userdata-file worker.yaml
```

### Bootstrap Etcd

Set the `endpoints` and `nodes`:

```bash
talosctl --talosconfig talosconfig config endpoint <control plane 1 IP>
talosctl --talosconfig talosconfig config node <control plane 1 IP>
```

Bootstrap `etcd`:

```bash
talosctl --talosconfig talosconfig bootstrap
```

### Retrieve the `kubeconfig`

At this point we can retrieve the admin `kubeconfig` by running:

```bash
talosctl --talosconfig talosconfig kubeconfig .
```
