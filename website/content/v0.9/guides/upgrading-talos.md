---
title: Upgrading Talos
---

Talos upgrades are effected by an API call.
The `talosctl` CLI utility will facilitate this.
<!-- , or you can use the automatic upgrade features provided by the [talos controller manager](https://github.com/talos-systems/talos-controller-manager) -->

## Video Walkthrough

To see a live demo of this writeup, see the video below:

<iframe width="560" height="315" src="https://www.youtube.com/embed/sw78qS8vBGc" frameborder="0" allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture" allowfullscreen></iframe>

## Upgrading from Talos 0.8

Talos 0.9 drops support for `bootkube` and self-hosted control plane.

Please make sure Talos is upgraded to the latest minor release of 0.8 first (0.8.4 at the moment
of this writing), then proceed with upgrading to the latest minor release of 0.9.

### Before Upgrade to 0.9

If cluster was bootstrapped on Talos version < 0.8.3, add checkpointer annotations to
the `kube-scheduler` and `kube-controller-manager` daemonsets to improve resiliency of
self-hosted control plane to reboots (this is critical for single control-plane node clusters):

```bash
$ kubectl -n kube-system patch daemonset kube-controller-manager --type json -p '[{"op": "add", "path":"/spec/template/metadata/annotations", "value": {"checkpointer.alpha.coreos.com/checkpoint": "true"}}]'
daemonset.apps/kube-controller-manager patched
$ kubectl -n kube-system patch daemonset kube-scheduler --type json -p '[{"op": "add", "path":"/spec/template/metadata/annotations", "value": {"checkpointer.alpha.coreos.com/checkpoint": "true"}}]'
daemonset.apps/kube-scheduler patched
```

Talos 0.9 only supports Kubernetes versions 1.19.x and 1.20.x.
If running 1.18.x, please upgrade Kubernetes before upgrading Talos.

Make sure cluster is running latest minor release of Talos 0.8.

Prepare by downloading `talosctl` binary for Talos release 0.9.x.

### After Upgrade to 0.9

After the upgrade to 0.9, Talos will still be running self-hosted control plane until the [conversion process](../converting-control-plane/) is run.

> Note: Talos 0.9 doesn't include bootkube recovery option (`talosctl recover`), so
> it's not possible to recover self-hosted control plane after upgrading to 0.9.

As soon as all the nodes get upgraded to 0.9, run `talosctl convert-k8s` to convert the control plane
to the new static pod format for 0.9.

Once the conversion process is complete, Kubernetes can be upgraded.

## `talosctl` Upgrade

To manually upgrade a Talos node, you will specify the node's IP address and the
installer container image for the version of Talos to which you wish to upgrade.

For instance, if your Talos node has the IP address `10.20.30.40` and you want
to install the official version `v0.9.0`, you would enter a command such
as:

```sh
  $ talosctl upgrade --nodes 10.20.30.40 \
      --image ghcr.io/talos-systems/installer:v0.9.0
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

Talos 0.9 introduces new required parameters in machine configuration:

* `.cluster.aggregatorCA`
* `.cluster.serviceAccount`

Talos supports both ECDSA and RSA certificates and keys for Kubernetes and etcd, with ECDSA being default.
Talos <= 0.8 supports only RSA keys and certificates.

Utility `talosctl gen config` generates by default config in 0.9 format which is not compatible with
Talos 0.8, but old format can be generated with `talosctl gen config --talos-version=v0.8`.
