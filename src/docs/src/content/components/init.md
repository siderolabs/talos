---
title: "init"
date: 2018-10-29T19:40:55-07:00
draft: false
weight: 20
menu:
  main:
    parent: 'components'
    weight: 20
---

A common theme throughout the design of Talos is minimalism.
We believe strongly in the UNIX philosophy that each program should do one job well.
The `init` included in Talos is one example of this.

We wanted to create a focused `init` that had one job - run Kubernetes.
There simply is no mechanism in place to do anything else.

To accomplish this, we must address real world operations needs like:

- Orchestration around creating a highly available control plane
- Log retrieval
- Restarting system services
- Rebooting a node
- and more

In the following sections we will take a closer look at how these needs are addressed, and how services managed by `init` are designed to enhance the Kubernetes experience.
