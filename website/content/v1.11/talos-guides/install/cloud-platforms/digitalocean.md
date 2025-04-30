---
title: "DigitalOcean"
description: "Creating a cluster via the CLI on DigitalOcean."
aliases:
  - ../../../cloud-platforms/digitalocean
---

## Creating a Talos Linux Cluster on Digital Ocean via the CLI

In this guide we will create an HA Kubernetes cluster with 1 worker node, in the NYC region.
We assume an existing [Space](https://www.digitalocean.com/docs/spaces/), and some familiarity with DigitalOcean.
If you need more information on DigitalOcean specifics, please see the [official DigitalOcean documentation](https://www.digitalocean.com/docs/).

### Create the Image

Download the DigitalOcean image `digital-ocean-amd64.raw.gz` from the [Image Factory](https://factory.talos.dev/image/376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba/{{< release >}}/digital-ocean-amd64.raw.gz).

>Note: the minimum version of Talos required to support Digital Ocean is v1.3.3.

Using an upload method of your choice (`doctl` does not have Spaces support), upload the image to a space.
(It's easy to drag the image file to the space using DigitalOcean's web console.)

*Note:* Make sure you upload the file as `public`.

Now, create an image using the URL of the uploaded image:

```bash
export REGION=nyc3

doctl compute image create \
    --region $REGION \
    --image-description talos-digital-ocean-tutorial \
    --image-url https://$SPACENAME.$REGION.digitaloceanspaces.com/digital-ocean-amd64.raw.gz \
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

Note the returned ID of the load balancer.

We will need the IP of the load balancer.
Using the ID of the load balancer, run:

```bash
doctl compute load-balancer get --format IP <load balancer ID>
```

Note that it may take a few minutes before the load balancer is provisioned, so repeat this command until it returns with the IP address.

### Create the Machine Configuration Files

Using the IP address (or DNS name, if you have created one) of the loadbalancer, generate the base configuration files for the Talos machines.
Also note that the load balancer forwards port 443 to port 6443 on the associated nodes, so we should use 443 as the port in the config definition:

```bash
$ talosctl gen config talos-k8s-digital-ocean-tutorial https://<load balancer IP or DNS>:443
created controlplane.yaml
created worker.yaml
created talosconfig
```

### Create the Droplets

#### Create a dummy SSH key

> Although SSH is not used by Talos, DigitalOcean requires that an SSH key be associated with a droplet during creation.
> We will create a dummy key that can be used to satisfy this requirement.

```bash
doctl compute ssh-key create --public-key "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQDbl0I1s/yOETIKjFr7mDLp8LmJn6OIZ68ILjVCkoN6lzKmvZEqEm1YYeWoI0xgb80hQ1fKkl0usW6MkSqwrijoUENhGFd6L16WFL53va4aeJjj2pxrjOr3uBFm/4ATvIfFTNVs+VUzFZ0eGzTgu1yXydX8lZMWnT4JpsMraHD3/qPP+pgyNuI51LjOCG0gVCzjl8NoGaQuKnl8KqbSCARIpETg1mMw+tuYgaKcbqYCMbxggaEKA0ixJ2MpFC/kwm3PcksTGqVBzp3+iE5AlRe1tnbr6GhgT839KLhOB03j7lFl1K9j1bMTOEj5Io8z7xo/XeF2ZQKHFWygAJiAhmKJ dummy@dummy.local" dummy

```

Note the ssh key ID that is returned - we will use it in creating the droplets.

#### Create the Control Plane Nodes

Run the following commands to create three control plane nodes:

```bash
doctl compute droplet create \
    --region $REGION \
    --image <image ID> \
    --size s-2vcpu-4gb \
    --enable-private-networking \
    --tag-names talos-digital-ocean-tutorial-control-plane \
    --user-data-file controlplane.yaml \
    --ssh-keys <ssh key ID> \
    talos-control-plane-1
doctl compute droplet create \
    --region $REGION \
    --image <image ID> \
    --size s-2vcpu-4gb \
    --enable-private-networking \
    --tag-names talos-digital-ocean-tutorial-control-plane \
    --user-data-file controlplane.yaml \
    --ssh-keys <ssh key ID> \
    talos-control-plane-2
doctl compute droplet create \
    --region $REGION \
    --image <image ID> \
    --size s-2vcpu-4gb \
    --enable-private-networking \
    --tag-names talos-digital-ocean-tutorial-control-plane \
    --user-data-file controlplane.yaml \
    --ssh-keys <ssh key ID> \
    talos-control-plane-3
```

Note the droplet ID returned for the first control plane node.

#### Create the Worker Nodes

Run the following to create a worker node:

```bash
doctl compute droplet create \
    --region $REGION \
    --image <image ID> \
    --size s-2vcpu-4gb \
    --enable-private-networking \
    --user-data-file worker.yaml \
    --ssh-keys <ssh key ID>  \
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

We can also watch the cluster bootstrap via:

```bash
talosctl --talosconfig talosconfig health
```
