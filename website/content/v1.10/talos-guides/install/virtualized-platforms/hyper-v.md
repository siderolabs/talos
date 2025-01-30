---
title: "Hyper-V"
description: "Creating a Talos Kubernetes cluster using Hyper-V."
aliases:
  - ../../../virtualized-platforms/hyper-v
---

## Prerequisites

1. Download the latest `metal-amd64.iso` from the [GitHub releases page](https://github.com/siderolabs/talos/releases).
2. Create a `New-TalosVM` folder in one of your PS Module Path folders (`$env:PSModulePath -split ';'`) and save the [New-TalosVM.psm1](https://github.com/nebula-it/New-TalosVM/blob/main/Talos/1.0.0/Talos.psm1) there.

## Plan Overview

We will create a basic 3-node cluster with one control-plane node and two worker nodes.
The main difference between the control plane and worker nodes is the amount of RAM and an additional storage VHD for the worker nodes.
This can be customized to your preference.

We use a `VMNamePrefix` argument for the VM name prefix, not the full hostname.
This command will find any existing VM with that prefix and increment the highest suffix found.
For example, if `talos-cp01` and `talos-cp02` exist, it will create VMs starting from `talos-cp03`, depending on the `NumberOfVMs` argument.

## Setup a Control Plane Node

> Note: Ensure the `LAB` adapter exists in Hyper-V and is set to external.

Create a single control plane node with the following command:

```powershell
New-TalosVM -VMNamePrefix talos-cp -CPUCount 2 -StartupMemory 4GB -SwitchName LAB -TalosISOPath C:\ISO\metal-amd64.iso -NumberOfVMs 1 -VMDestinationBasePath 'D:\Virtual Machines\Test VMs\Talos'
```

This will create the `talos-cp01` VM and power it on.

## Setup Worker Nodes

Create two worker nodes with the following command:

```powershell
New-TalosVM -VMNamePrefix talos-worker -CPUCount 4 -StartupMemory 8GB -SwitchName LAB -TalosISOPath C:\ISO\metal-amd64.iso -NumberOfVMs 2 -VMDestinationBasePath 'D:\Virtual Machines\Test VMs\Talos' -StorageVHDSize 50GB
```

This will create `talos-worker01` and `talos-worker02` VMs, each with an additional 50GB VHD for storage (which can be used for Mayastor).

## Push Config to the Nodes

Once the VMs are ready, find their IP addresses from the VM console.
Push the config to the control plane node with:

```powershell
# Set control plane IP variable
$CONTROL_PLANE_IP='10.10.10.x'

# Generate Talos config
talosctl gen config talos-cluster https://$($CONTROL_PLANE_IP):6443 --output-dir .

# Apply config to control plane node
talosctl apply-config --insecure --nodes $CONTROL_PLANE_IP --file .\controlplane.yaml
```

## Push Config to Worker Nodes

Similarly, for the worker nodes:

```powershell
talosctl apply-config --insecure --nodes 10.10.10.x --file .\worker.yaml
```

Apply the config to both worker nodes.

## Bootstrap Cluster

With the nodes ready, bootstrap the Kubernetes cluster:

```powershell
# Set node and endpoint permanently in config
talosctl config endpoint $CONTROL_PLANE_IP
talosctl config node $CONTROL_PLANE_IP

# Bootstrap cluster
talosctl bootstrap

# Generate kubeconfig
talosctl kubeconfig .
```

## Remove ISO

After a successful bootstrap, remove the ISO from the Hyper-V instances (both worker and control plane).
Otherwise, Talos might fail to boot.

This will generate the `kubeconfig` file, which you can use to connect to the cluster.
