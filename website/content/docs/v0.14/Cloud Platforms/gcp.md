---
title: "GCP"
description: "Creating a cluster via the CLI on Google Cloud Platform."
---

## Creating a Cluster via the CLI

In this guide, we will create an HA Kubernetes cluster in GCP with 1 worker node.
We will assume an existing [Cloud Storage bucket](https://cloud.google.com/storage/docs/creating-buckets), and some familiarity with Google Cloud.
If you need more information on Google Cloud specifics, please see the [official Google documentation](https://cloud.google.com/docs/).

[jq](https://stedolan.github.io/jq/) and [talosctl](../../introduction/quickstart/#talosctl) also needs to be installed

## Manual Setup

### Environment Setup

We'll make use of the following environment variables throughout the setup.
Edit the variables below with your correct information.

```bash
# Storage account to use
export STORAGE_BUCKET="StorageBucketName"
# Region
export REGION="us-central1"
```

### Create the Image

First, download the Google Cloud image from a Talos [release](https://github.com/talos-systems/talos/releases).
These images are called `gcp-$ARCH.tar.gz`.

#### Upload the Image

Once you have downloaded the image, you can upload it to your storage bucket with:

```bash
gsutil cp /path/to/gcp-amd64.tar.gz gs://$STORAGE_BUCKET
```

#### Register the image

Now that the image is present in our bucket, we'll register it.

```bash
gcloud compute images create talos \
 --source-uri=gs://$STORAGE_BUCKET/gcp-amd64.tar.gz \
 --guest-os-features=VIRTIO_SCSI_MULTIQUEUE
```

### Network Infrastructure

#### Load Balancers and Firewalls

Once the image is prepared, we'll want to work through setting up the network.
Issue the following to create a firewall, load balancer, and their required components.

`130.211.0.0/22` and `35.191.0.0/16` are the GCP [Load Balancer IP ranges](https://cloud.google.com/load-balancing/docs/health-checks#fw-rule)

```bash
# Create Instance Group
gcloud compute instance-groups unmanaged create talos-ig \
  --zone $REGION-b

# Create port for IG
gcloud compute instance-groups set-named-ports talos-ig \
    --named-ports tcp6443:6443 \
    --zone $REGION-b

# Create health check
gcloud compute health-checks create tcp talos-health-check --port 6443

# Create backend
gcloud compute backend-services create talos-be \
    --global \
    --protocol TCP \
    --health-checks talos-health-check \
    --timeout 5m \
    --port-name tcp6443

# Add instance group to backend
gcloud compute backend-services add-backend talos-be \
    --global \
    --instance-group talos-ig \
    --instance-group-zone $REGION-b

# Create tcp proxy
gcloud compute target-tcp-proxies create talos-tcp-proxy \
    --backend-service talos-be \
    --proxy-header NONE

# Create LB IP
gcloud compute addresses create talos-lb-ip --global

# Forward 443 from LB IP to tcp proxy
gcloud compute forwarding-rules create talos-fwd-rule \
    --global \
    --ports 443 \
    --address talos-lb-ip \
    --target-tcp-proxy talos-tcp-proxy

# Create firewall rule for health checks
gcloud compute firewall-rules create talos-controlplane-firewall \
     --source-ranges 130.211.0.0/22,35.191.0.0/16 \
     --target-tags talos-controlplane \
     --allow tcp:6443

# Create firewall rule to allow talosctl access
gcloud compute firewall-rules create talos-controlplane-talosctl \
  --source-ranges 0.0.0.0/0 \
  --target-tags talos-controlplane \
  --allow tcp:50000
```

### Cluster Configuration

With our networking bits setup, we'll fetch the IP for our load balancer and create our configuration files.

```bash
LB_PUBLIC_IP=$(gcloud compute forwarding-rules describe talos-fwd-rule \
               --global \
               --format json \
               | jq -r .IPAddress)

talosctl gen config talos-k8s-gcp-tutorial https://${LB_PUBLIC_IP}:443
```

Additionally, you can specify `--config-patch` with RFC6902 jsonpatch which will be applied during the config generation.

### Compute Creation

We are now ready to create our GCP nodes.

```bash
# Create the control plane nodes.
for i in $( seq 1 3 ); do
  gcloud compute instances create talos-controlplane-$i \
    --image talos \
    --zone $REGION-b \
    --tags talos-controlplane \
    --boot-disk-size 20GB \
    --metadata-from-file=user-data=./controlplane.yaml
done

# Add control plane nodes to instance group
for i in $( seq 0 1 3 ); do
  gcloud compute instance-groups unmanaged add-instances talos-ig \
      --zone $REGION-b \
      --instances talos-controlplane-$i
done

# Create worker
gcloud compute instances create talos-worker-0 \
  --image talos \
  --zone $REGION-b \
  --boot-disk-size 20GB \
  --metadata-from-file=user-data=./worker.yaml
```

### Bootstrap Etcd

You should now be able to interact with your cluster with `talosctl`.
We will need to discover the public IP for our first control plane node first.

```bash
CONTROL_PLANE_0_IP=$(gcloud compute instances describe talos-controlplane-0 \
                     --zone $REGION-b \
                     --format json \
                     | jq -r '.networkInterfaces[0].accessConfigs[0].natIP')
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

### Cleanup

```bash
# cleanup VM's
gcloud compute instances delete \
  talos-worker-0 \
  talos-controlplane-0 \
  talos-controlplane-1 \
  talos-controlplane-2

# cleanup firewall rules
gcloud compute firewall-rules delete \
  talos-controlplane-talosctl \
  talos-controlplane-firewall

# cleanup forwarding rules
gcloud compute forwarding-rules delete \
  talos-fwd-rule

# cleanup addresses
gcloud compute addresses delete \
  talos-lb-ip

# cleanup proxies
gcloud compute target-tcp-proxies delete \
  talos-tcp-proxy

# cleanup backend services
gcloud compute backend-services delete \
  talos-be

# cleanup health checks
gcloud compute health-checks delete \
  talos-health-check

# cleanup unmanaged instance groups
gcloud compute instance-groups unmanaged delete \
  talos-ig

# cleanup Talos image
gcloud compute images delete \
  talos
```

## Using GCP Deployment manager

Using GCP deployment manager automatically creates a Google Storage bucket and uploads the Talos image to it.

By default this setup creates a three node control plane and a single worker in `us-west2-c`

First we need to create a folder to store our deployment manifests and perform all subsequent operations from that folder.

```bash
mkdir -p talos-gcp-deployment
cd talos-gcp-deployment
```

### Getting the deployment manifests

We need to download two deployment manifests for the deployment from the Talos github repository.

```bash
curl -fsSLO "https://raw.githubusercontent.com/talos-systems/talos/master/website/content/docs/v0.14/Cloud%20Platforms/gcp/config.yaml"
curl -fsSLO "https://raw.githubusercontent.com/talos-systems/talos/master/website/content/docs/v0.14/Cloud%20Platforms/gcp/talos-ha.yaml"
```

### Updating the config

Now we need to update the local `config.yaml` file with any required changes such as changing the default zone, Talos version, machine sizes, nodes count etc.

An example `config.yaml` file is shown below:

```yaml
imports:
  - path: talos-ha.jinja

resources:
  - name: talos-ha
    type: talos-ha.jinja
    properties:
      zone: us-west2-c
      talosVersion: v0.13.2
      controlPlaneNodeCount: 5
      controlPlaneNodeType: n1-standard-1
      workerNodeCount: 3
      workerNodeType: n1-standard-1
outputs:
  - name: bucketName
    value: $(ref.talos-ha.bucketName)
  - name: loadbalancerIP
    value: $(ref.talos-ha.loadbalancerIP)
  - name: controlPlaneNodeIPs
    value: $(ref.talos-ha.controlPlaneNodeIPs)
  - name: workerNodeIPs
    value: $(ref.talos-ha.workerNodeIPs)
```

### Creating the deployment

Now we are ready to create the deployment.
Confirm with `y` for any prompts.
Run the following command to create the deployment:

```bash
# a unique name for the deployment, resources are prefixed with the deployment name
export DEPLOYMENT_NAME="talos-gcp-ha"
gcloud deployment-manager deployments create "${DEPLOYMENT_NAME}" --config config.yaml
```

### Retrieving the outputs

First we need to get the deployment outputs.

```bash
# first get the outputs
OUTPUTS=$(gcloud deployment-manager deployments describe "${DEPLOYMENT_NAME}" --format json | jq '.outputs[]')

BUCKET_NAME=$(jq -r '. | select(.name == "bucketName").finalValue' <<< "${OUTPUTS}")
LOADBALANCER_IP=$(jq -r '. | select(.name == "loadbalancerIP").finalValue' <<< "${OUTPUTS}")
CONTROLPLANE0_IP=$(jq -r '. | select(.name == "controlPlaneNodeIPs[0]").finalValue' <<< "${OUTPUTS}")
CONTROLPLANE_IPS=$(jq -r '. | select(.name | contains("controlPlaneNodeIPs")).finalValue' <<< "${OUTPUTS}")
WORKER_IPS=$(jq -r '. | select(.name | contains("workerNodeIPs")).finalValue' <<< "${OUTPUTS}")
```

### Generating talos config

We need to generate `talosconfig`, controlplane and worker configs

```bash
# use a directory to store the configs
mkdir -p generated
talosctl gen config \
  "${DEPLOYMENT_NAME}" \
  "https://${LOADBALANCER_IP}:443" \
  --output-dir generated/
```

### Bootstrap nodes

Now we'r ready to bootstrap the nodes.

```bash
# bootstrap controlplane nodes
for CONTROLPLANE_IP in "${CONTROLPLANE_IPS}"; do
  talosctl apply-config \
    --insecure \
    --nodes "${CONTROLPLANE_IP}" \
    --endpoints "${CONTROLPLANE_IP}" \
    --file generated/controlplane.yaml
done

# bootstrap worker nodes
for WORKER_IP in "${WORKER_IPS}"; do
  talosctl apply-config \
    --insecure \
    --nodes "${WORKER_IP}" \
    --endpoints "${WORKER_IP}" \
    --file generated/worker.yaml
done
```

### Bootstrap etcd

```bash
talosctl \
  --talosconfig generated/talosconfig \
  --nodes "${CONTROLPLANE0_IP}" \
  --endpoints "${CONTROLPLANE0_IP}" \
  bootstrap
```

### Retrieve `kubeconfig`

At this point we can retrieve the admin `kubeconfig` by running:

```bash
talosctl \
  --talosconfig generated/talosconfig \
  kubeconfig generated
```

### Check cluster status

```bash
kubectl \
  --kubeconfig generated/kubeconfig \
  get nodes
```

### Cleanup deployment

```bash
gsutil rm \
  "gs://${DEPLOYMENT_NAME}-talos-assets/gcp-amd64.tar.gz"
gcloud deployment-manager deployments delete "${DEPLOYMENT_NAME}"
```
