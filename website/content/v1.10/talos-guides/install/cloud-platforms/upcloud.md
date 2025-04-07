---
title: "UpCloud"
description: "Creating a cluster via the CLI (upctl) on UpCloud.com."
aliases:
  - ../../../cloud-platforms/upcloud
---

In this guide we will create an HA Kubernetes cluster 3 control plane nodes and 1 worker node.
We assume some familiarity with UpCloud.
If you need more information on UpCloud specifics, please see the [official UpCloud documentation](https://upcloud.com/resources/docs).

## Create the Image

The best way to create an image for UpCloud, is to build one using
[Hashicorp packer](https://www.packer.io/docs/builders/hetzner-cloud), with the
`upcloud-amd64.raw.xz` image available from the [Image Factory](https://factory.talos.dev/).
Using the general ISO is also possible, but the UpCloud image has some UpCloud
specific features implemented, such as the fetching of metadata and user data to configure the nodes.

To create the cluster, you need a few things locally installed:

1. [UpCloud CLI](https://github.com/UpCloudLtd/upcloud-cli)
2. [Hashicorp Packer](https://learn.hashicorp.com/tutorials/packer/get-started-install-cli)

> NOTE: Make sure your account allows API connections.
> To do so, log into
> [UpCloud control panel](https://hub.upcloud.com/login) and go to **People**
> -> **Account** -> **Permissions** -> **Allow API connections** checkbox.
> It is recommended
> to create a separate subaccount for your API access and _only_ set the API permission.

To use the UpCloud CLI, you need to create a config in `$HOME/.config/upctl.yaml`

```yaml
username: your_upcloud_username
password: your_upcloud_password
```

To use the UpCloud packer plugin, you need to also export these credentials to your
environment variables, by e.g. putting the following in your `.bashrc` or `.zshrc`

```shell
export UPCLOUD_USERNAME="<username>"
export UPCLOUD_PASSWORD="<password>"
```

Next create a config file for packer to use:

```hcl
# upcloud.pkr.hcl

packer {
  required_plugins {
    upcloud = {
      version = ">=v1.0.0"
      source  = "github.com/UpCloudLtd/upcloud"
    }
  }
}

variable "talos_version" {
  type    = string
  default = "{{< release >}}"
}

locals {
  image = "https://factory.talos.dev/image/376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba/${var.talos_version}/upcloud-amd64.raw.xz"
}

variable "username" {
  type        = string
  description = "UpCloud API username"
  default     = "${env("UPCLOUD_USERNAME")}"
}

variable "password" {
  type        = string
  description = "UpCloud API password"
  default     = "${env("UPCLOUD_PASSWORD")}"
  sensitive   = true
}

source "upcloud" "talos" {
  username        = "${var.username}"
  password        = "${var.password}"
  zone            = "us-nyc1"
  storage_name    = "Debian GNU/Linux 11 (Bullseye)"
  template_name   = "Talos (${var.talos_version})"
}

build {
  sources = ["source.upcloud.talos"]

  provisioner "shell" {
    inline = [
      "apt-get install -y wget xz-utils",
      "wget -q -O /tmp/talos.raw.xz ${local.image}",
      "xz -d -c /tmp/talos.raw.xz | dd of=/dev/vda",
    ]
  }

  provisioner "shell-local" {
      inline = [
      "upctl server stop --type hard custom",
      ]
  }
}
```

Now create a new image by issuing the commands shown below.

```bash
packer init .
packer build .
```

After doing this, you can find the custom image in the console interface under storage.

## Creating a Cluster via the CLI

### Create an Endpoint

To communicate with the Talos cluster you will need a single endpoint that is used
to access the cluster.
This can either be a loadbalancer that will sit in front of
all your control plane nodes, a DNS name with one or more A or AAAA records pointing
to the control plane nodes, or directly the IP of a control plane node.

Which option is best for you will depend on your needs.
Endpoint selection has been further documented [here]({{< relref "../../../introduction//getting-started/#decide-the-kubernetes-endpoint" >}}).

After you decide on which endpoint to use, note down the domain name or IP, as
we will need it in the next step.

### Create the Machine Configuration Files

#### Generating Base Configurations

Using the DNS name of the endpoint created earlier, generate the base
configuration files for the Talos machines:

```bash
$ talosctl gen config talos-upcloud-tutorial https://<load balancer IP or DNS>:<port> --install-disk /dev/vda
created controlplane.yaml
created worker.yaml
created talosconfig
```

At this point, you can modify the generated configs to your liking.
Depending on the Kubernetes version you want to run, you might need to select a different Talos version, as not all versions are compatible.
 You can find the support matrix [here]({{< relref "../../../introduction/support-matrix" >}}).

Optionally, you can specify [machine configuration patches]({{< relref "../../configuration/patching/#configuration-patching-with-talosctl-cli" >}})
which will be applied during the config generation.

#### Validate the Configuration Files

```bash
$ talosctl validate --config controlplane.yaml --mode cloud
controlplane.yaml is valid for cloud mode
$ talosctl validate --config worker.yaml --mode cloud
worker.yaml is valid for cloud mode
```

### Create the Servers

#### Create the Control Plane Nodes

Run the following to create three total control plane nodes:

```bash
for ID in $(seq 3); do
    upctl server create \
      --zone us-nyc1 \
      --title talos-us-nyc1-master-$ID \
      --hostname talos-us-nyc1-master-$ID \
      --plan 2xCPU-4GB \
      --os "Talos ({{< release >}})" \
      --user-data "$(cat controlplane.yaml)" \
      --enable-metada
done
```

> Note: modify the zone and OS depending on your preferences.
> The OS should match the template name generated with packer in the previous step.

Note the IP address of the first control plane node, as we will need it later.

#### Create the Worker Nodes

Run the following to create a worker node:

```bash
upctl server create \
  --zone us-nyc1 \
  --title talos-us-nyc1-worker-1 \
  --hostname talos-us-nyc1-worker-1 \
  --plan 2xCPU-4GB \
  --os "Talos ({{< release >}})" \
  --user-data "$(cat worker.yaml)" \
  --enable-metada
```

### Bootstrap Etcd

To configure `talosctl` we will need the first control plane node's IP, as noted earlier.
We only add one node IP, as that is the entry into our cluster against which our commands will be run.
All requests to other nodes are proxied through the endpoint, and therefore not
all nodes need to be manually added to the config.
You don't want to run your commands against all nodes, as this can destroy your
cluster if you are not careful [(further documentation)]({{< relref "../../../introduction//getting-started/#configure-your-talosctl-client" >}}).

Set the `endpoints` and `nodes`:

```bash
talosctl --talosconfig talosconfig config endpoint <control plane 1 IP>
talosctl --talosconfig talosconfig config node <control plane 1 IP>
```

Bootstrap `etcd`:

```bash
talosctl --talosconfig talosconfig bootstrap
```

### Retrieve the `kubeconfig`

At this point we can retrieve the admin `kubeconfig` by running:

```bash
talosctl --talosconfig talosconfig kubeconfig
```

It will take a few minutes before Kubernetes has been fully bootstrapped, and is accessible.

You can check if the nodes are registered in Talos by running

```bash
talosctl --talosconfig talosconfig get members
```

To check if your nodes are ready, run

```bash
kubectl get nodes
```
