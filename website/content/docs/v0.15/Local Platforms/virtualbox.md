---
title: VirtualBox
description: "Creating Talos Kubernetes cluster using VurtualBox VMs."
---

In this guide we will create a Kubernetes cluster using VirtualBox.

## Video Walkthrough

To see a live demo of this writeup, visit Youtube here:

<iframe width="560" height="315" src="https://www.youtube.com/embed/bIszwavcBiU" frameborder="0" allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture" allowfullscreen></iframe>

## Installation

### How to Get VirtualBox

Install VirtualBox with your operating system package manager or from the [website](https://www.virtualbox.org/).
For example, on Ubuntu for x86:

```bash
apt install virtualbox
```

### Install talosctl

You can download `talosctl` via
[github.com/talos-systems/talos/releases](https://github.com/talos-systems/talos/releases)

```bash
curl https://github.com/talos-systems/talos/releases/download/<version>/talosctl-<platform>-<arch> -L -o talosctl
```

For example version `v0.15.0` for `linux` platform:

```bash
curl https://github.com/talos-systems/talos/releases/latest/download/talosctl-linux-amd64 -L -o talosctl
sudo cp talosctl /usr/local/bin
sudo chmod +x /usr/local/bin/talosctl
```

### Download ISO Image

In order to install Talos in VirtualBox, you will need the ISO image from the Talos release page.
You can download `talos-amd64.iso` via
[github.com/talos-systems/talos/releases](https://github.com/talos-systems/talos/releases)

```bash
mkdir -p _out/
curl https://github.com/talos-systems/talos/releases/download/<version>/talos-<arch>.iso -L -o _out/talos-<arch>.iso
```

For example version `v0.15.0` for `linux` platform:

```bash
mkdir -p _out/
curl https://github.com/talos-systems/talos/releases/latest/download/talos-amd64.iso -L -o _out/talos-amd64.iso
```

## Create VMs

Start by creating a new VM by clicking the "New" button in the VirtualBox UI:

<img src="/images/vbox-guide/new-vm.png" width="500px">

Supply a name for this VM, and specify the Type and Version:

<img src="/images/vbox-guide/vm-name.png" width="500px">

Edit the memory to supply at least 2GB of RAM for the VM:

<img src="/images/vbox-guide/vm-memory.png" width="500px">

Proceed through the disk settings, keeping the defaults.
You can increase the disk space if desired.

Once created, select the VM and hit "Settings":

<img src="/images/vbox-guide/edit-settings.png" width="500px">

In the "System" section, supply at least 2 CPUs:

<img src="/images/vbox-guide/edit-cpu.png" width="500px">

In the "Network" section, switch the network "Attached To" section to "Bridged Adapter":

<img src="/images/vbox-guide/edit-nic.png" width="500px">

Finally, in the "Storage" section, select the optical drive and, on the right, select the ISO by browsing your filesystem:

<img src="/images/vbox-guide/add-iso.png" width="500px">

Repeat this process for a second VM to use as a worker node.
You can also repeat this for additional nodes desired.

## Start Control Plane Node

Once the VMs have been created and updated, start the VM that will be the first control plane node.
This VM will boot the ISO image specified earlier and enter "maintenance mode".
Once the machine has entered maintenance mode, there will be a console log that details the IP address that the node received.
Take note of this IP address, which will be referred to as `$CONTROL_PLANE_IP` for the rest of this guide.
If you wish to export this IP as a bash variable, simply issue a command like `export CONTROL_PLANE_IP=1.2.3.4`.

<img src="/images/vbox-guide/maintenance-mode.png" width="500px">

## Generate Machine Configurations

With the IP address above, you can now generate the machine configurations to use for installing Talos and Kubernetes.
Issue the following command, updating the output directory, cluster name, and control plane IP as you see fit:

```bash
talosctl gen config talos-vbox-cluster https://$CONTROL_PLANE_IP:6443 --output-dir _out
```

This will create several files in the `_out` directory: controlplane.yaml, worker.yaml, and talosconfig.

## Create Control Plane Node

Using the `controlplane.yaml` generated above, you can now apply this config using talosctl.
Issue:

```bash
talosctl apply-config --insecure --nodes $CONTROL_PLANE_IP --file _out/controlplane.yaml
```

You should now see some action in the VirtualBox console for this VM.
Talos will be installed to disk, the VM will reboot, and then Talos will configure the Kubernetes control plane on this VM.

> Note: This process can be repeated multiple times to create an HA control plane.

## Create Worker Node

Create at least a single worker node using a process similar to the control plane creation above.
Start the worker node VM and wait for it to enter "maintenance mode".
Take note of the worker node's IP address, which will be referred to as `$WORKER_IP`

Issue:

```bash
talosctl apply-config --insecure --nodes $WORKER_IP --file _out/worker.yaml
```

> Note: This process can be repeated multiple times to add additional workers.

## Using the Cluster

Once the cluster is available, you can make use of `talosctl` and `kubectl` to interact with the cluster.
For example, to view current running containers, run `talosctl containers` for a list of containers in the `system` namespace, or `talosctl containers -k` for the `k8s.io` namespace.
To view the logs of a container, use `talosctl logs <container>` or `talosctl logs -k <container>`.

First, configure talosctl to talk to your control plane node by issuing the following, updating paths and IPs as necessary:

```bash
export TALOSCONFIG="_out/talosconfig"
talosctl config endpoint $CONTROL_PLANE_IP
talosctl config node $CONTROL_PLANE_IP
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

You can then use kubectl in this fashion:

```bash
kubectl get nodes
```

## Cleaning Up

To cleanup, simply stop and delete the virtual machines from the VirtualBox UI.
