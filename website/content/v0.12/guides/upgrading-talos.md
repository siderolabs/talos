---
title: Upgrading Talos
---

Talos upgrades are effected by an API call.
The `talosctl` CLI utility will facilitate this.
<!-- , or you can use the automatic upgrade features provided by the [talos controller manager](https://github.com/talos-systems/talos-controller-manager) -->

## Video Walkthrough

To see a live demo of this writeup, see the video below:

<iframe width="560" height="315" src="https://www.youtube.com/embed/sw78qS8vBGc" frameborder="0" allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture" allowfullscreen></iframe>

## Upgrading from Talos 0.11

Only for clusters bootstrapped with Talos <= 0.8: please make sure control plane was converted to use static pods
(first introduced in Talos 0.9), as Talos 0.12 drops support for self-hosted control plane.

### After Upgrade to 0.12

There are no special items to follow after the upgrade to Talos 0.12, please see section on machine configuration changes
below.

## `talosctl` Upgrade

To manually upgrade a Talos node, you will specify the node's IP address and the
installer container image for the version of Talos to which you wish to upgrade.

For instance, if your Talos node has the IP address `10.20.30.40` and you want
to install the official version `v0.12.0`, you would enter a command such
as:

```sh
  $ talosctl upgrade --nodes 10.20.30.40 \
      --image ghcr.io/talos-systems/installer:v0.12.0
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

There are two new machine configuration fields introduced in Talos 0.12 which are not being used in Talos 0.12 yet,
but they will be used to support additional features in Talos 0.13:

* `.cluster.id`: 32 random bytes, base64 encoded
* `.cluster.secret`: 32 random bytes, base64 encoded

Values of these fields should be kept in sync across all nodes of the cluster (control plane and worker nodes).

These fields can be added to the machine configuration of the running Talos cluster upgraded to Talos 0.12 with the following commands
(doesn't require a reboot):

```bash
$ CLUSTER_ID=`dd if=/dev/urandom of=/dev/stdout bs=1 count=32 | base64`
32+0 records in
32+0 records out
32 bytes copied, 0,000180749 s, 177 kB/s
$ CLUSTER_SECRET=`dd if=/dev/urandom of=/dev/stdout bs=1 count=32 | base64`
32+0 records in
32+0 records out
32 bytes copied, 0,000180749 s, 177 kB/s
$ talosctl -n <IP> patch mc --immediate --patch "[{\"op\": \"add\", \"path\": \"/cluster/id\", \"value\": \"$CLUSTER_ID\"},{\"op\": \"add\", \"path\": \"/cluster/secret\", \"value\": \"$CLUSTER_SECRET\"}]"
patched mc at the node <IP>
```

Repeat the last command for every node of the cluster.
