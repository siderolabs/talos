---
title: "Hyper-V"
description: "Creating Talos Kubernetes cluster using Hyper-V."
---

# Pre-requisities 
1. Download the latest `talos-amd64.iso` ISO from github [releases page](https://github.com/talos-systems/talos/releases)
2. Create a New-TalosVM folder in any of your PS Module Path folders `$env:PSModulePath -split ';'` and save the [New-TalosVM.psm1](https://github.com/nebula-it/New-TalosVM/blob/main/New-TalosVM.psm1) there

# Plan overview
Here we will create a basic 3 node cluster with 1 control-plane and 2 worker nodes. The only difference between control plane and worker node is the amount of RAM and an additional storage VHD, this is personal perference so you can configure it to your liking.

We are using a `VMNamePrefix` argument for a VM Name prefix and not the full hostname. This command will find any existing VM with that prefix and +1 the highest suffix it finds. e.g if VMs `talos-cp01` and `talos-cp02` exist, this will create VMs starting from `talos-cp03`, depending on NumberOfVMs argument.

# Setup Control Plane Node
Use the following command to create control plane node:

```
New-TalosVM -VMNamePrefix talos-cp -CPUCount 2 -StartupMemory 4GB -SwitchName LAB -TalosISOPath C:\ISO\talos-amd64.iso -NumberOfVMs 1 -VMDestinationBasePath 'D:\Virtual Machines\Test VMs\Talos'
```
This will create `talos-cp01` VM and power it on.

# Setup Worker Nodes
Use the following command to create 2 worker nodes:
```
New-TalosVM -VMNamePrefix talos-w -CPUCount 4 -StartupMemory 8GB -SwitchName LAB -TalosISOPath C:\ISO\talos-amd64.iso -NumberOfVMs 2 -VMDestinationBasePath 'D:\Virtual Machines\Test VMs\Talos' -StorageVHDSize 50GB
```
This will create two VMs `talos-w01` and `talos-w02` and will attach an additional VHD of 50GB for storage. (which in my case will be passed to mayastor)

# Pushing config to the nodes
Now that our VMs are ready, find their IP addresses from console of VM.
```
# set control plane IP variable
$CONTROL_PLANE_IP='10.10.10.x'

# Generate talos config
talosctl gen config talos-cluster https://$($CONTROL_PLANE_IP):6443 --output-dir .

# Apply config to control plane node
talosctl apply-config --insecure --nodes $CONTROL_PLANE_IP --file .\controlplane.yaml
```

# Pusing config to worker nodes
For the worker nodes use worker.yml file
```
talosctl apply-config --insecure --nodes 10.10.10.x --file .\worker.yaml
```
Apply the above to both nodes

# Bootstrap cluster
Now that our nodes are ready, we are ready to spin up kubernetes cluster.
```
# Use following command to set node and endpoint permanantly in config so you dont have to type it everytime
talosctl config endpoint $CONTROL_PLANE_IP
talosctl config node $CONTROL_PLANE_IP

# Bootstrap cluster
talosctl bootstrap

# Generate kubeconfig
talosctl kubeconfig .
```

This will generate the kubeconfig file, you can use that to connect to cluster.
