---
title: "DigitalOcean"
description: "Creating a cluster via the CLI on DigitalOcean."
---

## Creating a Cluster via the CLI

In this guide we will create an HA Kubernetes cluster with 1 worker node.
We assume an existing [Space](https://www.digitalocean.com/docs/spaces/), and some familiarity with DigitalOcean.
If you need more information on DigitalOcean specifics, please see the [official DigitalOcean documentation](https://www.digitalocean.com/docs/).

### Create the Image

First, download the DigitalOcean image from a Talos release.
Extract the archive to get the `disk.raw` file, compress it using `gzip` to `disk.raw.gz`.

Using an upload method of your choice (`doctl` does not have Spaces support), upload the image to a space.
Now, create an image using the URL of the uploaded image:

```bash
doctl compute image create \
    --region $REGION \
    --image-description talos-digital-ocean-tutorial \
    --image-url https://talos-tutorial.$REGION.digitaloceanspaces.com/disk.raw.gz \
    Talos
```

Save the image ID.
We will need it when creating droplets.

### Create a Load Balancer

```bash
doctl compute load-balancer create \
    --region $REGION \
    --name talos-digital-ocean-tutorial-lb \
    --tag-name talos-digital-ocean-tutorial-control-plane \
    --health-check protocol:tcp,port:6443,check_interval_seconds:10,response_timeout_seconds:5,healthy_threshold:5,unhealthy_threshold:3 \
    --forwarding-rules entry_protocol:tcp,entry_port:443,target_protocol:tcp,target_port:6443
```

We will need the IP of the load balancer.
Using the ID of the load balancer, run:

```bash
doctl compute load-balancer get --format IP <load balancer ID>
```

Save it, as we will need it in the next step.

### Create the Machine Configuration Files

#### Generating Base Configurations

Using the DNS name of the loadbalancer created earlier, generate the base configuration files for the Talos machines:

```bash
$ talosctl gen config talos-k8s-digital-ocean-tutorial https://<load balancer IP or DNS>:<port>
created controlplane.yaml
created worker.yaml
created talosconfig
```

At this point, you can modify the generated configs to your liking.
Optionally, you can specify `--config-patch` with RFC6902 jsonpatch which will be applied during the config generation.

#### Validate the Configuration Files

```bash
$ talosctl validate --config controlplane.yaml --mode cloud
controlplane.yaml is valid for cloud mode
$ talosctl validate --config worker.yaml --mode cloud
worker.yaml is valid for cloud mode
```

### Create the Droplets

#### Create the Control Plane Nodes

Run the following twice, to give ourselves three total control plane nodes:

```bash
doctl compute droplet create \
    --region $REGION \
    --image <image ID> \
    --size s-2vcpu-4gb \
    --enable-private-networking \
    --tag-names talos-digital-ocean-tutorial-control-plane \
    --user-data-file controlplane.yaml \
    --ssh-keys <ssh key fingerprint> \
    talos-control-plane-1
doctl compute droplet create \
    --region $REGION \
    --image <image ID> \
    --size s-2vcpu-4gb \
    --enable-private-networking \
    --tag-names talos-digital-ocean-tutorial-control-plane \
    --user-data-file controlplane.yaml \
    --ssh-keys <ssh key fingerprint> \
    talos-control-plane-2
doctl compute droplet create \
    --region $REGION \
    --image <image ID> \
    --size s-2vcpu-4gb \
    --enable-private-networking \
    --tag-names talos-digital-ocean-tutorial-control-plane \
    --user-data-file controlplane.yaml \
    --ssh-keys <ssh key fingerprint> \
    talos-control-plane-3
```

> Note: Although SSH is not used by Talos, DigitalOcean still requires that an SSH key be associated with the droplet.
> Create a dummy key that can be used to satisfy this requirement.

#### Create the Worker Nodes

Run the following to create a worker node:

```bash
doctl compute droplet create \
    --region $REGION \
    --image <image ID> \
    --size s-2vcpu-4gb \
    --enable-private-networking \
    --user-data-file worker.yaml \
    --ssh-keys <ssh key fingerprint> \
    talos-worker-1
```

### Bootstrap Etcd

To configure `talosctl` we will need the first control plane node's IP:

```bash
doctl compute droplet get --format PublicIPv4 <droplet ID>
```

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
