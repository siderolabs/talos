---
title: Getting Started
weight: 30
description: "A guide to setting up a Talos cluster"
---

This guide walks you through creating a simple Talos cluster with one control plane node and one or more worker nodes. 

If you're looking to set up a cluster with multiple control plane nodes, see [Production Notes]({{< relref "prodnotes" >}}).

**New to Talos?** Start with [Quickstart]({{< relref "quickstart" >}}) to create a local virtual cluster on your workstation.

**Planning for production?** See [Production Notes]({{< relref "prodnotes" >}}) for additional requirements and best practices.

**Installing on cloud or virtualized platforms?** Check out the [platform-specific guides]({{< relref "../talos-guides/install" >}}) for installation methods tailored to different environments.


## Prerequisites

To create a Kubernetes cluster with Talos, you’ll need to:

- **Install talosctl**: `talosctl` is the CLI tool used to interact with the Talos API. Since Talos Linux does not have SSH access, `talosctl` is the  primary tool for managing and configuring your Talos machines


    You can install `talosctl` on macOS or Linux by running:

```bash
brew install siderolabs/tap/talosctl
```

Refer to the [talosctl installation guide]({{< relref "../talos-guides/install/talosctl" >}}) for installation on other platforms.
    
- **Ensure network access**: Your machines will need internet access to download the Talos installer and container images, sync time, and more.


    If you’re working in a restricted network environment, check out the official documentation on using [registry proxies]({{< relref "../talos-guides/configuration/pull-through-cache" >}}), local registries, or setting up an [air-gapped installation]({{< relref "../advanced/air-gapped" >}}).

## Talos Cluster Setup Overview

Every Talos cluster follows the same process, regardless of where you deploy it:

1. **Boot** - Start machines with the Talos Linux image
2. **Configure** - Create a root of trust certificate authority and generate configuration files
3. **Apply** - Apply machine configurations to the nodes
4. **Connect** - Set up your local `talosctl` client
5. **Bootstrap** - Initialize the Kubernetes cluster. 

**Note**: You can also opt to use [Omni](https://www.siderolabs.com/omni-signup/) to create a Talos cluster spanning different platforms, bare metal, cloud providers, and virtual machines.

Let's walk through each step and create a Talos cluster.

## Step 1: Download The Talos Linux Image

Get the latest ISO for your architecture from our [Image factory](https://factory.talos.dev/).


## Step 2: Boot Your Machine

Boot your hardware using the ISO image you just downloaded. At this stage, you'll:

- Boot one machine as your control plane node.
- Boot additional machines as worker nodes (this is optional).

You’ll see the Talos dashboard once your hardware boots from the ISO image.

**Note**: The ISO runs entirely in RAM and won't modify your disks until you apply a configuration.

**Troubleshooting network connectivity:** If your machine fails to establish a network connection after booting, you may need to add network drivers through system extensions. Add these extensions to your Talos image via the [Image factory](https://factory.talos.dev/), or see the [system extensions repository](https://github.com/siderolabs/extensions) for more information.

## Step 3: Store Your Node IP Addresses in a Variable

To create variables for your machines’ IP addresses:


1. Copy the IP address displayed on each machine console, including the control plane and any worker nodes you’ve created. 

If you don’t have a display connected, retrieve the IP addresses from your DHCP server.

![IP address display](/images/IP-address-install-display.png)



2. Create a variable for your control plane node’s IP address by replacing `<your-control-plane-ip>` with the actual IP:

```bash
export CONTROL_PLANE_IP=<your-control-plane-ip>
```

3. If you have worker nodes, store their IP addresses in a Bash array. Replace each `<worker-ip>` placeholder with the actual IP address of a worker node. You can include as many IP addresses as needed:

```bash
WORKER_IP=("<worker-ip-1>" "<worker-ip-2>" "<worker-ip-3>"...)
```

## Step 4: Unmount the ISO

Unplug your installation USB drive or unmount the ISO. This prevents you from accidentally installing to the USB drive and makes it clearer which disk to select for installation.

## Step 5: Learn About Your Installation Disks 

When you first boot your machine from the ISO, Talos runs temporarily in memory. This means that your Talos nodes, configurations, and cluster membership won't survive reboots or power cycles.

However, once you apply the machine configuration (which you'll do later in this guide), you'll install Talos, its complete operating system, and your configuration to a specified disk for permanent storage.

Run this command to view all the available disks on your control plane:

```bash
talosctl get disks --insecure --nodes $CONTROL_PLANE_IP
```

Note the disk ID (e.g., `sda`, `vda`) as you will use it in the next step.



## Step 6: Generate Cluster Configuration

Talos Linux is configured entirely using declarative configuration files avoiding the need to deal with SSH and running commands.

To generate these declarative configuration files: 

1. Define variables for your cluster name and the disk ID from step 5. Replace the placeholders with your actual values:

```bash
export CLUSTER_NAME=<cluster_name>
export DISK_NAME=<control_plane_disk_name>
```

2. Run this command to generate the configuration file:

```bash
talosctl gen config $CLUSTER_NAME https://$CONTROL_PLANE_IP:6443 --install-disk /dev/$DISK_NAME
```

This command generates machine configurations that specify the Kubernetes API endpoint (which is your control plane node's IP) for cluster communication and the target disk for the Talos installation.

You'll get three files from this command:

- **controlplane.yaml**: The configuration for your control plane.
- **worker.yaml**: The configuration for your worker nodes.
- **talosconfig**: Your `talosctl` configuration file, used to connect to and authenticate access to your cluster.



## Step 7: Apply Configurations

Now that you've created your configurations, it's time to apply them to bring your nodes and cluster online:

1. Run this command to apply the control plane configuration:

```bash
talosctl apply-config --insecure --nodes $CONTROL_PLANE_IP --file controlplane.yaml
```

2. Next, apply the worker node configuration:

```bash
for ip in "${WORKER_IP[@]}"; do
    echo "Applying config to worker node: $ip"
    talosctl apply-config --insecure --nodes "$ip" --file worker.yaml
done
```

## Step 8: Bootstrap Your Etcd Cluster

Wait for your control plane node to finish booting, then bootstrap your etcd cluster by running:

```bash
talosctl bootstrap --nodes $CONTROL_PLANE_IP --endpoints $CONTROL_PLANE_IP --talosconfig=./talosconfig
```

<Note>Run this command ONCE on a SINGLE control plane node. If you have multiple control plane nodes, you can choose any of them.</Note>

## Step 9: Get Kubernetes Access

Download your `kubeconfig` file to start using `kubectl`.

You can get your `kubeconfig` file in one of two ways:

- Merge your new cluster into your local Kubernetes configuration:

```bash
talosctl kubeconfig --nodes $CONTROL_PLANE_IP --endpoints $CONTROL_PLANE_IP \
  --talosconfig=./talosconfig
```

- **S**pecify a filename if you prefer not to merge with your default Kubernetes configuration:

```bash
talosctl kubeconfig alternative-kubeconfig --nodes $CONTROL_PLANE_IP --endpoints $CONTROL_PLANE_IP \
  --talosconfig=./talosconfig
export KUBECONFIG=./alternative-kubeconfig

```

## Step 10: Check Cluster Health

Run the following command to check the health of your nodes:

```bash
talosctl --nodes $CONTROL_PLANE_IP --endpoints $CONTROL_PLANE_IP --talosconfig=./talosconfig health
```

## Step 11: Verify Node Registration

Confirm that your nodes are registered in Kubernetes:

```bash
kubectl get nodes
```
You should see your control plane and worker nodes listed with a **Ready** status.

## Next Steps

Congratulations! You now have a working Kubernetes cluster on Talos Linux . 

For a list of all the commands and operations that `talosctl` provides, see the CLI reference.

### What's Next?
- Deploy your first application
- Set up persistent storage
- Configure networking policies
- Explore the talosctl CLI reference
- Plan your production deployment

