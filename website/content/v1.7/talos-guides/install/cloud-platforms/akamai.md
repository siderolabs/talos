---
title: "Akamai"
description: "Creating a cluster via the CLI on Akamai Cloud(Linode)."
aliases:
  - ../../../cloud-platforms/akamai
---

## Creating a Talos Linux Cluster on Akamai Connected Cloud via the CLI

This guide will demonstrate how to create a highly-available Kubernetes cluster with one worker using the [Akamai Connected Cloud](https://www.linode.com/) provider.

[Akamai Connected Cloud](https://www.linode.com/) has a very well documented REST API, and an open-source [CLI](https://www.linode.com/docs/products/tools/cli/get-started/) tool to interact with the API which will be used in this guide.
Make sure to follow installation and authentication instructions for the `linode-cli` tool.

### Upload image

Download the Akamai image `akamai-amd64.raw.gz` from the [latest Talos release](https://github.com/siderolabs/talos/releases/latest/).

Upload the image

```bash
export REGION=us-ord

linode-cli image-upload --region ${REGION} --label talos _out/akamai-amd64.raw.gz
```

### Create a Load Balancer

```bash
export REGION=us-ord

linode-cli nodebalancers create --region ${REGION} --no-defaults --label talos
export NODEBALANCER_ID=$(linode-cli nodebalancers list --label talos --format id --text --no-headers)
linode-cli nodebalancers config-create --port 443 --protocol tcp --check connection ${NODEBALANCER_ID}
```

### Create the Machine Configuration Files

Using the IP address (or DNS name, if you have created one) of the loadbalancer, generate the base configuration files for the Talos machines.
Also note that the load balancer forwards port 443 to port 6443 on the associated nodes, so we should use 443 as the port in the config definition:

```bash
export NODEBALANCER_IP=$(linode-cli nodebalancers list --label talos --format ipv4 --text --no-headers)

talosctl gen config talos-kubernetes-akamai https://${NODEBALANCER_IP} --with-examples=false
```

### Create the Linodes

#### Create the Control Plane Nodes

Run the following commands to create three control plane nodes:

```bash
export NODEBALANCER_ID=$(linode-cli nodebalancers list --label talos --format id --text --no-headers)
export NODEBALANCER_CONFIG_ID=$(linode-cli nodebalancers configs-list ${NODEBALANCER_ID} --format id --text --no-headers)
export REGION=us-ord

for id in $(seq 3); do
  linode_label="talos-control-plane-${id}"
  # create linode
  linode-cli linodes create  \
    --no-defaults \
    --root_pass securepass123! \
    --type g6-standard-4 \
    --region ${REGION} \
    --image ${image_id} \
    --label ${linode_label} \
    --private_ip true \
    --tags talos-control-plane \
    --group "talos-control-plane" \
    --metadata.user_data "$(cat ./controlplane.yaml | base64)"

  # change kernel to "direct disk"
  linode_id=$(linode-cli linodes list --label ${linode_label} --format id --text --no-headers)
  confiig_id=$(linode-cli linodes configs-list ${linode_id} --format id --text --no-headers)
  linode-cli linodes config-update ${linode_id} ${confiig_id} --kernel "linode/direct-disk"

  # add machine to nodebalancer
  private_ip=$(linode-cli linodes list --label ${linode_label} --format ipv4 --json | jq -r ".[0].ipv4[1]")
  linode-cli nodebalancers node-create ${NODEBALANCER_ID}  ${NODEBALANCER_CONFIG_ID}  --label ${linode_label} --address ${private_ip}:6443
done
```

#### Create the Worker Nodes

Run the following to create a worker node:

```bash
export IMAGE_ID=$(linode-cli images list --label talos --format id --text --no-headers)
export REGION=us-ord
export LINODE_LABEL="talos-worker-1"
linode-cli linodes create  \
    --no-defaults \
    --root_pass akamaipass123! \
    --type g6-standard-4 \
    --region us-ord \
    --image ${IMAGE_ID} \
    --label ${LINODE_LABEL} \
    --private_ip true \
    --tags talos-worker \
    --group "talos-worker" \
    --metadata.user_data "$(cat ./worker.yaml | base64)"
linode_id=$(linode-cli linodes list --label ${LINODE_LABEL} --format id --text --no-headers)
confiig_id=$(linode-cli linodes configs-list ${linode_id} --format id --text --no-headers)
linode-cli linodes config-update ${linode_id} ${confiig_id} --kernel "linode/direct-disk"
```

### Bootstrap Etcd

Set the `endpoints` and `nodes`:

```bash
export LINODE_LABEL=talos-control-plane-1
export LINODE_IP=$(linode-cli linodes list --label ${LINODE_LABEL} --format ipv4 --json | jq -r ".[0].ipv4[0]")
talosctl --talosconfig talosconfig config endpoint ${LINODE_IP}
talosctl --talosconfig talosconfig config node ${LINODE_IP}
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

We can also watch the cluster bootstrap via:

```bash
talosctl --talosconfig talosconfig health
```
