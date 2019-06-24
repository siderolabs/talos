---
title: "kubeadm"
date: 2018-10-29T19:40:55-07:00
draft: false
menu:
  docs:
    parent: 'components'
---

[`kubeadm`](https://github.com/kubernetes/kubernetes/tree/master/cmd/kubeadm) handles the installation and configuration of Kubernetes. This is done to stay as close as possible to upstream Kubernetes best practices and recommendations. By integrating with `kubeadm` natively, the development and operational ecosystem is familiar to all Kubernetes users.

Kubeadm configuration is defined in the userdata under the `services.kubeadm` section.
