---
title: "Openstack"
description: "Creating a cluster via the CLI on Openstack."
---

## Creating a Cluster via the CLI

In this guide, we will create an HA Kubernetes cluster in Openstack with 1 worker node.
We will assume an existing some familiarity with Openstack.
If you need more information on Openstack specifics, please see the [official Openstack documentation](https://docs.openstack.org).

### Environment Setup

You should have an existing openrc file.
This file will provide environment variables necessary to talk to your Openstack cloud.
See [here](https://docs.openstack.org/newton/user-guide/common/cli-set-environment-variables-using-openstack-rc.html) for instructions on fetching this file.

### Create the Image

First, download the Openstack image from a Talos [release](https://github.com/talos-systems/talos/releases).
These images are called `openstack-$ARCH.tar.gz`.
Untar this file with `tar -xvf openstack-$ARCH.tar.gz`.
The resulting file will be called `disk.raw`.

#### Upload the Image

Once you have the image, you can upload to Openstack with:

```bash
openstack image create --public --disk-format raw --file disk.raw talos
```

### Network Infrastructure

#### Load Balancer and Network Ports

Once the image is prepared, you will need to work through setting up the network.
Issue the following to create a load balancer, the necessary network ports for each control plane node, and associations between the two.

Creating loadbalancer:

```bash
# Create load balancer, updating vip-subnet-id if necessary
openstack loadbalancer create --name talos-control-plane --vip-subnet-id public

# Create listener
openstack loadbalancer listener create --name talos-control-plane-listener --protocol TCP --protocol-port 6443 talos-control-plane

# Pool and health monitoring
openstack loadbalancer pool create --name talos-control-plane-pool --lb-algorithm ROUND_ROBIN --listener talos-control-plane-listener --protocol TCP
openstack loadbalancer healthmonitor create --delay 5 --max-retries 4 --timeout 10 --type TCP talos-control-plane-pool
```

Creating ports:

```bash
# Create ports for control plane nodes, updating network name if necessary
openstack port create --network shared talos-control-plane-1
openstack port create --network shared talos-control-plane-2
openstack port create --network shared talos-control-plane-3

# Create floating IPs for the ports, so that you will have talosctl connectivity to each control plane
openstack floating ip create --port talos-control-plane-1 public
openstack floating ip create --port talos-control-plane-2 public
openstack floating ip create --port talos-control-plane-3 public
```

> Note: Take notice of the private and public IPs associated with each of these ports, as they will be used in the next step.
> Additionally, take node of the port ID, as it will be used in server creation.

Associate port's private IPs to loadbalancer:

```bash
# Create members for each port IP, updating subnet-id and address as necessary.
openstack loadbalancer member create --subnet-id shared-subnet --address <PRIVATE IP OF talos-control-plane-1 PORT> --protocol-port 6443 talos-control-plane-pool
openstack loadbalancer member create --subnet-id shared-subnet --address <PRIVATE IP OF talos-control-plane-2 PORT> --protocol-port 6443 talos-control-plane-pool
openstack loadbalancer member create --subnet-id shared-subnet --address <PRIVATE IP OF talos-control-plane-3 PORT> --protocol-port 6443 talos-control-plane-pool
```

#### Security Groups

This example uses the default security group in Openstack.
Ports have been opened to ensure that connectivity from both inside and outside the group is possible.
You will want to allow, at a minimum, ports 6443 (Kubernetes API server) and 50000 (Talos API) from external sources.
It is also recommended to allow communication over all ports from within the subnet.

### Cluster Configuration

With our networking bits setup, we'll fetch the IP for our load balancer and create our configuration files.

```bash
LB_PUBLIC_IP=$(openstack loadbalancer show talos-control-plane -f json | jq -r .vip_address)

talosctl gen config talos-k8s-openstack-tutorial https://${LB_PUBLIC_IP}:6443
```

Additionally, you can specify `--config-patch` with RFC6902 jsonpatch which will be applied during the config generation.

### Compute Creation

We are now ready to create our Openstack nodes.

Create control plane:

```bash
# Create control planes 2 and 3, substituting the same info.
for i in $( seq 1 3 ); do
  openstack server create talos-control-plane-$i --flavor m1.small --nic port-id=talos-control-plane-$i --image talos --user-data /path/to/controlplane.yaml
done
```

Create worker:

```bash
# Update network name as necessary.
openstack server create talos-worker-1 --flavor m1.small --network shared --image talos --user-data /path/to/worker.yaml
```

> Note: This step can be repeated to add more workers.

### Bootstrap Etcd

You should now be able to interact with your cluster with `talosctl`.
We will use one of the floating IPs we allocated earlier.
It does not matter which one.

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
