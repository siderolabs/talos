---
title: "Hetzner"
description: "Creating a cluster via the CLI (hcloud) on Hetzner."
---

## Upload image

Hetzner Cloud does not support uploading custom images.
You can email their support to get a Talos ISO uploaded by following [issues:3599](https://github.com/talos-systems/talos/issues/3599#issuecomment-841172018) or you can prepare image snapshot by yourself.

There are two options to upload your own.

1. Run an instance in rescue mode and replace the system OS with the Talos image
2. Use [Hashicorp packer](https://www.packer.io/docs/builders/hetzner-cloud) to prepare an image

### CLI

Create a new node from existing talos configuration files by issuing the commands shown below.

```bash
hcloud ssh-key create \
  --name ${SSH_KEY_NAME} \
  --public-key-from-file ${PATH_TO_SSH_KEY_FILE}
hcloud server create \
  --image debian-11  \
  --name ${NODE_NAME} \
  --type ${SERVER_TYPE} \
  --ssh-key ${SSH_KEY_NAME} \
  --user-data-from-file ${PATH_TO_TALOS_CONFIG_FILE} \
  --start-after-create=false
hcloud server enable-rescue ${NODE_NAME} \
  --ssh-key ${SSH_KEY_NAME}

hcloud server poweron ${NODE_NAME}
cat << EOF | hcloud server ssh ${NODE_NAME}
cd /tmp
wget -O /tmp/talos.raw.xz https://github.com/talos-systems/talos/releases/download/v${TALOS_VERSION}/hcloud-amd64.raw.xz
xz -d -c /tmp/talos.raw.xz | dd of=/dev/sda && sync
EOF
hcloud server shutdown ${NODE_NAME}
hcloud server poweron ${NODE_NAME}
```

### Create a Load Balancer

Create a load balancer by issuing the commands shown below.
Save the IP/DNS name, as this info will be used in the next step.

```bash
hcloud load-balancer create --name controlplane --network-zone eu-central --type lb11 --label 'type=controlplane'

### Result is like:
# LoadBalancer 484487 created
# IPv4: 49.12.X.X
# IPv6: 2a01:4f8:X:X::1

hcloud load-balancer add-service controlplane \
    --listen-port 6443 --destination-port 6443 --protocol tcp
hcloud load-balancer add-target controlplane \
    --label-selector 'type=controlplane'
```

### Create the Machine Configuration Files

#### Generating Base Configurations

Using the IP/DNS name of the loadbalancer created earlier, generate the base configuration files for the Talos machines by issuing:

```bash
$ talosctl gen config talos-k8s-hcloud-tutorial https://<load balancer IP or DNS>:6443
created controlplane.yaml
created worker.yaml
created talosconfig
```

At this point, you can modify the generated configs to your liking.
Optionally, you can specify `--config-patch` with RFC6902 jsonpatches which will be applied during the config generation.

#### Validate the Configuration Files

Validate any edited machine configs with:

```bash
$ talosctl validate --config controlplane.yaml --mode cloud
controlplane.yaml is valid for cloud mode
$ talosctl validate --config worker.yaml --mode cloud
worker.yaml is valid for cloud mode
```

### Create the Servers

We can now create our servers.
Note that you can find ```IMAGE_ID``` in the snapshot section of the console: ```https://console.hetzner.cloud/projects/$PROJECT_ID/servers/snapshots```.

#### Create the Control Plane Nodes

Create the control plane nodes with:

```bash
export IMAGE_ID=<your-image-id>

hcloud server create --name talos-control-plane-1 \
    --image ${IMAGE_ID} \
    --type cx21 --location hel1 \
    --label 'type=controlplane' \
    --user-data-from-file controlplane.yaml

hcloud server create --name talos-control-plane-2 \
    --image ${IMAGE_ID} \
    --type cx21 --location fsn1 \
    --label 'type=controlplane' \
    --user-data-from-file controlplane.yaml

hcloud server create --name talos-control-plane-3 \
    --image ${IMAGE_ID} \
    --type cx21 --location nbg1 \
    --label 'type=controlplane' \
    --user-data-from-file controlplane.yaml
```

#### Create the Worker Nodes

Create the worker nodes with the following command, repeating (and incrementing the name counter) as many times as desired.

```bash
hcloud server create --name talos-worker-1 \
    --image ${IMAGE_ID} \
    --type cx21 --location hel1 \
    --label 'type=worker' \
    --user-data-from-file worker.yaml
```

### Bootstrap Etcd

To configure `talosctl` we will need the first control plane node's IP.
This can be found by issuing:

```bash
hcloud server list | grep talos-control-plane
```

Set the `endpoints` and `nodes` for your talosconfig with:

```bash
talosctl --talosconfig talosconfig config endpoint <control-plane-1-IP>
talosctl --talosconfig talosconfig config node <control-plane-1-IP>
```

Bootstrap `etcd` on the first control plane node with:

```bash
talosctl --talosconfig talosconfig bootstrap
```

### Retrieve the `kubeconfig`

At this point we can retrieve the admin `kubeconfig` by running:

```bash
talosctl --talosconfig talosconfig kubeconfig .
```
