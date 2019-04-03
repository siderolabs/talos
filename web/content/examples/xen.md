---
title: "Xen"
date: 2018-11-06T06:25:46-08:00
draft: false
weight: 40
menu:
  main:
    parent: 'examples'
    weight: 40
---

## Creating a Master Node

On `Dom0`, install Talos to an available block device:

```bash
docker run \
 --rm \
 --privileged \
 --volume /dev:/dev \
 talos-systems/talos:latest image -b /dev/sdb
```

Save the following as `/etc/xen/master.cfg`

```python
name = "master"

builder='hvm'
bootloader = "/bin/pygrub"
firmware_override = "/usr/lib64/xen/boot/hvmloader"

vcpus=2
memory = 4096
serial = "pty"

kernel = "/var/lib/xen/talos/vmlinuz"
ramdisk = "/var/lib/xen/talos/initramfs.xz"
disk = [ 'phy:/dev/sdb,xvda,w', ]
vif = [ 'mac=52:54:00:A8:4C:E1,bridge=xenbr0,model=e1000', ]
extra = "consoleblank=0 console=hvc0 console=tty0 console=ttyS0,9600 talos.platform=bare-metal talos.userdata=http://${IP}:8080/master.yaml"
```

{{% note %}}`http://${IP}:8080/master.yaml` should be reachable by the VM and contain a valid master configuration file.{{% /note %}}

Now, create the VM:

```bash
xl create /etc/xen/master.cfg
```

## Creating a Worker Node

On `Dom0`, install Talos to an available block device:

```bash
docker run \
 --rm \
 --privileged \
 --volume /dev:/dev \
 talos-systems/talos:latest image -b /dev/sdc
```

Save the following as `/etc/xen/worker.cfg`

```python
name = "worker"

builder='hvm'
bootloader = "/bin/pygrub"
firmware_override = "/usr/lib64/xen/boot/hvmloader"

vcpus=2
memory = 4096
serial = "pty"

kernel = "/var/lib/xen/talos/vmlinuz"
ramdisk = "/var/lib/xen/talos/initramfs.xz"
disk = [ 'phy:/dev/sdc,xvda,w', ]
vif = [ 'mac=52:54:00:B9:5D:F2,bridge=xenbr0,model=e1000', ]
extra = "consoleblank=0 console=hvc0 console=tty0 console=ttyS0,9600 talos.platform=bare-metal talos.userdata=http://${IP}:8080/worker.yaml"
```

{{% note %}}`http://${IP}:8080/worker.yaml` should be reachable by the VM and contain a valid worker configuration file.{{% /note %}}

Now, create the VM:

```bash
xl create /etc/xen/worker.cfg
```
