---
title: "GCP"
description: "Creating a cluster via the CLI on Google Cloud Platform."
---

## Creating a Cluster via the CLI

In this guide, we will create an HA Kubernetes cluster in GCP with 1 worker node.
We will assume an existing [Cloud Storage bucket](https://cloud.google.com/storage/docs/creating-buckets), and some familiarity with Google Cloud.
If you need more information on Google Cloud specifics, please see the [official Google documentation](https://cloud.google.com/docs/).

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

### Compute Creation

We are now ready to create our GCP nodes.

```bash
# Create control plane 0
gcloud compute instances create talos-controlplane-0 \
  --image talos \
  --zone $REGION-b \
  --tags talos-controlplane \
  --boot-disk-size 20GB \
  --metadata-from-file=user-data=./init.yaml

# Create control plane 1/2
for i in $( seq 1 2 ); do
  gcloud compute instances create talos-controlplane-$i \
    --image talos \
    --zone $REGION-b \
    --tags talos-controlplane \
    --boot-disk-size 20GB \
    --metadata-from-file=user-data=./controlplane.yaml
done

# Add control plane nodes to instance group
for i in $( seq 0 1 2 ); do
  gcloud compute instance-groups unmanaged add-instances talos-ig \
      --zone $REGION-b \
      --instances talos-controlplane-$i
done

# Create worker
gcloud compute instances create talos-worker-0 \
  --image talos \
  --zone $REGION-b \
  --boot-disk-size 20GB \
  --metadata-from-file=user-data=./join.yaml
```

### Retrieve the `kubeconfig`

You should now be able to interact with your cluster with `talosctl`.
We will need to discover the public IP for our first control plane node first.

```bash
CONTROL_PLANE_0_IP=$(gcloud compute instances describe talos-controlplane-0 \
                     --zone $REGION-b \
                     --format json \
                     | jq -r '.networkInterfaces[0].accessConfigs[0].natIP')

talosctl --talosconfig ./talosconfig config endpoint $CONTROL_PLANE_0_IP
talosctl --talosconfig ./talosconfig config node $CONTROL_PLANE_0_IP
talosctl --talosconfig ./talosconfig kubeconfig .
kubectl --kubeconfig ./kubeconfig get nodes
```
