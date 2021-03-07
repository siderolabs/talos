---
title: "Storage"
description: ""
---

Talos is known to work with Rook and NFS.

## Rook

We recommend at least Rook v1.5.

## NFS

The NFS client is part of the [`kubelet` image](https://github.com/talos-systems/kubelet) maintained by the Talos team.
This means that the version installed in your running `kubelet` is the version of NFS supported by Talos.
