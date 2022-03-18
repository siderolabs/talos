---
title: Upgrading Talos
---

Talos upgrades are effected by an API call.
The `talosctl` CLI utility will facilitate this.
<!-- , or you can use the automatic upgrade features provided by the [talos controller manager](https://github.com/talos-systems/talos-controller-manager) -->

## Video Walkthrough

To see a live demo of this writeup, see the video below:

<iframe width="560" height="315" src="https://www.youtube.com/embed/AAF6WhX0USo" title="YouTube video player" frameborder="0" allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture" allowfullscreen></iframe>

### After Upgrade to 0.15

TBD

## `talosctl` Upgrade

To manually upgrade a Talos node, you will specify the node's IP address and the
installer container image for the version of Talos to which you wish to upgrade.

For instance, if your Talos node has the IP address `10.20.30.40` and you want
to install the official version `v0.15.0`, you would enter a command such
as:

```sh
  $ talosctl upgrade --nodes 10.20.30.40 \
      --image ghcr.io/talos-systems/installer:v0.15.0
```

There is an option to this command: `--preserve`, which can be used to explicitly tell Talos to either keep intact its ephemeral data or not.
In most cases, it is correct to just let Talos perform its default action.
However, if you are running a single-node control-plane, you will want to make sure that `--preserve=true`.

If Talos fails to run the upgrade, the `--stage` flag may be used to perform the upgrade after a reboot
which is followed by another reboot to upgraded version.

<!--
## Talos Controller Manager

The Talos Controller Manager can coordinate upgrades of your nodes
automatically.
It ensures that a controllable number of nodes are being
upgraded at any given time.
It also applies an upgrade flow which allows you to classify some machines as
early adopters and others as getting only stable, tested versions.

To find out more about the controller manager and to get it installed and
configured, take a look at the [GitHub page](https://github.com/talos-systems/talos-controller-manager).
Please note that the controller manager is still in fairly early development.
More advanced features, such as time slot scheduling, will be coming in the
future.
-->

## Machine Configuration Changes

TBD
