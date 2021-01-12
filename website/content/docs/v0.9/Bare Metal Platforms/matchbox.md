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
created init.yaml
created controlplane.yaml
created join.yaml
created talosconfig
```

At this point, you can modify the generated configs to your liking.

#### Validate the Configuration Files

```bash
$ talosctl validate --config init.yaml --mode metal
init.yaml is valid for metal mode
$ talosctl validate --config controlplane.yaml --mode metal
controlplane.yaml is valid for metal mode
$ talosctl validate --config join.yaml --mode metal
join.yaml is valid for metal mode
```

#### Publishing the Machine Configuration Files

In bare-metal setups it is up to the user to provide the configuration files over HTTP(S).
A special kernel parameter (`talos.config`) must be used to inform Talos about _where_ it should retreive its' configuration file.
To keep things simple we will place `init.yaml`, `controlplane.yaml`, and `join.yaml` into Matchbox's `assets` directory.
This directory is automatically served by Matchbox.

### Create the Matchbox Configuration Files

The profiles we will create will reference `vmlinuz`, and `initramfs.xz`.
Download these files from the [release](https://github.com/talos-systems/talos/releases) of your choice, and place them in `/var/lib/matchbox/assets`.

#### Profiles

##### The Bootstrap Node

```json
{
  "id": "init",
  "name": "init",
  "boot": {
    "kernel": "/assets/vmlinuz",
    "initrd": ["/assets/initramfs.xz"],
    "args": [
      "initrd=initramfs.xz",
      "init_on_alloc=1",
      "init_on_free=1",
      "slab_nomerge",
      "pti=on",
      "console=tty0",
      "console=ttyS0",
      "printk.devkmsg=on",
      "talos.platform=metal",
      "talos.config=http://matchbox.talos.dev/assets/init.yaml"
    ]
  }
}
```

> Note: Be sure to change `http://matchbox.talos.dev` to the endpoint of your matchbox server.

##### Additional Control Plane Nodes

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
      "init_on_free=1",
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
      "init_on_free=1",
      "slab_nomerge",
      "pti=on",
      "console=tty0",
      "console=ttyS0",
      "printk.devkmsg=on",
      "talos.platform=metal",
      "talos.config=http://matchbox.talos.dev/assets/join.yaml"
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
  "profile": "init",
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

### Retrieve the `kubeconfig`

At this point we can retrieve the admin `kubeconfig` by running:

```bash
talosctl --talosconfig talosconfig config endpoint <control plane 1 IP>
talosctl --talosconfig talosconfig config node <control plane 1 IP>
talosctl --talosconfig talosconfig kubeconfig .
```
