---
title: Firecracker
description: "Creating Talos Kubernetes cluster using Firecracker VMs."
---

In this guide we will create a Kubernetes cluster using Firecracker.

> Note: Talos on [QEMU](../qemu/) offers easier way to run Talos in a set of VMs.

## Requirements

- Linux
- a kernel with
  - KVM enabled (`/dev/kvm` must exist)
  - `CONFIG_NET_SCH_NETEM` enabled
  - `CONFIG_NET_SCH_INGRESS` enabled
- at least `CAP_SYS_ADMIN` and `CAP_NET_ADMIN` capabilities
- [firecracker](https://github.com/firecracker-microvm/firecracker/releases) (v0.21.0 or higher)
- `bridge`, `static` and `firewall` CNI plugins from the [standard CNI plugins](https://github.com/containernetworking/cni), and `tc-redirect-tap` CNI plugin from the [awslabs tc-redirect-tap](https://github.com/awslabs/tc-redirect-tap) installed to `/opt/cni/bin`
- iptables
- `/etc/cni/conf.d` directory should exist
- `/var/run/netns` directory should exist

## Installation

### How to get firecracker (v0.21.0 or higher)

You can download `firecracker` binary via
[github.com/firecracker-microvm/firecracker/releases](https://github.com/firecracker-microvm/firecracker/releases)

```bash
curl https://github.com/firecracker-microvm/firecracker/releases/download/<version>/firecracker-<version>-<arch> -L -o firecracker
```

For example version `v0.21.1` for `linux` platform:

```bash
curl https://github.com/firecracker-microvm/firecracker/releases/download/v0.21.1/firecracker-v0.21.1-x86_64 -L -o firecracker
sudo cp firecracker /usr/local/bin
sudo chmod +x /usr/local/bin/firecracker
```

### Install talosctl

You can download `talosctl` and all required binaries via
[github.com/talos-systems/talos/releases](https://github.com/talos-systems/talos/releases)

```bash
curl https://github.com/talos-systems/talos/releases/download/<version>/talosctl-<platform>-<arch> -L -o talosctl
```

For example version `v0.8.0` for `linux` platform:

```bash
curl https://github.com/talos-systems/talos/releases/download/v0.8.0/talosctl-linux-amd64 -L -o talosctl
sudo cp talosctl /usr/local/bin
sudo chmod +x /usr/local/bin/talosctl
```

### Install bridge, firewall and static required CNI plugins

You can download standard CNI required plugins via
[github.com/containernetworking/plugins/releases](https://github.com/containernetworking/plugins/releases)

```bash
curl https://github.com/containernetworking/plugins/releases/download/<version>/cni-plugins-<platform>-<arch>-<version>tgz -L -o cni-plugins-<platform>-<arch>-<version>.tgz
```

For example version `v0.8.5` for `linux` platform:

```bash
curl https://github.com/containernetworking/plugins/releases/download/v0.8.5/cni-plugins-linux-amd64-v0.8.5.tgz -L -o cni-plugins-linux-amd64-v0.8.5.tgz
mkdir cni-plugins-linux
tar -xf cni-plugins-linux-amd64-v0.8.5.tgz -C cni-plugins-linux
sudo mkdir -p /opt/cni/bin
sudo cp cni-plugins-linux/{bridge,firewall,static} /opt/cni/bin
```

### Install tc-redirect-tap CNI plugin

You should install CNI plugin from the `tc-redirect-tap` repository [github.com/awslabs/tc-redirect-tap](https://github.com/awslabs/tc-redirect-tap)

```bash
go get -d github.com/awslabs/tc-redirect-tap/cmd/tc-redirect-tap
cd $GOPATH/src/github.com/awslabs/tc-redirect-tap
make all
sudo cp tc-redirect-tap /opt/cni/bin
```

> Note: if `$GOPATH` is not set, it defaults to `~/go`.

## Install Talos kernel and initramfs

Firecracker provisioner depends on Talos uncompressed kernel (`vmlinuz`) and initramfs (`initramfs.xz`).
These files can be downloaded from the Talos release:

```bash
mkdir -p _out/
curl https://github.com/talos-systems/talos/releases/download/<version>/vmlinuz -L -o _out/vmlinuz
curl https://github.com/talos-systems/talos/releases/download/<version>/initramfs.xz -L -o _out/initramfs.xz
```

For example version `v0.8.0`:

```bash
curl https://github.com/talos-systems/talos/releases/download/v0.8.0/vmlinuz -L -o _out/vmlinuz
curl https://github.com/talos-systems/talos/releases/download/v0.8.0/initramfs.xz -L -o _out/initramfs.xz
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

## Manual Clean Up

The `talosctl cluster destroy` command depends heavily on the clusters state directory.
It contains all related information of the cluster.
The PIDs and network associated with the cluster nodes.

If you happened to have deleted the state folder by mistake or you would like to cleanup
the environment, here are the steps how to do it manually:

### Stopping VMs

Find the process of `firecracker --api-sock` execute:

```bash
ps -elf | grep '[f]irecracker --api-sock'
```

To stop the VMs manually, execute:

```bash
sudo kill -s SIGTERM <PID>
```

Example output, where VMs are running with PIDs **158065** and **158216**

```bash
ps -elf | grep '[f]irecracker --api-sock'
4 S root      158065  157615 44  80   0 - 264152 -     07:54 ?        00:34:25 firecracker --api-sock /root/.talos/clusters/k8s/k8s-master-1.sock
4 S root      158216  157617 18  80   0 - 264152 -     07:55 ?        00:14:47 firecracker --api-sock /root/.talos/clusters/k8s/k8s-worker-1.sock
sudo kill -s SIGTERM 158065
sudo kill -s SIGTERM 158216
```

### Remove VMs

Find the process of `talosctl firecracker-launch` execute:

```bash
ps -elf | grep 'talosctl firecracker-launch'
```

To remove the VMs manually, execute:

```bash
sudo kill -s SIGTERM <PID>
```

Example output, where VMs are running with PIDs **157615** and **157617**

```bash
ps -elf | grep '[t]alosctl firecracker-launch'
0 S root      157615    2835  0  80   0 - 184934 -     07:53 ?        00:00:00 talosctl firecracker-launch
0 S root      157617    2835  0  80   0 - 185062 -     07:53 ?        00:00:00 talosctl firecracker-launch
sudo kill -s SIGTERM 157615
sudo kill -s SIGTERM 157617
```

### Remove load balancer

Find the process of `talosctl loadbalancer-launch` execute:

```bash
ps -elf | grep 'talosctl loadbalancer-launch'
```

To remove the LB manually, execute:

```bash
sudo kill -s SIGTERM <PID>
```

Example output, where loadbalancer is running with PID **157609**

```bash
ps -elf | grep '[t]alosctl loadbalancer-launch'
4 S root      157609    2835  0  80   0 - 184998 -     07:53 ?        00:00:07 talosctl loadbalancer-launch --loadbalancer-addr 10.5.0.1 --loadbalancer-upstreams 10.5.0.2
sudo kill -s SIGTERM 157609
```

### Remove network

This is more tricky part as if you have already deleted the state folder.
If you didn't then it is written in the `state.yaml` in the
`/root/.talos/clusters/<cluster-name>` directory.

```bash
sudo cat /root/.talos/clusters/<cluster-name>/state.yaml | grep bridgename
bridgename: talos<uuid>
```

If you only had one cluster, then it will be the interface with name
`talos<uuid>`

```bash
46: talos<uuid>: <NO-CARRIER,BROADCAST,MULTICAST,UP> mtu 1500 qdisc noqueue state DOWN group default qlen 1000
    link/ether a6:72:f4:0a:d3:9c brd ff:ff:ff:ff:ff:ff
    inet 10.5.0.1/24 brd 10.5.0.255 scope global talos17c13299
       valid_lft forever preferred_lft forever
    inet6 fe80::a472:f4ff:fe0a:d39c/64 scope link
       valid_lft forever preferred_lft forever
```

To remove this interface:

```bash
sudo ip link del talos<uuid>
```

### Remove state directory

To remove the state directory execute:

```bash
sudo rm -Rf /root/.talos/clusters/<cluster-name>
```

## Troubleshooting

### Logs

Inspect logs directory

```bash
sudo cat /root/.talos/clusters/<cluster-name>/*.log
```

Logs are saved under `<cluster-name>-<role>-<node-id>.log`

For example in case of **k8s** cluster name:

```bash
sudo ls -la /root/.talos/clusters/k8s | grep log
-rw-r--r--. 1 root root      69415 Apr 26 20:58 k8s-master-1.log
-rw-r--r--. 1 root root      68345 Apr 26 20:58 k8s-worker-1.log
-rw-r--r--. 1 root root      24621 Apr 26 20:59 lb.log
```

Inspect logs during the installation

```bash
sudo su -
tail -f /root/.talos/clusters/<cluster-name>/*.log
```

## Post-installation

After executing these steps and you should be able to use `kubectl`

```bash
sudo talosctl kubeconfig .
mv kubeconfig $HOME/.kube/config
sudo chown $USER:$USER $HOME/.kube/config
```
