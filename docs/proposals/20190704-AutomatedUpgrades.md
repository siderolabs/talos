# Proposal: Automated Upgrades

Author(s): [Andrew Rynhard](@andrewrynhard)

## Abstract

In the next version of Talos we aim to make it completely ephemeral by:

- having an extremely small footprint, allowing for the `rootfs` to run purely in memory
- formatting the owned block device on upgrades
- performing upgrades from a container

## Background

Currently, Talos bootstraps a machine in the `initramfs`.
In this stage the required partitions are created, if necessary, and mounted.
This design requires four partitions:

- a boot partition
- two root partitions
  - `ROOT-A`
  - `ROOT-B`
- and a data partition

Our immutable approach necessitates two root partitions in order to perform an upgrade.
These two partitions are toggled on each upgrade (e.g. When upgrading while running `ROOT-A`, the `ROOT-B` partition if used for the new install and then targeted on the next boot).
By requiring two root partitions, the artifacts produced are sizable (~5GB), making our footprint bigger than we need.

In addition to the above, we currently run `init` in the `initramfs` and then perform a `switch_root` to the mounted `rootfs` and then execute `/self/proc/exe` to pass control to the same version of `init` but with a flag that indicates we should run `rootfs` specific code.
By executing `/self/proc/exe` we run into a number of limitations:

- tightly coupled `initramfs` and `rootfs` PID 1 processes
- requires a reboot upon upgrading
- `init` is bloated

> Note: There are a number of other tasks performed in the `initramfs` but I will spare the details as they don't have much influence on the design outlined in this proposal.

## Proposal

This proposal introduces the idea of breaking apart `init` into two separate binaries with more focused scopes.
I propose we call these binaries `init` (runs in `initramfs`) and `machined` (runs in `rootfs`).
Additionally, optimizations in the `rootfs` size, and container based upgrades are proposed.

> Note: I'm also playing with the name `noded` instead of `machined`.
> Either way I'd like our nomenclature to align throughout Talos (e.g. nodeconfig, machineconfig).

### Splitting `init` into `init` and `machined`

By splitting the current implementation of `init` into two distincts binaries we have the potential to avoid reboots on upgrades.
Since our current `init` code performs tasks in the early boot stage, _and_ it acts is PID 1 once running the `rootfs`, we must always reboot in order run a new version of Talos.
Aside from these benefits, its just better in general to have more focused scope.

#### The Scope of `init`

Splitting our current `init` into two doesn't completely remove the need for a reboot, but it does reduce the chances that a reboot is required.
We can decrease these chances by having the new `init` perform only the following tasks:

- mount the special filesystems (e.g. `/dev`, `/proc`, `/sys`, etc.)
- mount the `rootfs` (a `squashfs` filesystem) using a loopback device
- start `machined`

By stripping `init` down to the bare minimum to start the `rootfs`, we reduce the chances that we introduce a change that requires upgrading `init`.
Additionally, on upgrades, we can return control back to `init` to perform a teardown of the node and reutrn it back to a state that is the same as when the machine was freshly booted.
This provides a clean slate for upgrades.

#### The Scope of `machined`

Once we enter the `rootfs` we have three high level tasks:

- retreive the machineconfig
- create, format, and mount partitions per the builtin specifications
- start system and k8s.io services

By having a versioned `machined` that is decoupled from `initramfs` we can encapsulate a version of Talos with one binary that knows how to run Talos.
It will know how to handle versioned machineconfigs and migrate them to newer formats.
If we ever decide to change paritition layout, filesystem types, or anything at the blockdevice level, the versioned `machined` will know how to handle that too.
The same can be said about the services that `machined` is responsible for.

### Running `rootfs` in Memory

To take nodes one step closer to becoming completely ephemeral, we will run the `rootfs` in memory.
This is part of the solution to remove the need for `ROOT-A` and `ROOT-B` partitions.
It also allows us to dedicate the owned block device to data that can be reproduced (e.g. containerd data, kubelet data, etcd data, CNI data, etc.).

### Using `squashfs` for the `rootfs`

For performance reasons, and a smaller footprint, we can create our `rootfs` as a `squashfs` image.
The way in which we can run the `rootfs` is by mounting it using a loopback device.
Once mounted, we have will perform a `switch_root` into the mounted `rootfs`.

> Note: A benefit of `squashfs` is that it allows us to retain our read only `rootfs` without having to mount it as such.

### Slimming down `rootfs`

To slim down the size of our rootfs, I propose that we remove Kubernetes tarballs entirely and let the `kubelet` download them.
By doing this we can embrace configurability at the Kubernetes layer.
The disadvantage to this is that this introduces a number a variables in the number of things that can go wrong.
Having "approved" tarballs packaged with Talos allows us to run conformance tests against exactly what users will use.
That being said, I think this is overall a better direction for Talos.

### A New Partition Scheme

With the move to run the `rootfs` in memory, we will remove the need for `ROOT-A` and `ROOT-B` partitions.
In the case of cloud installs, or where users may want to boot without PXE/iPXE, we will require a boot partition.
In all cases we require a data partition.

I also propose we rename the `DATA` partition to `EPHEMERAL` to make it clear that this partition should not be used to store application data.
Our suggestion to users who do want persistent storage on the host is to mount another disk and to use that.
Talos will make the guarantee to only manage the "owned" block device.

### Performing Upgrades in a Container

The move to running `rootfs` in memory, and thus making the block device ephemeral, and the split of `init` into `init` and `machined`, allows us to take an approach to upgrades that is clean, and relatively low risk.
The workflow will look roughly like the following:

- an upgrade request is received by `osd` and proxied to `machined`
- `machined` stops `system` and `k8s.io` services
- `machined` unmounts all mounts in `/var`
- `machined` runs the specified installer container

### Automated Upgrades

All of the aformentioned improvements help in the higher goal of automated upgrades.
Once the above is implemented, upgrades should be simple and self-contained to a machine.
By running upgrades in containers, we can have very simple operator implementation that can rollout an upgrade across a cluster by simply:

- checking for a new tag
- orchestrating cluster-wide rollout

The operator will initially coordinate upgrades only, but it will also provide a basis for a number of future automation opportunities.

#### Release Channels

We will maintain a service that allows for the operator to discover when new releases of Talos are made available.
There will be three "channels" that a machine can subscribe to:

- alpha
- beta
- stable

The implementation details around this service and its' interaction with the operator is not in the scope of this proposal.

#### Workflow

In order to upgrade nodes, the operator will subscribe to events for nodes.
It will keep an in-memory list of all nodes.

The operator will periodically query the target registry for a list of containers within a time range.
The time range should start where a previous query has ended.
The initial query should use the currently installed image's timestamp to decide where to start.

In order to supply enough information to the operator, such that it can make an informed decision about using a particular container, we will make use of labels.
With labels we can communicate to the operator that a container is:

1. an official release
2. within a given channel
3. contains a version that is compatible with the current cluster

There may be more information we end up needing, the above is just an example.

Once a container has been chosen, the operator will then create a schedule based on polices.
The initial policies will perform an upgrade based on:

1. x% of nodes at a given time
2. availability of a new image in a given channel
3. time of day
4. machines per unit of time (e.g. one machine a day, two machines per hour, etc.)
5. strategy:
    - In-place
    - Replace

A user should be able to apply more than one policy.

Once the operator has decided scheduling, it will use the upgrade RPC to perform an upgrades according to the calculated upgrade schedule.

## Rationale

One of the goals in Talos is to allow for machines to be thought of as a container in that it is ephemeral, reproducable, and predictable.
Since the `kubelet` can reproduce a machine's workload, it is not too crazy to think that we can facilitate upgrades by completely wiping a node, and then performing a fresh install as if the machine is new.

## Compatibility

This change introduces backwards incompatible changes.
The way in which `init` will be ran, and upgrades will be performed, are fundamentally different.

## Implementation

- slim down the `rootfs` by removing Kubernetes tarballs (@andrewrynhard due by v0.2)
- move `rootfs` from tarball to `squashfs` (@andrewrynhard due by v0.2)
- split `init` into `init` and `machined` (@andrewrynhard due by v0.2)
- run `rootfs` in memory (@andrewrynhard due by v0.2)
- use new design for installs and upgrades (@andrewrynhard due by v0.2)
- implement channel service (due by v0.3)
- implement the operator (@andrewrynhard due by v0.3)

## Open issues (if applicable)
