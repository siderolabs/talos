---
title: "Azure"
date: 2019-8-16
draft: false
menu:
  docs:
    parent: 'guides'
---

## Image Creation

For each [Talos release](https://github.com/talos-systems/talos/releases),
we provide an Azure compatible vhd (`azure.tar.gz`).  If you want to build
the image locally, you can do so by running:

```bash
make installer
make image-azure
```

This will produce `build/azure.tar.gz`.

## Environment Setup

Before proceeding, you'll want to make sure you have `osctl` available and the
azure cli `az` installed and configured. `osctl` is published on each release
and is available on our releases page [Talos release](https://github.com/talos-systems/talos/releases).
If you want to build it locally, you can do so by running:

```bash
make osctl-[linux|darwin]
cp build/osctl-[linux|darwin]-amd64 /usr/local/bin
```

We'll also make use of the following environment variables throughout the setup:

```bash
# Storage account to use, default to 'mytalosvhd'
STORAGE_ACCOUNT=${STORAGE_ACCOUNT:-mytalosvhd}

# Resource group name, default to 'talos'
GROUP=${GROUP:-talos}

# Location, default to 'westus2'
LOCATION=${LOCATION:-westus2}
```

## Upload Image

After downloading or creating the image locally, we'll want to upload it to
Azure and create an image.

```bash
# Create resource group
az group create -l $LOCATION -n $GROUP

# Create storage account
az storage account create -g $GROUP -n $STORAGE_ACCOUNT

# Get storage account connection string
CONNECTION=$(az storage account show-connection-string -n $STORAGE_ACCOUNT -g $GROUP -o tsv)

# Create a container in the storage account
az storage container create -n talos --connection-string $CONNECTION

# Upload the vhd
az storage blob upload --connection-string $CONNECTION --container-name talos -f build/talos-azure.vhd -n disk.vhd

# Create an image based on the vhd
az image create --name talos --source https://$STORAGE_ACCOUNT.blob.core.windows.net/talos/disk.vhd --os-type linux -g $GROUP
```

## Network Infrastructure

Once the resource group is created and image uploaded, we'll want to work
through the network security rules.

```bash
# Create network security group
az network nsg create -g $GROUP -n talos

# Client -> Proxyd
az network nsg rule create -g $GROUP --nsg-name talos -n proxyd --priority 1000 --destination-port-ranges 443  --direction inbound

# Client -> OSD
az network nsg rule create -g $GROUP --nsg-name talos -n osd --priority 1001 --destination-port-ranges 50000 --direction inbound

# Trustd
az network nsg rule create -g $GROUP --nsg-name talos -n trustd --priority 1002 --destination-port-ranges 50001 --direction inbound

# etcd
az network nsg rule create -g $GROUP --nsg-name talos -n etcd --priority 1003 --destination-port-ranges 2379-2380 --direction inbound

# Proxyd -> Kubernetes API Server
az network nsg rule create -g $GROUP --nsg-name talos -n kube --priority 1004 --destination-port-ranges 6443 --direction inbound
```

## Cluster Configuration

After getting the network security group set up, we'll need to allocate public
IPs for our master nodes. The example below assumes a HA control plane of 3 nodes.
You can adjust this for your needs.

```bash
# Reserve public IPs
az network public-ip create -g $GROUP --name talos-master-1 --allocation-method static
az network public-ip create -g $GROUP --name talos-master-2 --allocation-method static
az network public-ip create -g $GROUP --name talos-master-3 --allocation-method static

# Gather public IPs into a comma separated string
MASTERIPS=$(az network public-ip list -g $GROUP -o tsv --query [].ipAddress | paste -sd,)

# Generate a default Talos config for this cluster
# # This should generate master-{1,2,3}.yaml, worker.yaml, and talosconfig in your PWD
./osctl config generate cluster.local $MASTERIPS
```

## Compute Creation

```bash
# Create master nodes
# # `--admin-username` and `--generate-ssh-keys` are required by the az cli,
# # but are not actually used by talos
# # `--os-disk-size-gb` is the backing disk for Kubernetes and any workload containers
# # `--boot-diagnostics-storage` is to enable console output which may be necessary
# # for troubleshooting
az vm create \
  --name talos1 \
  --image talos \
  --custom-data ./master-1.yaml \
  --public-ip-address talos-master-1 \
  -g $GROUP \
  --admin-username talos \
  --generate-ssh-keys \
  --verbose \
  --boot-diagnostics-storage $STORAGE_ACCOUNT \
  --nsg talos \
  --os-disk-size-gb 64 \
  --no-wait
az vm create --name talos2 --image talos --custom-data ./master-2.yaml  --public-ip-address talos-master-2 -g $GROUP --admin-username talos --generate-ssh-keys --verbose --boot-diagnostics-storage $STORAGE_ACCOUNT --nsg talos --os-disk-size-gb 64 --no-wait
az vm create --name talos3 --image talos --custom-data ./master-3.yaml  --public-ip-address talos-master-3 -g $GROUP --admin-username talos --generate-ssh-keys --verbose --boot-diagnostics-storage $STORAGE_ACCOUNT --nsg talos --os-disk-size-gb 64 --no-wait

## Create worker nodes, reuse as needed
az vm create --name talos4 --image talos --custom-data ./worker.yaml -g $GROUP --admin-username talos --generate-ssh-keys --verbose --boot-diagnostics-storage $STORAGE_ACCOUNT --nsg talos --os-disk-size-gb 64 --no-wait
```

## Enjoy your cluster

You should now be able to interact with your cluster with `osctl`:

```bash
osctl --talosconfig ./talosconfig kubeconfig > kubeconfig
kubectl --kubeconfig ./kubeconfig get nodes
```

You will need to apply a PSP and CNI configuration. More details can be found
in the [getting started](/docs/guides/getting_started) guide.
