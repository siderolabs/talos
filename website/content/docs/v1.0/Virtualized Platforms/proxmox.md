---
title: Proxmox
description: "Creating Talos Kubernetes cluster using Proxmox."
---

In this guide we will create a Kubernetes cluster using Proxmox.

## Video Walkthrough

To see a live demo of this writeup, visit Youtube here:

<iframe width="560" height="315" src="https://www.youtube.com/embed/MyxigW4_QFM" frameborder="0" allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture" allowfullscreen></iframe>

## Installation

### How to Get Proxmox

It is assumed that you have already installed Proxmox onto the server you wish to create Talos VMs on.
Visit the [Proxmox](https://www.proxmox.com/en/downloads) downloads page if necessary.

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

In order to install Talos in Proxmox, you will need the ISO image from the Talos release page.
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

## Upload ISO

From the Proxmox UI, select the "local" storage and enter the "Content" section.
Click the "Upload" button:

<img src="/images/proxmox-guide/click-upload.png" width="500px">

Select the ISO you downloaded previously, then hit "Upload"

<img src="/images/proxmox-guide/select-iso.png" width="500px">

## Create VMs

Start by creating a new VM by clicking the "Create VM" button in the Proxmox UI:

<img src="/images/proxmox-guide/create-vm.png" width="500px">

Fill out a name for the new VM:

<img src="/images/proxmox-guide/edit-vm-name.png" width="500px">

In the OS tab, select the ISO we uploaded earlier:

<img src="/images/proxmox-guide/edit-os.png" width="500px">

Keep the defaults set in the "System" tab.

Keep the defaults in the "Hard Disk" tab as well, only changing the size if desired.

In the "CPU" section, give at least 2 cores to the VM:

<img src="/images/proxmox-guide/edit-cpu.png" width="500px">

Verify that the RAM is set to at least 2GB:

<img src="/images/proxmox-guide/edit-ram.png" width="500px">

Keep the default values for networking, verifying that the VM is set to come up on the bridge interface:

<img src="/images/proxmox-guide/edit-nic.png" width="500px">

Finish creating the VM by clicking through the "Confirm" tab and then "Finish".

Repeat this process for a second VM to use as a worker node.
You can also repeat this for additional nodes desired.

## Start Control Plane Node

Once the VMs have been created and updated, start the VM that will be the first control plane node.
This VM will boot the ISO image specified earlier and enter "maintenance mode".

### With DHCP server

Once the machine has entered maintenance mode, there will be a console log that details the IP address that the node received.
Take note of this IP address, which will be referred to as `$CONTROL_PLANE_IP` for the rest of this guide.
If you wish to export this IP as a bash variable, simply issue a command like `export CONTROL_PLANE_IP=1.2.3.4`.

<img src="/images/proxmox-guide/maintenance-mode.png" width="500px">

### Without DHCP server

To apply the machine configurations in maintenance mode, VM has to have IP on the network.
So you can set it on boot time manually.

<img src="/images/proxmox-guide/maintenance-mode-grub-menu.png" width="600px">

Press `e` on the boot time.
And set the IP parameters for the VM.
[Format is](https://www.kernel.org/doc/Documentation/filesystems/nfs/nfsroot.txt):

```bash
ip=<client-ip>:<srv-ip>:<gw-ip>:<netmask>:<host>:<device>:<autoconf>
```

For example $CONTROL_PLANE_IP will be 192.168.0.100 and gateway 192.168.0.1

```bash
linux /boot/vmlinuz init_on_alloc=1 slab_nomerge pti=on panic=0 consoleblank=0 printk.devkmsg=on earlyprintk=ttyS0 console=tty0 console=ttyS0 talos.platform=metal ip=192.168.0.100::192.168.0.1:255.255.255.0::eth0:off
```

<img src="/images/proxmox-guide/maintenance-mode-grub-menu-ip.png" width="630px">

Then press Ctrl-x or F10

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

You should now see some action in the Proxmox console for this VM.
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

## Cleaning Up

To cleanup, simply stop and delete the virtual machines from the Proxmox UI.
