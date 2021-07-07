---
title: Upgrades
weight: 5
---

## Talos

The upgrade process for Talos, like everything else, begins with an API call.
This call tells a node the installer image to use to perform the upgrade.
Each Talos version corresponds to an installer with the same version, such that the
version of the installer is the version of Talos which will be installed.

Because Talos is image based, even at run-time, upgrading Talos is almost
exactly the same set of operations as installing Talos, with the difference that
the system has already been initialized with a configuration.

An upgrade makes use of an A-B image scheme in order to facilitate rollbacks.
This scheme retains the one previous Talos kernel and OS image following each upgrade.
If an upgrade fails to boot, Talos will roll back to the previous version.
Likewise, Talos may be manually rolled back via API (or `talosctl rollback`).
This will simply update the boot reference and reboot.

An upgrade can `preserve` data or not.
If Talos is told to NOT preserve data, it will wipe its ephemeral partition, remove itself from the etcd cluster (if it is a control node), and generally make itself as pristine as is possible.
There are likely to be changes to the default option here over time, so if your setup has a preference to one way or the other, it is better to specify it explicitly, but we try to always be "safe" with this setting.

### Sequence

When a Talos node receives the upgrade command, the first thing it does is cordon
itself in kubernetes, to avoid receiving any new workload.
It then starts to drain away its existing workload.

**NOTE**: If any of your workloads is sensitive to being shut down ungracefully, be sure to use the `lifecycle.preStop` Pod [spec](https://kubernetes.io/docs/concepts/containers/container-lifecycle-hooks/#container-hooks).

Once all of the workload Pods are drained, Talos will start shutting down its
internal processes.
If it is a control node, this will include etcd.
If `preserve` is not enabled, Talos will even leave etcd membership.
(Don't worry about this; we make sure the etcd cluster is healthy and that it will remain healthy after our node departs, before we allow this to occur.)

Once all the processes are stopped and the services are shut down, all of the
filesystems will be unmounted.
This allows Talos to produce a very clean upgrade, as close as possible to a pristine system.
We verify the disk and then perform the actual image upgrade.

Finally, we tell the bootloader to boot _once_ with the new kernel and OS image.
Then we reboot.

After the node comes back up and Talos verifies itself, it will make permanent
the bootloader change, rejoin the cluster,  and finally uncordon itself to receive new workloads.

### FAQs

**Q.** What happens if an upgrade fails?

**A.** There are many potential ways an upgrade can fail, but we always try to do
the safe thing.

The most common first failure is an invalid installer image reference.
In this case, Talos will fail to download the upgraded image and will abort the upgrade.

Sometimes, Talos is unable to successfully kill off all of the disk access points, in which case it cannot safely unmount all filesystems to effect the upgrade.
In this case, it will abort the upgrade and reboot.

It is possible (especially with test builds) that the upgraded Talos system will fail to start.
In this case, the node will be rebooted, and the bootloader will automatically use the previous Talos kernel and image, thus effectively aborting the upgrade.

Lastly, it is possible that Talos itself will upgrade successfully, start up, and rejoin the cluster but your workload will fail to run on it, for whatever reason.
This is when you would use the `talosctl rollback` command to revert back to the previous Talos version.

**Q.** Can upgrades be scheduled?

**A.** We provide the [Talos Controller Manager](https://github.com/talos-systems/talos-controller-manager) to coordinate upgrades of a cluster.
Additionally, because the upgrade sequence is API-driven, you can easily tie this in to your own business logic to schedule and coordinate your upgrades.

**Q.** Can the upgrade process be observed?

**A.** The Talos Controller Manager does this internally, watching the logs of
the node being upgraded, using the streaming log API of Talos.

You can do the same thing using the `talosctl logs --follow machined` command.

**Q.** Are worker node upgrades handled differently from control plane node upgrades?

**A.** Short answer: no.

Long answer:  Both node types follow the same set procedure.
However, since control plane nodes run additional services, such as etcd, there are some extra steps and checks performed on them.
From the user's standpoint, however, the processes are identical.

There are also additional restrictions on upgrading control plane nodes.
For instance, Talos will refuse to upgrade a control plane node if that upgrade will cause a loss of quorum for etcd.
This can generally be worked around by setting `preserve` to `true`.

**Q.** Will an upgrade try to do the whole cluster at once?
Can I break my cluster by upgrading everything?

**A.** No.

Nothing prevents the user from sending any number of near-simultaneous upgrades to each node of the cluster.
While most people would not attempt to do this, it may be the desired behaviour in certain situations.

If, however, multiple control plane nodes are asked to upgrade at the same time, Talos will protect itself by making sure only one control plane node upgrades at any time, through its checking of etcd quorum.
A lease is taken out by the winning control plane node, and no other control plane node is allowed to execute the upgrade until the lease is released and the etcd cluster is healthy and _will_ be healthy when the next node performs its upgrade.

**Q.** Is there an operator or controller which will keep my nodes updated
automatically?

**A.** Yes.

We provide the [Talos Controller Manager](https://github.com/talos-systems/talos-controller-manager) to perform this maintenance in a simple, controllable fashion.
