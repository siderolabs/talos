---
title: Upgrading Talos Linux
---

OS upgrades, like other operations on Talos Linux, are effected by an API call, which can be sent via the `talosctl` CLI utility.
Because Talos Linux is image based, an upgrade is almost the same as installing Talos, with the difference that the system has already been initialized with a configuration.

The upgrade API call passes a node the installer image to use to perform the upgrade.
Each Talos version has a corresponding installer.

Upgrades use an A-B image scheme in order to facilitate rollbacks.
This scheme retains the previous Talos kernel and OS image following each upgrade.
If an upgrade fails to boot, Talos will roll back to the previous version.
Likewise, Talos may be manually rolled back via API (or `talosctl rollback`).
This will simply update the boot reference and reboot.

Unless explicitly told to `preserve` data, an upgrade will cause the node to wipe the ephemeral partition, remove itself from the etcd cluster (if it is a control node), and generally make itself as pristine as is possible.
(This is generally the desired behavior, except in specialised use cases such as single-node clusters.)

*Note* that unless the Kubernetes version has been specified in the machine config, an upgrade of the Talos Linux OS will also apply an upgrade of the Kubernetes version.
Each release of Talos Linux includes the latest stable Kubernetes version by default.

## Video Walkthrough

To see a live demo of an upgrade of Talos Linux, see the video below:

<iframe width="560" height="315" src="https://www.youtube.com/embed/AAF6WhX0USo" title="YouTube video player" frameborder="0" allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture" allowfullscreen></iframe>

### After Upgrade to 0.15

TBD

## `talosctl upgrade`

To upgrade a Talos node, specify the node's IP address and the
installer container image for the version of Talos to upgrade to.

For instance, if your Talos node has the IP address `10.20.30.40` and you want
to install the official version `v1.0.0`, you would enter a command such
as:

```sh
  $ talosctl upgrade --nodes 10.20.30.40 \
      --image ghcr.io/talos-systems/installer:v1.0.0
```

There is an option to this command: `--preserve`, which will explicitly tell Talos to keep ephemeral data intact.
In most cases, it is correct to let Talos perform its default action of erasing the ephemeral data.
However, if you are running a single-node control-plane, you will want to make sure that `--preserve=true`.

Rarely, a upgrade command will fail to run due to a process holding a file open on disk, or you may wish to set a node to upgrade, but delay the actual reboot as long as possible.
In these cases, you can use the `--stage` flag.
This puts the upgrade artifacts on disk, and adds some metadata to a disk partition that gets checked very early in the boot process.
The node is *not* rebooted by the `upgrade --stage` process.
However, whenever the system does next reboot, Talos sees that it needs to apply an upgrade, and will do so immediately.
Because this occurs in a just rebooted system, there will be no conflict with any files being held open.
After the upgrade is applied, the node will reboot again, in order to boot into the new version.
Note that because Talos Linux now reboots via the kexec syscall, the extra reboot adds very little time.

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

## Upgrade Sequence

When a Talos node receives the upgrade command, it cordons
itself in Kubernetes, to avoid receiving any new workload.
It then starts to drain its existing workload.

**NOTE**: If any of your workloads are sensitive to being shut down ungracefully, be sure to use the `lifecycle.preStop` Pod [spec](https://kubernetes.io/docs/concepts/containers/container-lifecycle-hooks/#container-hooks).

Once all of the workload Pods are drained, Talos will start shutting down its
internal processes.
If it is a control node, this will include etcd.
If `preserve` is not enabled, Talos will leave etcd membership.
(Talos ensures the etcd cluster is healthy and will remain healthy after our node leaves the etcd cluster, before allowing a control plane node to be upgraded.)

Once all the processes are stopped and the services are shut down, the filesystems will be unmounted.
This allows Talos to produce a very clean upgrade, as close as possible to a pristine system.
We verify the disk and then perform the actual image upgrade.
We set the bootloader to boot _once_ with the new kernel and OS image, then we reboot.

After the node comes back up and Talos verifies itself, it will make
the bootloader change permanent, rejoin the cluster, and finally uncordon itself to receive new workloads.

### FAQs

**Q.** What happens if an upgrade fails?

**A.** Talos Linux attempts to safely handle upgrade failures.

The most common failure is an invalid installer image reference.
In this case, Talos will fail to download the upgraded image and will abort the upgrade.

Sometimes, Talos is unable to successfully kill off all of the disk access points, in which case it cannot safely unmount all filesystems to effect the upgrade.
In this case, it will abort the upgrade and reboot.
(`upgrade --stage` can ensure that upgrades can occur even when the filesytems cannot be unmounted.)

It is possible (especially with test builds) that the upgraded Talos system will fail to start.
In this case, the node will be rebooted, and the bootloader will automatically use the previous Talos kernel and image, thus effectively rolling back the upgrade.

Lastly, it is possible that Talos itself will upgrade successfully, start up, and rejoin the cluster but your workload will fail to run on it, for whatever reason.
This is when you would use the `talosctl rollback` command to revert back to the previous Talos version.

**Q.** Can upgrades be scheduled?

**A.** Because the upgrade sequence is API-driven, you can easily tie it in to your own business logic to schedule and coordinate your upgrades.

**Q.** Can the upgrade process be observed?

**A.** Yes, using the `talosctl dmesg -f` command.

**Q.** Are worker node upgrades handled differently from control plane node upgrades?

**A.** Short answer: no.

Long answer:  Both node types follow the same set procedure.
From the user's standpoint, however, the processes are identical.
However, since control plane nodes run additional services, such as etcd, there are some extra steps and checks performed on them.
For instance, Talos will refuse to upgrade a control plane node if that upgrade would cause a loss of quorum for etcd.
If multiple control plane nodes are asked to upgrade at the same time, Talos will protect the Kubernetes cluster by ensuring only one control plane node actively upgrades at any time, via checking etcd quorum.
If running a single-node cluster, and you want to force an upgrade despite the loss of quorum, you can set `preserve` to `true`.

**Q.** Can I break my cluster by upgrading everything at once?

**A.** Maybe - it's not recommended.

Nothing prevents the user from sending near-simultaneous upgrades to each node of the cluster - and while Talos Linux and Kubernetes can generally deal with this situation, other components of the cluster may not be able to recover from more than one node rebooting at a time.
(e.g. any software that maintains a quorum or state across nodes, such as Rook/Ceph)
