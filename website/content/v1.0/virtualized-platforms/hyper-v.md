---
title: "Hyper-V"
description: "Creating a Talos Kubernetes cluster using Hyper-V."
---

## Pre-requisities

1. Download the latest `talos-amd64.iso` ISO from github [releases page](https://github.com/talos-systems/talos/releases)
2. Create a New-TalosVM folder in any of your PS Module Path folders `$env:PSModulePath -split ';'` and save the [New-TalosVM.psm1](https://github.com/nebula-it/New-TalosVM/blob/main/New-TalosVM.psm1) there

## Plan Overview

Here we will create a basic 3 node cluster with a single control-plane node and two worker nodes.
The only difference between control plane and worker node is the amount of RAM and an additional storage VHD.
This is personal preference and can be configured to your liking.

We are using a `VMNamePrefix` argument for a VM Name prefix and not the full hostname.
This command will find any existing VM with that prefix and "+1" the highest suffix it finds.
For example, if VMs `talos-cp01` and `talos-cp02` exist, this will create VMs starting from `talos-cp03`, depending on NumberOfVMs argument.

## Setup a Control Plane Node

Use the following command to create a single control plane node:

````powershell
New-TalosVM -VMNamePrefix talos-cp -CPUCount 2 -StartupMemory 4GB -SwitchName LAB -TalosISOPath C:\ISO\talos-amd64.iso -NumberOfVMs 1 -VMDestinationBasePath 'D:\Virtual Machines\Test VMs\Talos'
```

This will create `talos-cp01` VM and power it on.

## Setup Worker Nodes

Use the following command to create 2 worker nodes:

```powershell
New-TalosVM -VMNamePrefix talos-worker -CPUCount 4 -StartupMemory 8GB -SwitchName LAB -TalosISOPath C:\ISO\talos-amd64.iso -NumberOfVMs 2 -VMDestinationBasePath 'D:\Virtual Machines\Test VMs\Talos' -StorageVHDSize 50GB
```

This will create two VMs: `talos-worker01` and `talos-wworker02` and attach an additional VHD of 50GB for storage (which in my case will be passed to Mayastor).

## Pushing Config to the Nodes

Now that our VMs are ready, find their IP addresses from console of VM.
With that information, push config to the control plane node with:

```powershell
# set control plane IP variable
$CONTROL_PLANE_IP='10.10.10.x'

# Generate talos config
talosctl gen config talos-cluster https://$($CONTROL_PLANE_IP):6443 --output-dir .

# Apply config to control plane node
talosctl apply-config --insecure --nodes $CONTROL_PLANE_IP --file .\controlplane.yaml
```

## Pushing Config to Worker Nodes

Similarly, for the workers:

```powershell
talosctl apply-config --insecure --nodes 10.10.10.x --file .\worker.yaml
```

Apply the config to both nodes.

## Bootstrap Cluster

Now that our nodes are ready, we are ready to bootstrap the Kubernetes cluster.

```powershell
# Use following command to set node and endpoint permanantly in config so you dont have to type it everytime
talosctl config endpoint $CONTROL_PLANE_IP
talosctl config node $CONTROL_PLANE_IP

# Bootstrap cluster
talosctl bootstrap

# Generate kubeconfig
talosctl kubeconfig .
```

This will generate the `kubeconfig` file, you can use to connect to the cluster.
