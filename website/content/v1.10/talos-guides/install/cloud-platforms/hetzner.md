---
title: "Hetzner"
description: "Creating a cluster via the CLI (hcloud) on Hetzner."
aliases:
  - ../../../cloud-platforms/hetzner
---

## Upload image

**NOTE:** Hetzner Cloud provides Talos as Public ISO with the schematic id `ce4c980550dd2ab1b17bbf2b08801c7eb59418eafe8f279833297925d67c7515` (Hetzner + qemu-guest-agent) since 2025-04-23.
Minor updates of the ISO will be provided by Hetzner Cloud on a best effort.

If you need an ISO with a different schematic id, please email the support team to get a Talos ISO uploaded by following [issues:3599](https://github.com/siderolabs/talos/issues/3599#issuecomment-841172018) or you can prepare image snapshot by yourself.

There are three options to upload your own.

1. Run an instance in rescue mode and replace the system OS with the Talos image
2. Use [Hashicorp packer](https://www.packer.io/docs/builders/hetzner-cloud) to prepare an image
3. Use special utility [hcloud-upload-image](https://github.com/apricote/hcloud-upload-image/)

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
wget -O /tmp/talos.raw.xz https://factory.talos.dev/image/376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba/{{< release >}}/hcloud-amd64.raw.xz
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
      source  = "github.com/hetznercloud/hcloud"
      version = "~> 1"
    }
  }
}

variable "talos_version" {
  type    = string
  default = "{{< release >}}"
}

variable "arch" {
  type    = string
  default = "amd64"
}

variable "server_type" {
  type    = string
  default = "cx22"
}

variable "server_location" {
  type    = string
  default = "hel1"
}

locals {
  image = "https://factory.talos.dev/image/376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba/${var.talos_version}/hcloud-${var.arch}.raw.xz"
}

source "hcloud" "talos" {
  rescue       = "linux64"
  image        = "debian-11"
  location     = "${var.server_location}"
  server_type  = "${var.server_type}"
  ssh_username = "root"

  snapshot_name   = "talos system disk - ${var.arch} - ${var.talos_version}"
  snapshot_labels = {
    type    = "infra",
    os      = "talos",
    version = "${var.talos_version}",
    arch    = "${var.arch}",
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

Additionally you could create a file containing

```hcl
arch            = "arm64"
server_type     = "cax11"
server_location = "fsn1"
``````

and build the snapshot for arm64.

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

### hcloud-upload-image

Install process described [here](https://github.com/apricote/hcloud-upload-image/?tab=readme-ov-file#getting-started) (you can download binary or build from source, it is also possible to use Docker).

For process simplification you can use this `bash` script:

```bash
#!/usr/bin/env bash
export TALOS_IMAGE_VERSION={{< release >}} # You can change to the current version
export TALOS_IMAGE_ARCH=amd64 # You can change to arm architecture
export HCLOUD_SERVER_ARCH=x86 # HCloud server architecture can be x86 or arm
wget https://factory.talos.dev/image/376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba/${TALOS_IMAGE_VERSION}/hcloud-${TALOS_IMAGE_ARCH}.raw.xz
hcloud-upload-image upload \
      --image-path *.xz \
      --architecture $HCLOUD_SERVER_ARCH \
      --compression xz
```

After these actions, you can find the snapshot in the console interface.

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
$ talosctl gen config talos-k8s-hcloud-tutorial https://<load balancer IP or DNS>:6443 \
    --with-examples=false --with-docs=false
created controlplane.yaml
created worker.yaml
created talosconfig
```

Generating the config without examples and docs is necessary because otherwise you can easily exceed the 32 kb limit on uploadable userdata (see [issue 8805](https://github.com/siderolabs/talos/issues/8805)).

At this point, you can modify the generated configs to your liking.
Optionally, you can specify [machine configuration patches]({{< relref "../../configuration/patching/#configuration-patching-with-talosctl-cli" >}}) which will be applied during the config generation.

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
    --type cx22 --location hel1 \
    --label 'type=controlplane' \
    --user-data-from-file controlplane.yaml

hcloud server create --name talos-control-plane-2 \
    --image ${IMAGE_ID} \
    --type cx22 --location fsn1 \
    --label 'type=controlplane' \
    --user-data-from-file controlplane.yaml

hcloud server create --name talos-control-plane-3 \
    --image ${IMAGE_ID} \
    --type cx22 --location nbg1 \
    --label 'type=controlplane' \
    --user-data-from-file controlplane.yaml
```

#### Create the Worker Nodes

Create the worker nodes with the following command, repeating (and incrementing the name counter) as many times as desired.

```bash
hcloud server create --name talos-worker-1 \
    --image ${IMAGE_ID} \
    --type cx22 --location hel1 \
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

After a successful bootstrap, you should see that all the members have joined:

```bash
talosctl --talosconfig talosconfig -n <control-plane-1-IP> get members
```

### Retrieve the `kubeconfig`

At this point we can retrieve the admin `kubeconfig` by running:

```bash
talosctl --talosconfig talosconfig kubeconfig .
```

### Install Hetzner's Cloud Controller Manager

First of all, we need to patch the Talos machine configuration used by each node:

```yaml
# patch.yaml
cluster:
  externalCloudProvider:
    enabled: true
```

Then run the following command:

```bash
talosctl --talosconfig talosconfig patch machineconfig --patch-file patch.yaml --nodes <comma separated list of all your nodes' IP addresses>
```

With that in place, we can now follow the [official instructions](https://github.com/hetznercloud/hcloud-cloud-controller-manager), ignoring the `kubeadm` related steps.
