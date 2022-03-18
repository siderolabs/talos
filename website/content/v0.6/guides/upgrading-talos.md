---
title: Upgrading Talos
---

## Video Walkthrough

To see a live demo of this writeup, see the video below:

<iframe width="560" height="315" src="https://www.youtube.com/embed/sw78qS8vBGc" frameborder="0" allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture" allowfullscreen></iframe>

## Talos

In an effort to create more production ready clusters, Talos will now taint control plane nodes as unschedulable.
This means that any application you might have deployed must tolerate this taint if you intend on running the application on control plane nodes.

Another feature you will notice is the automatic uncordoning of nodes that have been upgraded.
Talos will now uncordon a node if the cordon was initiated by the upgrade process.

## Talosctl

The `talosctl` CLI now requires an explicit set of nodes.
This can be configured with `talos config nodes` or set on the fly with `talos --nodes`.
