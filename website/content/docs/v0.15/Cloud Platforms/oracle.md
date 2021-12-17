---
title: "Oracle"
description: "Creating a cluster via the CLI (oci) on OracleCloud.com."
---

## Upload image

Oracle Cloud at the moment does not have a Talos official image.
So you can use [Bring Your Own Image (BYOI)](https://docs.oracle.com/en-us/iaas/Content/Compute/References/bringyourownimage.htm) approach.

Once the image is uploaded, set the ```Boot volume type``` to ``Paravirtualized`` mode.

OracleCloud has highly available NTP service, it can be enabled in Talos machine config with:

```yaml
machine:
  time:
    servers:
      - 169.254.169.254
```

## Creating a Cluster via the CLI

```bash
```

### Create a Load Balancer

Create a load balancer by issuing the commands shown below.
Save the IP/DNS name, as this info will be used in the next step.

```bash
```

### Create the Machine Configuration Files

#### Generating Base Configurations

Using the IP/DNS name of the loadbalancer created earlier, generate the base configuration files for the Talos machines by issuing:

```bash
$ talosctl gen config talos-k8s-oracle-tutorial https://<load balancer IP or DNS>:6443
created controlplane.yaml
created worker.yaml
created talosconfig
```

At this point, you can modify the generated configs to your liking.
Optionally, you can specify `--config-patch` with RFC6902 jsonpatches which will be applied during the config generation.

#### Validate the Configuration Files

Validate any edited machine configs with:

```bash
$ talosctl validate --config controlplane.yaml --mode cloud
controlplane.yaml is valid for cloud mode
$ talosctl validate --config worker.yaml --mode cloud
worker.yaml is valid for cloud mode
```

### Create the Servers

#### Create the Control Plane Nodes

Create the control plane nodes with:

```bash
```

#### Create the Worker Nodes

Create the worker nodes with the following command, repeating (and incrementing the name counter) as many times as desired.

```bash
```

### Bootstrap Etcd

To configure `talosctl` we will need the first control plane node's IP.
This can be found by issuing:

```bash
```

Set the `endpoints` and `nodes` for your talosconfig with:

```bash
talosctl --talosconfig talosconfig config endpoint <control-plane-1-IP>
talosctl --talosconfig talosconfig config node <control-plane-1-IP>
```

Bootstrap `etcd` on the first control plane node with:

```bash
talosctl --talosconfig talosconfig bootstrap
```

### Retrieve the `kubeconfig`

At this point we can retrieve the admin `kubeconfig` by running:

```bash
talosctl --talosconfig talosconfig kubeconfig .
```
