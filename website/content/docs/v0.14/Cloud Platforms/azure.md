---
title: "Azure"
description: "Creating a cluster via the CLI on Azure."
---

## Creating a Cluster via the CLI

In this guide we will create an HA Kubernetes cluster with 1 worker node.
We assume existing [Blob Storage](https://docs.microsoft.com/en-us/azure/storage/blobs/), and some familiarity with Azure.
If you need more information on Azure specifics, please see the [official Azure documentation](https://docs.microsoft.com/en-us/azure/).

### Environment Setup

We'll make use of the following environment variables throughout the setup.
Edit the variables below with your correct information.

```bash
# Storage account to use
export STORAGE_ACCOUNT="StorageAccountName"

# Storage container to upload to
export STORAGE_CONTAINER="StorageContainerName"

# Resource group name
export GROUP="ResourceGroupName"

# Location
export LOCATION="centralus"

# Get storage account connection string based on info above
export CONNECTION=$(az storage account show-connection-string \
                    -n $STORAGE_ACCOUNT \
                    -g $GROUP \
                    -o tsv)
```

### Create the Image

First, download the Azure image from a [Talos release](https://github.com/talos-systems/talos/releases).
Once downloaded, untar with `tar -xvf /path/to/azure-amd64.tar.gz`

#### Upload the VHD

Once you have pulled down the image, you can upload it to blob storage with:

```bash
az storage blob upload \
  --connection-string $CONNECTION \
  --container-name $STORAGE_CONTAINER \
  -f /path/to/extracted/talos-azure.vhd \
  -n talos-azure.vhd
```

#### Register the Image

Now that the image is present in our blob storage, we'll register it.

```bash
az image create \
  --name talos \
  --source https://$STORAGE_ACCOUNT.blob.core.windows.net/$STORAGE_CONTAINER/talos-azure.vhd \
  --os-type linux \
  -g $GROUP
```

### Network Infrastructure

#### Virtual Networks and Security Groups

Once the image is prepared, we'll want to work through setting up the network.
Issue the following to create a network security group and add rules to it.

```bash
# Create vnet
az network vnet create \
  --resource-group $GROUP \
  --location $LOCATION \
  --name talos-vnet \
  --subnet-name talos-subnet

# Create network security group
az network nsg create -g $GROUP -n talos-sg

# Client -> apid
az network nsg rule create \
  -g $GROUP \
  --nsg-name talos-sg \
  -n apid \
  --priority 1001 \
  --destination-port-ranges 50000 \
  --direction inbound

# Trustd
az network nsg rule create \
  -g $GROUP \
  --nsg-name talos-sg \
  -n trustd \
  --priority 1002 \
  --destination-port-ranges 50001 \
  --direction inbound

# etcd
az network nsg rule create \
  -g $GROUP \
  --nsg-name talos-sg \
  -n etcd \
  --priority 1003 \
  --destination-port-ranges 2379-2380 \
  --direction inbound

# Kubernetes API Server
az network nsg rule create \
  -g $GROUP \
  --nsg-name talos-sg \
  -n kube \
  --priority 1004 \
  --destination-port-ranges 6443 \
  --direction inbound
```

#### Load Balancer

We will create a public ip, load balancer, and a health check that we will use for our control plane.

```bash
# Create public ip
az network public-ip create \
  --resource-group $GROUP \
  --name talos-public-ip \
  --allocation-method static

# Create lb
az network lb create \
  --resource-group $GROUP \
  --name talos-lb \
  --public-ip-address talos-public-ip \
  --frontend-ip-name talos-fe \
  --backend-pool-name talos-be-pool

# Create health check
az network lb probe create \
  --resource-group $GROUP \
  --lb-name talos-lb \
  --name talos-lb-health \
  --protocol tcp \
  --port 6443

# Create lb rule for 6443
az network lb rule create \
  --resource-group $GROUP \
  --lb-name talos-lb \
  --name talos-6443 \
  --protocol tcp \
  --frontend-ip-name talos-fe \
  --frontend-port 6443 \
  --backend-pool-name talos-be-pool \
  --backend-port 6443 \
  --probe-name talos-lb-health
```

#### Network Interfaces

In Azure, we have to pre-create the NICs for our control plane so that they can be associated with our load balancer.

```bash
for i in $( seq 0 1 2 ); do
  # Create public IP for each nic
  az network public-ip create \
    --resource-group $GROUP \
    --name talos-controlplane-public-ip-$i \
    --allocation-method static


  # Create nic
  az network nic create \
    --resource-group $GROUP \
    --name talos-controlplane-nic-$i \
    --vnet-name talos-vnet \
    --subnet talos-subnet \
    --network-security-group talos-sg \
    --public-ip-address talos-controlplane-public-ip-$i\
    --lb-name talos-lb \
    --lb-address-pools talos-be-pool
done

# NOTES:
# Talos can detect PublicIPs automatically if PublicIP SKU is Basic.
# Use `--sku Basic` to set SKU to Basic.
```

### Cluster Configuration

With our networking bits setup, we'll fetch the IP for our load balancer and create our configuration files.

```bash
LB_PUBLIC_IP=$(az network public-ip show \
              --resource-group $GROUP \
              --name talos-public-ip \
              --query [ipAddress] \
              --output tsv)

talosctl gen config talos-k8s-azure-tutorial https://${LB_PUBLIC_IP}:6443
```

### Compute Creation

We are now ready to create our azure nodes.
Azure allows you to pass Talos machine configuration to the virtual machine at bootstrap time via
`user-data` or `custom-data` methods.

Talos supports only `custom-data` method, machine configuration is available to the VM only on the first boot.

```bash
# Create availability set
az vm availability-set create \
  --name talos-controlplane-av-set \
  -g $GROUP

# Create the controlplane nodes
for i in $( seq 0 1 2 ); do
  az vm create \
    --name talos-controlplane-$i \
    --image talos \
    --custom-data ./controlplane.yaml \
    -g $GROUP \
    --admin-username talos \
    --generate-ssh-keys \
    --verbose \
    --boot-diagnostics-storage $STORAGE_ACCOUNT \
    --os-disk-size-gb 20 \
    --nics talos-controlplane-nic-$i \
    --availability-set talos-controlplane-av-set \
    --no-wait
done

# Create worker node
  az vm create \
    --name talos-worker-0 \
    --image talos \
    --vnet-name talos-vnet \
    --subnet talos-subnet \
    --custom-data ./worker.yaml \
    -g $GROUP \
    --admin-username talos \
    --generate-ssh-keys \
    --verbose \
    --boot-diagnostics-storage $STORAGE_ACCOUNT \
    --nsg talos-sg \
    --os-disk-size-gb 20 \
    --no-wait

# NOTES:
# `--admin-username` and `--generate-ssh-keys` are required by the az cli,
# but are not actually used by talos
# `--os-disk-size-gb` is the backing disk for Kubernetes and any workload containers
# `--boot-diagnostics-storage` is to enable console output which may be necessary
# for troubleshooting
```

### Bootstrap Etcd

You should now be able to interact with your cluster with `talosctl`.
We will need to discover the public IP for our first control plane node first.

```bash
CONTROL_PLANE_0_IP=$(az network public-ip show \
                    --resource-group $GROUP \
                    --name talos-controlplane-public-ip-0 \
                    --query [ipAddress] \
                    --output tsv)
```

Set the `endpoints` and `nodes`:

```bash
talosctl --talosconfig talosconfig config endpoint $CONTROL_PLANE_0_IP
talosctl --talosconfig talosconfig config node $CONTROL_PLANE_0_IP
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
