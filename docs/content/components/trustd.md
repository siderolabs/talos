---
title: "trustd"
date: 2018-10-29T19:40:55-07:00
draft: false
menu:
  docs:
    parent: 'components'
---

Security is one of the highest priorities within Talos.
To run a Kubernetes cluster a certain level of trust is required to operate a cluster.
For example, orchestrating the bootstrap of a highly available control plane requires the distribution of sensitive PKI data.

To that end, we created `trustd`.
Based on the concept of a Root of Trust, `trustd` is a simple daemon responsible for establishing trust within the system.
Once trust is established, various methods become available to the trustee.
It can, for example, accept a write request from another node to place a file on disk.

We imagine that the number available methods will grow as Talos gets tested in the real world.
