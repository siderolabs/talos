---
title: "Matchbox"
description: "In this guide we will create an HA Kubernetes cluster with 3 worker nodes using an existing load balancer and matchbox deployment."
---

## Creating a Cluster

In this guide we will create an HA Kubernetes cluster with 3 worker nodes.
We assume an existing load balancer, matchbox deployment, and some familiarity with iPXE.

We leave it up to the user to decide if they would like to use static networking, or DHCP.
The setup and configuration of DHCP will not be covered.

### Create the Machine Configuration Files

#### Generating Base Configurations

Using the DNS name of the load balancer, generate the base configuration files for the Talos machines:

```bash
$ talosctl gen config talos-k8s-metal-tutorial https://<load balancer IP or DNS>:<port>
created controlplane.yaml
created worker.yaml
created talosconfig
```

At this point, you can modify the generated configs to your liking.
Optionally, you can specify `--config-patch` with RFC6902 jsonpatch which will be applied during the config generation.

#### Validate the Configuration Files

```bash
$ talosctl validate --config controlplane.yaml --mode metal
controlplane.yaml is valid for metal mode
$ talosctl validate --config worker.yaml --mode metal
worker.yaml is valid for metal mode
```

#### Publishing the Machine Configuration Files

In bare-metal setups it is up to the user to provide the configuration files over HTTP(S).
A special kernel parameter (`talos.config`) must be used to inform Talos about _where_ it should retreive its' configuration file.
To keep things simple we will place `controlplane.yaml`, and `worker.yaml` into Matchbox's `assets` directory.
This directory is automatically served by Matchbox.

### Create the Matchbox Configuration Files

The profiles we will create will reference `vmlinuz`, and `initramfs.xz`.
Download these files from the [release](https://github.com/talos-systems/talos/releases) of your choice, and place them in `/var/lib/matchbox/assets`.

#### Profiles

##### Control Plane Nodes

```json
{
  "id": "control-plane",
  "name": "control-plane",
  "boot": {
    "kernel": "/assets/vmlinuz",
    "initrd": ["/assets/initramfs.xz"],
    "args": [
      "initrd=initramfs.xz",
      "init_on_alloc=1",
      "slab_nomerge",
      "pti=on",
      "console=tty0",
      "console=ttyS0",
      "printk.devkmsg=on",
      "talos.platform=metal",
      "talos.config=http://matchbox.talos.dev/assets/controlplane.yaml"
    ]
  }
}
```

> Note: Be sure to change `http://matchbox.talos.dev` to the endpoint of your matchbox server.

##### Worker Nodes

```json
{
  "id": "default",
  "name": "default",
  "boot": {
    "kernel": "/assets/vmlinuz",
    "initrd": ["/assets/initramfs.xz"],
    "args": [
      "initrd=initramfs.xz",
      "init_on_alloc=1",
      "slab_nomerge",
      "pti=on",
      "console=tty0",
      "console=ttyS0",
      "printk.devkmsg=on",
      "talos.platform=metal",
      "talos.config=http://matchbox.talos.dev/assets/worker.yaml"
    ]
  }
}
```

#### Groups

Now, create the following groups, and ensure that the `selector`s are accurate for your specific setup.

```json
{
  "id": "control-plane-1",
  "name": "control-plane-1",
  "profile": "control-plane",
  "selector": {
    ...
  }
}
```

```json
{
  "id": "control-plane-2",
  "name": "control-plane-2",
  "profile": "control-plane",
  "selector": {
    ...
  }
}
```

```json
{
  "id": "control-plane-3",
  "name": "control-plane-3",
  "profile": "control-plane",
  "selector": {
    ...
  }
}
```

```json
{
  "id": "default",
  "name": "default",
  "profile": "default"
}
```

### Boot the Machines

Now that we have our configuraton files in place, boot all the machines.
Talos will come up on each machine, grab its' configuration file, and bootstrap itself.

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
