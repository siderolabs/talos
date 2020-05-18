---
title: 'machined'
---

A common theme throughout the design of Talos is minimalism.
We believe strongly in the UNIX philosophy that each program should do one job well.
The `init` included in Talos is one example of this, and we are calling it "`machined`".

We wanted to create a focused `init` that had one job - run Kubernetes.
To that extent, `machined` is relatively static in that it does not allow for arbitrary user defined services.
Only the services necessary to run Kubernetes and manage the node are available.
This includes:

- <router-link to="./containerd">containerd</router-link>
- <router-link to="./kubeadm">kubeadm</router-link>
- [kubelet](https://kubernetes.io/docs/concepts/overview/components/)
- <router-link to="./networkd">networkd</router-link>
- <router-link to="./timed">timed</router-link>
- <router-link to="./osd">osd</router-link>
- <router-link to="./trustd">trustd</router-link>
- <router-link to="./udevd">udevd</router-link>
