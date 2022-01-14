---
title: "VMware"
description: "Creating Talos Kubernetes cluster using VMware."
---

## Creating a Cluster via the `govc` CLI

In this guide we will create an HA Kubernetes cluster with 2 worker nodes.
We will use the `govc` cli which can be downloaded [here](https://github.com/vmware/govmomi/tree/master/govc#installation).

## Prereqs/Assumptions

This guide will use the virtual IP ("VIP") functionality that is built into Talos in order to provide a stable, known IP for the Kubernetes control plane.
This simply means the user should pick an IP on their "VM Network" to designate for this purpose and keep it handy for future steps.

## Create the Machine Configuration Files

### Generating Base Configurations

Using the VIP chosen in the prereq steps, we will now generate the base configuration files for the Talos machines.
This can be done with the `talosctl gen config ...` command.
Take note that we will also use a JSON6902 patch when creating the configs so that the control plane nodes get some special information about the VIP we chose earlier, as well as a daemonset to install vmware tools on talos nodes.

First, download `the cp.patch` to your local machine and edit the VIP to match your chosen IP.
You can do this by issuing `https://raw.githubusercontent.com/talos-systems/talos/master/website/content/docs/v0.15/Virtualized%20Platforms/vmware/cp.patch`.
It's contents should look like the following:

```yaml
- op: add
  path: /machine/network
  value:
    interfaces:
    - interface: eth0
      dhcp: true
      vip:
        ip: <VIP>
- op: replace
  path: /cluster/extraManifests
  value:
    - "https://raw.githubusercontent.com/mologie/talos-vmtoolsd/master/deploy/unstable.yaml"
```

With the patch in hand, generate machine configs with:

```bash
$ talosctl gen config vmware-test https://<VIP>:<port> --config-patch-control-plane "$(yq r -j cp.patch)"
created controlplane.yaml
created worker.yaml
created talosconfig
```

At this point, you can modify the generated configs to your liking if needed.
Optionally, you can specify additional patches by adding to the `cp.patch` file downloaded earlier, or create your own patch files.

### Validate the Configuration Files

```bash
$ talosctl validate --config controlplane.yaml --mode cloud
controlplane.yaml is valid for cloud mode
$ talosctl validate --config worker.yaml --mode cloud
worker.yaml is valid for cloud mode
```

## Set Environment Variables

`govc` makes use of the following environment variables

```bash
export GOVC_URL=<vCenter url>
export GOVC_USERNAME=<vCenter username>
export GOVC_PASSWORD=<vCenter password>
```

> Note: If your vCenter installation makes use of self signed certificates, you'll want to export `GOVC_INSECURE=true`.

There are some additional variables that you may need to set:

```bash
export GOVC_DATACENTER=<vCenter datacenter>
export GOVC_RESOURCE_POOL=<vCenter resource pool>
export GOVC_DATASTORE=<vCenter datastore>
export GOVC_NETWORK=<vCenter network>
```

## Choose Install Approach

As part of this guide, we have a more automated install script that handles some of the complexity of importing OVAs and creating VMs.
If you wish to use this script, we will detail that next.
If you wish to carry out the manual approach, simply skip ahead to the "Manual Approach" section.

### Scripted Install

Download the `vmware.sh` script to your local machine.
You can do this by issuing `curl -fsSLO "https://raw.githubusercontent.com/talos-systems/talos/master/website/content/docs/v0.15/Virtualized%20Platforms/vmware/vmware.sh"`.
This script has default variables for things like Talos version and cluster name that may be interesting to tweak before deploying.

#### Import OVA

To create a content library and import the Talos OVA corresponding to the mentioned Talos version, simply issue:

```bash
./vsphere.sh upload_ova
```

#### Create Cluster

With the OVA uploaded to the content library, you can create a 5 node (by default) cluster with 3 control plane and 2 worker nodes:

```bash
./vsphere.sh create
```

This step will create a VM from the OVA, edit the settings based on the env variables used for VM size/specs, then power on the VMs.

You may now skip past the "Manual Approach" section down to "Bootstrap Cluster".

### Manual Approach

#### Import the OVA into vCenter

A `talos.ova` asset is published with each [release](https://github.com/talos-systems/talos/releases).
We will refer to the version of the release as `$TALOS_VERSION` below.
It can be easily exported with `export TALOS_VERSION="v0.3.0-alpha.10"` or similar.

```bash
curl -LO https://github.com/talos-systems/talos/releases/download/$TALOS_VERSION/talos.ova
```

Create a content library (if needed) with:

```bash
govc library.create <library name>
```

Import the OVA to the library with:

```bash
govc library.import -n talos-${TALOS_VERSION} <library name> /path/to/downloaded/talos.ova
```

#### Create the Bootstrap Node

We'll clone the OVA to create the bootstrap node (our first control plane node).

```bash
govc library.deploy <library name>/talos-${TALOS_VERSION} control-plane-1
```

Talos makes use of the `guestinfo` facility of VMware to provide the machine/cluster configuration.
This can be set using the `govc vm.change` command.
To facilitate persistent storage using the vSphere cloud provider integration with Kubernetes, `disk.enableUUID=1` is used.

```bash
govc vm.change \
  -e "guestinfo.talos.config=$(cat controlplane.yaml | base64)" \
  -e "disk.enableUUID=1" \
  -vm control-plane-1
```

#### Update Hardware Resources for the Bootstrap Node

- `-c` is used to configure the number of cpus
- `-m` is used to configure the amount of memory (in MB)

```bash
govc vm.change \
  -c 2 \
  -m 4096 \
  -vm control-plane-1
```

The following can be used to adjust the ephemeral disk size.

```bash
govc vm.disk.change -vm control-plane-1 -disk.name disk-1000-0 -size 10G
```

```bash
govc vm.power -on control-plane-1
```

#### Create the Remaining Control Plane Nodes

```bash
govc library.deploy <library name>/talos-${TALOS_VERSION} control-plane-2
govc vm.change \
  -e "guestinfo.talos.config=$(base64 controlplane.yaml)" \
  -e "disk.enableUUID=1" \
  -vm control-plane-2

govc library.deploy <library name>/talos-${TALOS_VERSION} control-plane-3
govc vm.change \
  -e "guestinfo.talos.config=$(base64 controlplane.yaml)" \
  -e "disk.enableUUID=1" \
  -vm control-plane-3
```

```bash
govc vm.change \
  -c 2 \
  -m 4096 \
  -vm control-plane-2

govc vm.change \
  -c 2 \
  -m 4096 \
  -vm control-plane-3
```

```bash
govc vm.disk.change -vm control-plane-2 -disk.name disk-1000-0 -size 10G

govc vm.disk.change -vm control-plane-3 -disk.name disk-1000-0 -size 10G
```

```bash
govc vm.power -on control-plane-2

govc vm.power -on control-plane-3
```

#### Update Settings for the Worker Nodes

```bash
govc library.deploy <library name>/talos-${TALOS_VERSION} worker-1
govc vm.change \
  -e "guestinfo.talos.config=$(base64 worker.yaml)" \
  -e "disk.enableUUID=1" \
  -vm worker-1

govc library.deploy <library name>/talos-${TALOS_VERSION} worker-2
govc vm.change \
  -e "guestinfo.talos.config=$(base64 worker.yaml)" \
  -e "disk.enableUUID=1" \
  -vm worker-2
```

```bash
govc vm.change \
  -c 4 \
  -m 8192 \
  -vm worker-1

govc vm.change \
  -c 4 \
  -m 8192 \
  -vm worker-2
```

```bash
govc vm.disk.change -vm worker-1 -disk.name disk-1000-0 -size 10G

govc vm.disk.change -vm worker-2 -disk.name disk-1000-0 -size 10G
```

```bash
govc vm.power -on worker-1

govc vm.power -on worker-2
```

#### Bootstrap Cluster

In the vSphere UI, open a console to one of the control plane nodes.
You should see some output stating that etcd should be bootstrapped.
This text should look like:

```bash
"etcd is waiting to join the cluster, if this node is the first node in the cluster, please run `talosctl bootstrap` against one of the following IPs:
```

Take note of the IP mentioned here and issue:

```bash
talosctl --talosconfig talosconfig bootstrap -e <control plane IP> -n <control plane IP>
```

Keep this IP handy for the following steps as well.

#### Retrieve the `kubeconfig`

At this point we can retrieve the admin `kubeconfig` by running:

```bash
talosctl --talosconfig talosconfig config endpoint <control plane IP>
talosctl --talosconfig talosconfig config node <control plane IP>
talosctl --talosconfig talosconfig kubeconfig .
```

#### Configure `talos-vmtoolsd`

The talos-vmtoolsd application was deployed as a daemonset as part of the cluster creation; however, we must now provide a talos credentials file for it to use.

Create a new talosconfig with:

```bash
talosctl -n <control plane IP> config new vmtoolsd-secret.yaml --roles os:admin
```

Create a secret from the talosconfig:

```bash
kubectl -n kube-system create secret generic talos-vmtoolsd-config \
  --from-file=talosconfig=./vmtoolsd-secret.yaml
```

Clean up the generated file from local system:

```bash
rm vmtoolsd-secret.yaml
```

Once configured, you should now see these daemonset pods go into "Running" state and in vCenter, you will now see IPs and info from the Talos nodes present in the UI.
