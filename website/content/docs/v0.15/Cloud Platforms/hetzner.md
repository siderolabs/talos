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

### Rescue mode

Create a new Server in the Hetzner console.
Enable the Hetzner Rescue System for this server and reboot.
Upon a reboot, the server will boot a special minimal Linux distribution designed for repair and reinstall.
Once running, login to the server using ```ssh``` to prepare the system disk by doing the following:

```bash
# Check that you in Rescue mode
df

### Result is like:
# udev                   987432         0    987432   0% /dev
# 213.133.99.101:/nfs 308577696 247015616  45817536  85% /root/.oldroot/nfs
# overlay                995672      8340    987332   1% /
# tmpfs                  995672         0    995672   0% /dev/shm
# tmpfs                  398272       572    397700   1% /run
# tmpfs                    5120         0      5120   0% /run/lock
# tmpfs                  199132         0    199132   0% /run/user/0

# Download the Talos image
cd /tmp
wget -O /tmp/talos.raw.xz https://github.com/talos-systems/talos/releases/download/v0.13.0/hcloud-amd64.raw.xz
# Replace system
xz -d -c /tmp/talos.raw.xz | dd of=/dev/sda && sync
# shutdown the instance
shutdown -h now
```

To make sure disk content is consistent, it is recommended to shut the server down before taking an image (snapshot).
Once shutdown, simply create an image (snapshot) from the console.
You can now use this snapshot to run Talos on the cloud.

### Packer

Install [packer](https://learn.hashicorp.com/tutorials/packer/get-started-install-cli) to the local machine.

Create a config file for packer to use:

```hcl
# hcloud.pkr.hcl

packer {
  required_plugins {
    hcloud = {
      version = ">= 1.0.0"
      source  = "github.com/hashicorp/hcloud"
    }
  }
}

variable "talos_version" {
  type    = string
  default = "v0.13.0"
}

locals {
  image = "https://github.com/talos-systems/talos/releases/download/${var.talos_version}/hcloud-amd64.raw.xz"
}

source "hcloud" "talos" {
  rescue       = "linux64"
  image        = "debian-11"
  location     = "hel1"
  server_type  = "cx11"
  ssh_username = "root"

  snapshot_name = "talos system disk"
  snapshot_labels = {
    type    = "infra",
    os      = "talos",
    version = "${var.talos_version}",
  }
}

build {
  sources = ["source.hcloud.talos"]

  provisioner "shell" {
    inline = [
      "apt-get install -y wget",
      "wget -O /tmp/talos.raw.xz ${local.image}",
      "xz -d -c /tmp/talos.raw.xz | dd of=/dev/sda && sync",
    ]
  }
}
```

Create a new image by issuing the commands shown below.
Note that to create a new API token for your Project, switch into the Hetzner Cloud Console choose a Project, go to Access → Security, and create a new token.

```bash
# First you need set API Token
export HCLOUD_TOKEN=${TOKEN}

# Upload image
packer init .
packer build .
# Save the image ID
export IMAGE_ID=<image-id-in-packer-output>
```

After doing this, you can find the snapshot in the console interface.

## Creating a Cluster via the CLI

This section assumes you have the [hcloud console utility](https://community.hetzner.com/tutorials/howto-hcloud-cli) on your local machine.

```bash
# Set hcloud context and api key
hcloud context create talos-tutorial
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
