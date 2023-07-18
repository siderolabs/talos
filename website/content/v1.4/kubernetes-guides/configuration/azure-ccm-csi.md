---
title: "Azure Cloud Controller Manager and CSI driver for storage"
description: "Guide on how to install the Azure Cloud Controller Manager and Container Storage Interface driver in Kubernetes"
aliases:
  - ../../guides/azure-ccm-csi
---

This is a guide for installing the Azure Cloud Provider and Azure CSI.

The `cloud-provider-azure` module is used for interacting with Azure cloud resources through Kubernetes and this guide will also walk through setting up the CSI storage component to set up a StorageClass for workloads to use on the cluster.

The steps in this guide could be used for any Kubernetes cluster with the addition of the patch applied to a Talos cluster.

## Pre -requisites

This guide assumes a Talos cluster is already available and the user has an Azure account set up.

- Instructions for installing Talos can be found in [Talos Docs (Installation)](https://www.talos.dev/v1.4/talos-guides/install/).
- Instructions for installing **talosctl** and **kubectl** can be found in [Talos Docs (Quickstart)](https://www.talos.dev/v1.4/introduction/quickstart/#talosctl).

The applications in this guide will be installed using Helm.

- Instructions for install **helm** can be found in the [Helm Documentation](https://helm.sh/docs/intro/install/).

## Apply patch to Talos

There is an option in the Talos machine config to tell the control-plane to use an external controller manager.

This will apply an uninitialized label to a node when it registers to make it impossible to schedule workloads until the CCM has discovered that there is a new node in the cluster.

This configuration is referenced in [Talos Docs (Machine Controller Manager)](https://www.talos.dev/v1.4/reference/configuration/#machinecontrollermanagerconfig).

To apply this to the Talos cluster, create a patch file or edit the machineconfig.

To create a patch file:

```bash
vim patch.yaml
```

Add the following to the **patch.yaml** file:

```yaml
cluster:
  controllerManager:
    extraArgs:
      cloud-provider: external
```

Then, apply the patch with:

```bash
talosctl machineconfig patch patch.yaml
```

More information on applying machinconfig patches can be found at [Talos Docs (Machine Config Patch)](https://www.talos.dev/v1.4/reference/cli/#talosctl-machineconfig-patch).

## Azure Configuration File

The Azure Cloud Controller Manager requires a configuration file to gain permissions on the cluster which will require gathering a few values from the Azure Portal and creating an app registration to give the CCM the permissions it needs.

This file is usually placed on the filesystem, but this guide will cover creating a secret to store this configuration instead.

### App Registration

The App Registration is what we will use to authenticate to Azure for uploading blobs and creating resources.

For more information not in this guide or to see changes made to the app registration process, Azure's documentation can be found here:

- [Azure Documentation (App Registration)](https://learn.microsoft.com/en-us/azure/active-directory/develop/quickstart-register-app)

To create an App Registration in Azure:

- Search for and Select **Azure Active Directory**.
- Select **App registrations**, then select **New registration**.
- Name the application, for example "example-app".
- Select a supported account type, which determines who can use the application.
- Under **Redirect URI**, select **Web** for the type of application and enter the URI where the access token is sent to.
- Select **Register**.

Collect the following values from Azure, as they will be needed for the Azure CCM configuration file.

- **Tenant ID**
- **Subscription ID**
- **Client ID**
- **Client Secret**

#### Add permissions for App Registration

The App registration only needs permissions to the Compute Gallery and the Storage Account.

- Select the **Resource Group** the Talos cluster is deployed in
- Select **Access control (IAM)**
- Select **Add** role assignment
- Select the role needed for the account.

> **NOTE:** This will vary depending on what the CCM is being used for, but **Virtual Machine Contributor** is enough for the purposes if this installation guide.

### Collect additional information

In the Azure Portal, collected the following values to be used in the configuration file, **specific to the cluster the CCM is being installed on**:

- **Resource Group**
- **Location**
- **Virtual Network name**
- **Route Table name**

### Create the configuration file

Create a configuration file named **azure.cfg**

```shell
vim cloud.conf
```

Add the following to the **azure.cfg** file, but **replace the values with the values gathered at the beginning of this guide**.

```shell
{
  "cloud":"AzurePublicCloud",
  "tenantId": "${TENANT_ID}$",
  "subscriptionId": "${SUBSCRIPTION_ID}$",
  "aadClientId": "${CLIENT_ID}$",
  "aadClientSecret": "${CLIENT_SECRET}$",
  "resourceGroup": "${RESOURCE_GROUP}$",
  "location": "${LOCATION}",
  "loadBalancerSku": "standard",
  "securityGroupName": "${SECURITY_GROUP_NAME}",
  "vnetName": "${VIRTUAL_NETWORK_NAME}",
  "routeTableName": "${ROUTE_TABLE_NAME}"
}

```

Additional configurations can be found in the CCM docs here: [Cloud Provider Azure configs](https://github.com/kubernetes-sigs/cloud-provider-azure/blob/documentation/content/en/install/configs.md).

A secret can be created in Kubernetes using the following command:

> **NOTE**: This secret is created in the **kube-system** namespace because that is where the CCM and CSI components will be installed.

```bash
kubectl create secret generic azure-cloud-provider --from-file=cloud-config=./cloud.conf -n kube-system
```

## Install the Azure Cloud Controller Manager

Find the version compatible with the Kubernetes version installed with the Talos cluster https://github.com/kubernetes-sigs/cloud-provider-azure/blob/master/README.md

To use the latest release add the following helm repo:

> **NOTE**: To use a release specific to the Kubernetes version other than the latest version, replace **master** with the branch name specified in the version matrix above.

```bash
helm repo add cloud-provider-azure https://raw.githubusercontent.com/kubernetes-sigs/cloud-provider-azure/master/helm/repo
```

Update helm repositories:

```bash
helm repo update
```

Install the helm chart for `cloud-provider-azure`:

```bash
helm install azure-ccm cloud-provider-azure/cloud-provider-azure \
--set cloud-provider-azure.infra.clusterName="christian-tf" \
--set cloud-provider-azure.cloudControllerManager.cloudConfig='' \
--set cloud-provider-azure.cloudControllerManager.cloudConfigSecretName="azure-cloud-provider" \
--set cloud-provider-azure.cloudControllerManager.enableDynamicReloading="true" \
--set cloud-provider-azure.cloudControllerManager.configureCloudRoutes="true" \
--set cloud-provider-azure.cloudControllerManager.allocateNodeCidrs="true" \
--set cloud-provider-azure.cloudControllerManager.imageRepository="mcr.microsoft.com/oss/kubernetes"
```

## Install the Azure CSI Driver

dependencies:

- name: azuredisk-csi-driver
  repository: https://raw.githubusercontent.com/kubernetes-sigs/azuredisk-csi-driver/master/charts
  version: v1.27.1

Add the Azure CSI helm repo:

```bash
helm repo add azuredisk-csi-driver https://raw.githubusercontent.com/kubernetes-sigs/azuredisk-csi-driver/master/charts
```

Update helm repositories

```bash
helm repo update
```

```bash
helm install azure-csi azuredisk-csi-driver/azuredisk-csi-driver -n kube-system
```

Lastly, create a file for a StorageClass to use the CSI:

```bash
vim azure-ssd-lrs.yaml
```

Add the following contents to the file:

```yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: azuredisk-standard-ssd-lrs
provisioner: disk.csi.azure.com
parameters:
  skuName: StandardSSD_LRS
reclaimPolicy: Delete
volumeBindingMode: Immediate
allowVolumeExpansion: true
```

Create the storageclass:

```bash
kubectl apply -f azure-ssd-lrs.yaml
```

Persistent Volume Claims can now be created for workloads in the cluster using the StorageClass created.
