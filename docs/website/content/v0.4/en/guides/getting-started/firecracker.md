---
title: Firecracker
---

In this guide we will create a Kubernetes cluster using Firecracker.

## Requirements

- Linux
- a kernel with KVM enabled (`/dev/kvm` must exist)
- at least `CAP_SYS_ADMIN` and `CAP_NET_ADMIN` capabilities
- [firecracker](https://github.com/firecracker-microvm/firecracker/releases) (v0.21.0 or higher)
- `bridge`, and `firewall` CNI plugins from the [standard CNI plugins](https://github.com/containernetworking/cni), and `tc-redirect-tap` CNI plugin from the [Firecracker Go SDK](https://github.com/firecracker-microvm/firecracker-go-sdk/tree/master/cni) installed to `/opt/cni/bin`
- iptables
- `/etc/cni/conf.d` directory should exist
- `/var/run/netns` directory should exist

## Installation

### How to get firecracker (v0.21.0 or higher)

You can download `firecracker` binary via
https://github.com/firecracker-microvm/firecracker/releases

```
$ curl https://github.com/firecracker-microvm/firecracker/releases/download/<version>/firecracker-<version>-<arch> -L -o firecracker
```

For example version `v0.21.1` for `linux` platform:

```
$ curl https://github.com/firecracker-microvm/firecracker/releases/download/v0.21.1/firecracker-v0.21.1-x86_64 -L -o firecracker
$ sudo cp firecracker /usr/local/bin
$ sudo chmod +x /usr/local/bin/firecracker
```

### Install talosctl

You can download `talosctl` and all required binaries via
https://github.com/talos-systems/talos/releases

```
$ curl https://github.com/talos-systems/talos/releases/download/<version>/talosctl-<platform>-<arch> -L -o talosctl
```

For example version `v0.4.1` for `linux` platform:

```
$ curl https://github.com/talos-systems/talos/releases/download/v0.4.1/talosctl-linux-amd64 -L -o talosctl
$ sudo cp talosctl /usr/local/bin
$ sudo chmod +x /usr/local/bin/talosctl
```

### Install bridge and firewall required CNI plugins

You can download standard CNI required plugins via
https://github.com/containernetworking/plugins/releases

```
$ curl https://github.com/containernetworking/plugins/releases/download/<version>/cni-plugins-<platform>-<arch>-<version>tgz -L -o cni-plugins-<platform>-<arch>-<version>.tgz
```

For example version `v0.8.5` for `linux` platform:

```
$ curl https://github.com/containernetworking/plugins/releases/download/v0.8.5/cni-plugins-linux-amd64-v0.8.5.tgz -L -o cni-plugins-linux-amd64-v0.8.5.tgz
$ mkdir cni-plugins-linux
$ tar -xf cni-plugins-linux-amd64-v0.8.5.tgz -C cni-plugins-linux
$ sudo mkdir -p /opt/cni/bin
$ sudo cp cni-plugins-linux/{bridge,firewall} /opt/cni/bin
```

### Install tc-redirect-tap CNI plugin

You should install CNI plugin from the Firecracker Go SDK repository 
https://github.com/firecracker-microvm/firecracker-go-sdk/tree/master/cni

```
$ go get -d github.com/firecracker-microvm/firecracker-go-sdk
$ cd $GOPATH/src/github.com/firecracker-microvm/firecracker-go-sdk/cni
$ make all
$ sudo cp tc-redirect-tap /opt/cni/bin
```

## Create the Cluster

```bash
sudo talosctl cluster create --provisioner firecracker
```

Once the above finishes successfully, your talosconfig(`~/.talos/config`) will be configured to point to the new cluster.

## Retrieve and Configure the `kubeconfig`

```bash
talosctl kubeconfig .
```

## Using the Cluster

Once the cluster is available, you can make use of `talosctl` and `kubectl` to interact with the cluster.
For example, to view current running containers, run `talosctl containers` for a list of containers in the `system` namespace, or `talosctl containers -k` for the `k8s.io` namespace.
To view the logs of a container, use `talosctl logs <container>` or `talosctl logs -k <container>`.

A bridge interface will be created, and assigned the default IP 10.5.0.1.
Each node will be directly accessible on the subnet specified at cluster creation time.
A loadbalancer runs on 10.5.0.1 by default, which handles loadbalancing for the Talos, and Kubernetes APIs.

You can see a summary of the cluster state by running:

```bash
$ talosctl cluster show --provisioner firecracker
PROVISIONER       firecracker
NAME              talos-default
NETWORK NAME      talos-default
NETWORK CIDR      10.5.0.0/24
NETWORK GATEWAY   10.5.0.1
NETWORK MTU       1500

NODES:

NAME                     TYPE           IP         CPU    RAM      DISK
talos-default-master-1   Init           10.5.0.2   1.00   1.6 GB   4.3 GB
talos-default-master-2   ControlPlane   10.5.0.3   1.00   1.6 GB   4.3 GB
talos-default-master-3   ControlPlane   10.5.0.4   1.00   1.6 GB   4.3 GB
talos-default-worker-1   Join           10.5.0.5   1.00   1.6 GB   4.3 GB
```

## Cleaning Up

To cleanup, run:

```bash
sudo talosctl cluster destroy --provisioner firecracker
```

> Note: In that case that the host machine is rebooted before destroying the cluster, you may need to manually remove `~/.talos/clusters/talos-default`.
