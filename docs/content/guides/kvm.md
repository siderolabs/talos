---
title: "KVM"
date: 2018-10-29T19:40:55-07:00
draft: false
menu:
  docs:
    parent: 'guides'
---

## Creating a Master Node

On the KVM host, install a master node to an available block device:

```bash
docker run \
 --rm \
 --privileged \
 --volume /dev:/dev \
 talos-systems/talos:latest image -b /dev/sdb -f -p bare-metal -u http://${IP}:8080/master.yaml
```

{{% note %}}`http://${IP}:8080/master.yaml` should be reachable by the VM and contain a valid master configuration file.{{% /note %}}

Now, create the VM:

```bash
virt-install \
    -n master \
    --description "Kubernetes master node." \
    --os-type=Linux \
    --os-variant=generic \
    --virt-type=kvm \
    --cpu=host \
    --vcpus=2 \
    --ram=4096 \
    --disk path=/dev/sdb \
    --network bridge=br0,model=e1000,mac=52:54:00:A8:4C:E1 \
    --graphics none \
    --boot hd \
    --rng /dev/random
```

## Creating a Worker Node

On the KVM host, install a worker node to an available block device:

```bash
docker run \
 --rm \
 --privileged \
 --volume /dev:/dev \
 talos-systems/talos:latest image -b /dev/sdc -f -p bare-metal -u http://${IP}:8080/worker.yaml
```

{{% note %}}`http://${IP}:8080/worker.yaml` should be reachable by the VM and contain a valid worker configuration file.{{% /note %}}

Now, create the VM:

```bash
virt-install \
    -n master \
    --description "Kubernetes worker node." \
    --os-type=Linux \
    --os-variant=generic \
    --virt-type=kvm \
    --cpu=host \
    --vcpus=2 \
    --ram=4096 \
    --disk path=/dev/sdc \
    --network bridge=br0,model=e1000,mac=52:54:00:B9:5D:F2 \
    --graphics none \
    --boot hd \
    --rng /dev/random
```
