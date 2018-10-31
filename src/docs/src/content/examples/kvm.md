---
title: "KVM"
date: 2018-10-29T19:40:55-07:00
draft: false
menu:
  main:
    parent: 'examples'
    weight: 20
---

## Install the Master Node

On the KVM host, install a master node to an available block device:

```bash
docker run \
 --rm \
 --privileged \
 --volume /dev:/dev \
 autonomy/dianemo:latest image -b /dev/sda -f -p bare-metal -u http://${IP}:8080/master.yaml
```

> `http://${IP}:8080/master.yaml` should be reachable by the VM and contain a valid master configuration file.

```bash
virt-install \
    -n master \
    --description "Kubernetes master node." \
    --os-type=Linux \
    --os-variant=generic \
    --virt-type=kvm \
    --cpu=host \
    --ram=4096 \
    --vcpus=2 \
    --disk path=/dev/sdc \
    --network bridge=br0,model=e1000,mac=52:54:00:A8:4C:E1 \
    --graphics none \
    --boot hd \
    --rng /dev/random
```

## Install a Worker Node

Similarly, install a worker node to an available block device:

```bash
docker run \
 --rm \
 --privileged \
 --volume /dev:/dev \
 autonomy/dianemo:latest image -b /dev/sdb -f -p bare-metal -u http://${IP}:8080/worker.yaml
```

> `http://${IP}:8080/worker.yaml` should be reachable by the VM and contain a valid worker configuration file.

```bash
virt-install \
    -n master \
    --description "Kubernetes worker node." \
    --os-type=Linux \
    --os-variant=generic \
    --virt-type=kvm \
    --cpu=host \
    --ram=4096 \
    --vcpus=2 \
    --disk path=/dev/sdc \
    --network bridge=br0,model=e1000,mac=52:54:00:B9:5D:F2 \
    --graphics none \
    --boot hd \
    --rng /dev/random
```
