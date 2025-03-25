---
title: "Vagrant & Libvirt"
aliases:
  - ../../../virtualized-platforms/vagrant-libvirt
---

## Pre-requisities

1. Linux OS
2. [Vagrant](https://www.vagrantup.com) installed
3. [vagrant-libvirt](https://github.com/vagrant-libvirt/vagrant-libvirt) plugin installed
4. [talosctl](https://www.talos.dev/{{< version >}}/introduction/getting-started/#talosctl) installed
5. [kubectl](https://kubernetes.io/docs/tasks/tools/#kubectl) installed

## Overview

We will use Vagrant and its libvirt plugin to create a KVM-based cluster with 3 control plane nodes and 1 worker node.

For this, we will mount Talos ISO into the VMs using a virtual CD-ROM,
and configure the VMs to attempt to boot from the disk first with the fallback to the CD-ROM.

We will also configure a virtual IP address on Talos to achieve high-availability on kube-apiserver.

## Preparing the environment

First, we download the latest `metal-amd64.iso` ISO from GitHub releases into the `/tmp` directory.

```bash
wget --timestamping curl https://factory.talos.dev/image/376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba/{{< release >}}/metal-amd64.iso -O /tmp/metal-amd64.iso
```

Create a `Vagrantfile` with the following contents:

```ruby
Vagrant.configure("2") do |config|
  config.vm.define "control-plane-node-1" do |vm|
    vm.vm.provider :libvirt do |domain|
      domain.cpus = 2
      domain.memory = 2048
      domain.serial :type => "file", :source => {:path => "/tmp/control-plane-node-1.log"}
      domain.storage :file, :device => :cdrom, :path => "/tmp/metal-amd64.iso"
      domain.storage :file, :size => '4G', :type => 'raw'
      domain.boot 'hd'
      domain.boot 'cdrom'
    end
  end

  config.vm.define "control-plane-node-2" do |vm|
    vm.vm.provider :libvirt do |domain|
      domain.cpus = 2
      domain.memory = 2048
      domain.serial :type => "file", :source => {:path => "/tmp/control-plane-node-2.log"}
      domain.storage :file, :device => :cdrom, :path => "/tmp/metal-amd64.iso"
      domain.storage :file, :size => '4G', :type => 'raw'
      domain.boot 'hd'
      domain.boot 'cdrom'
    end
  end

  config.vm.define "control-plane-node-3" do |vm|
    vm.vm.provider :libvirt do |domain|
      domain.cpus = 2
      domain.memory = 2048
      domain.serial :type => "file", :source => {:path => "/tmp/control-plane-node-3.log"}
      domain.storage :file, :device => :cdrom, :path => "/tmp/metal-amd64.iso"
      domain.storage :file, :size => '4G', :type => 'raw'
      domain.boot 'hd'
      domain.boot 'cdrom'
    end
  end

  config.vm.define "worker-node-1" do |vm|
    vm.vm.provider :libvirt do |domain|
      domain.cpus = 1
      domain.memory = 1024
      domain.serial :type => "file", :source => {:path => "/tmp/worker-node-1.log"}
      domain.storage :file, :device => :cdrom, :path => "/tmp/metal-amd64.iso"
      domain.storage :file, :size => '4G', :type => 'raw'
      domain.boot 'hd'
      domain.boot 'cdrom'
    end
  end
end
```

## Bring up the nodes

Check the status of vagrant VMs:

```bash
vagrant status
```

You should see the VMs in "not created" state:

```text
Current machine states:

control-plane-node-1      not created (libvirt)
control-plane-node-2      not created (libvirt)
control-plane-node-3      not created (libvirt)
worker-node-1             not created (libvirt)
```

Bring up the vagrant environment:

```bash
vagrant up --provider=libvirt
```

Check the status again:

```bash
vagrant status
```

Now you should see the VMs in "running" state:

```text
Current machine states:

control-plane-node-1      running (libvirt)
control-plane-node-2      running (libvirt)
control-plane-node-3      running (libvirt)
worker-node-1             running (libvirt)
```

Find out the IP addresses assigned by the libvirt DHCP by running:

```bash
virsh list | grep vagrant | awk '{print $2}' | xargs -t -L1 virsh domifaddr
```

Output will look like the following:

```text
virsh domifaddr vagrant_control-plane-node-2
 Name       MAC address          Protocol     Address
-------------------------------------------------------------------------------
 vnet0      52:54:00:f9:10:e5    ipv4         192.168.121.119/24

virsh domifaddr vagrant_control-plane-node-1
 Name       MAC address          Protocol     Address
-------------------------------------------------------------------------------
 vnet1      52:54:00:0f:ae:59    ipv4         192.168.121.203/24

virsh domifaddr vagrant_worker-node-1
 Name       MAC address          Protocol     Address
-------------------------------------------------------------------------------
 vnet2      52:54:00:6f:28:95    ipv4         192.168.121.69/24

virsh domifaddr vagrant_control-plane-node-3
 Name       MAC address          Protocol     Address
-------------------------------------------------------------------------------
 vnet3      52:54:00:03:45:10    ipv4         192.168.121.125/24
```

Our control plane nodes have the IPs: `192.168.121.203`, `192.168.121.119`, `192.168.121.125` and the worker node has the IP `192.168.121.69`.

Now you should be able to interact with Talos nodes that are in maintenance mode:

```bash
talosctl -n 192.168.121.203 get disks --insecure
```

Sample output:

```text
DEV        MODEL   SERIAL   TYPE   UUID   WWID   MODALIAS                    NAME   SIZE     BUS_PATH
/dev/vda   -       -        HDD    -      -      virtio:d00000002v00001AF4   -      8.6 GB   /pci0000:00/0000:00:03.0/virtio0/
```

## Installing Talos

Pick an endpoint IP in the `vagrant-libvirt` subnet but not used by any nodes, for example `192.168.121.100`.

Generate a machine configuration:

```bash
talosctl gen config my-cluster https://192.168.121.100:6443 --install-disk /dev/vda
```

Edit `controlplane.yaml` to add the virtual IP you picked to a network interface under `.machine.network.interfaces`, for example:

```yaml
machine:
  network:
    interfaces:
      - deviceSelector:
          physical: true # should select any hardware network device, if you have just one, it will be selected
        dhcp: true
        vip:
          ip: 192.168.121.100
```

Apply the configuration to the initial control plane node:

```bash
talosctl -n 192.168.121.203 apply-config --insecure --file controlplane.yaml
```

You can tail the logs of the node:

```bash
sudo tail -f /tmp/control-plane-node-1.log
```

Set up your shell to use the generated talosconfig and configure its endpoints (use the IPs of the control plane nodes):

```bash
export TALOSCONFIG=$(realpath ./talosconfig)
talosctl config endpoint 192.168.121.203 192.168.121.119 192.168.121.125
```

Bootstrap the Kubernetes cluster from the initial control plane node:

```bash
talosctl -n 192.168.121.203 bootstrap
```

Finally, apply the machine configurations to the remaining nodes:

```bash
talosctl -n 192.168.121.119 apply-config --insecure --file controlplane.yaml
talosctl -n 192.168.121.125 apply-config --insecure --file controlplane.yaml
talosctl -n 192.168.121.69 apply-config --insecure --file worker.yaml
```

After a while, you should see that all the members have joined:

```bash
talosctl -n 192.168.121.203 get members
```

The output will be like the following:

```text
NODE              NAMESPACE   TYPE     ID                      VERSION   HOSTNAME                MACHINE TYPE   OS               ADDRESSES
192.168.121.203   cluster     Member   talos-192-168-121-119   1         talos-192-168-121-119   controlplane   Talos (v1.1.0)   ["192.168.121.119"]
192.168.121.203   cluster     Member   talos-192-168-121-69    1         talos-192-168-121-69    worker         Talos (v1.1.0)   ["192.168.121.69"]
192.168.121.203   cluster     Member   talos-192-168-121-203   6         talos-192-168-121-203   controlplane   Talos (v1.1.0)   ["192.168.121.100","192.168.121.203"]
192.168.121.203   cluster     Member   talos-192-168-121-125   1         talos-192-168-121-125   controlplane   Talos (v1.1.0)   ["192.168.121.125"]
```

## Interacting with Kubernetes cluster

Retrieve the kubeconfig from the cluster:

```bash
talosctl -n 192.168.121.203 kubeconfig ./kubeconfig
```

List the nodes in the cluster:

```bash
kubectl --kubeconfig ./kubeconfig get node -owide
```

You will see an output similar to:

```text
NAME                    STATUS   ROLES                  AGE     VERSION   INTERNAL-IP       EXTERNAL-IP   OS-IMAGE         KERNEL-VERSION   CONTAINER-RUNTIME
talos-192-168-121-203   Ready    control-plane,master   3m10s   v1.24.2   192.168.121.203   <none>        Talos (v1.1.0)   5.15.48-talos    containerd://1.6.6
talos-192-168-121-69    Ready    <none>                 2m25s   v1.24.2   192.168.121.69    <none>        Talos (v1.1.0)   5.15.48-talos    containerd://1.6.6
talos-192-168-121-119   Ready    control-plane,master   8m46s   v1.24.2   192.168.121.119   <none>        Talos (v1.1.0)   5.15.48-talos    containerd://1.6.6
talos-192-168-121-125   Ready    control-plane,master   3m11s   v1.24.2   192.168.121.125   <none>        Talos (v1.1.0)   5.15.48-talos    containerd://1.6.6
```

Congratulations, you have a highly-available Talos cluster running!

## Cleanup

You can destroy the vagrant environment by running:

```bash
vagrant destroy -f
```

And remove the ISO image you downloaded:

```bash
sudo rm -f /tmp/metal-amd64.iso
```
