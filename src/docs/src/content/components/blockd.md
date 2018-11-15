---
title: "blockd"
date: 2018-10-30T09:16:35-07:00
draft: false
weight: 80
menu:
  main:
    parent: 'components'
    weight: 80
---

Talos comes with a reserved block device with three partitions:

- an EFI System Partition (`ESP`)
- a `ROOT` partition mounted as read-only that contains the minimal set of binaries to operate system services
- and a `DATA` partion that is mounted as read/write at `/var/run`

These partitions are reserved and cannot be modified.
The one exception to this is that the `DATA` partition will be resized automatically in the `init` process to the maximum size possible.
Managing any other block device can be done via the `blockd` service.
