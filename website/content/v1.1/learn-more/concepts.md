---
title: "Concepts"
weight: 20
---

When people come across Talos, they frequently want a nice, bite-sized summary
of it.
Even better would be if we could give them a reference by which to extrapolate
what Talos is from something they already know.
This is surprisingly difficult when Talos represents such a
fundamentally-rethought operating system.

## Not based on X distro

A really easy (and useful!) way to summarize an operating system is to say that it is based on X, but focused on Y.
For instance, Mint was originally based on Ubuntu, but focused on Gnome 2 (instead of, at the time, Unity).
Or maybe something like Raspbian is based on Debian, but it is focused on the Raspberry Pi.
CentOS is RHEL, but made license-free.

Talos Linux _isn't_ based on any other distribution, so there's no help here.
We often think of ourselves as being the second-generation of
container-optimised operating systems, where things like CoreOS, Flatcar, and Rancher represent the first generation, but that implies heredity where there is none.
It does, though, allow a conceptual handle to the concept.

Talos Linux is actually a ground-up rewrite of the userspace, from PID 1.
We run the Linux kernel, but everything downstream of that is our own custom
code, written in Go, rigorously-tested, and published as an immutable,
integrated, cohesive image.
The Linux kernel launches what we call `machined`, for instance, not `systemd`.
There is no `systemd` on our system.
There are no GNU utilities, no shell, no SSH, no packages, nothing you could associate with
any other distribution.
We don't even have a build toolchain in the normal sense of the word.

## Not for individual use

Technically, Talos Linux installs to a computer much as other operating systems.
_Unlike_ other operating systems, however, Talos is not meant to run alone, on a
single machine.
Talos Linux comes with tooling from the very foundation to form clusters, even
before Kubernetes comes into play.
A design goal of Talos Linux is to come as close to eliminating the management
of individual nodes as possible.
In order to do that, Talos Linux operates as a cluster of machines, with lots of
checking and coordination between them, at all levels.

Break from your mind the idea of running an application on a computer.
There are no individual computers.
There is only a cluster.
Talos is meant to do one thing:  maintain a Kubernetes cluster, and it does this
very, very well.

The entirety of the configuration of any machine is specified by a single,
simple configuration file, which can often be the _same_ configuration file used
across _many_ machines.
Much like a biological system, if some component misbehaves, just cut it out and
let a replacement grow.
Rebuilds of Talos are remarkably fast, whether they be new machines, upgrades,
or reinstalls.
Never get hung up on an individual machine.

## Control Planes are not linear replicas

People familiar with traditional relational database replication tactics often
overlook a critical design concept of the Kubernetes (and Talos) database:
`etcd`.
Unlike linear replicas, which have dedicated masters and slaves/replicas, `etcd`
is highly dynamic.
The `master` in an `etcd` cluster is entirely temporal.
This means fail-overs are handled easily, often, and usually without any notice
of operators.
This _also_ means that the operational architecture is fundamentally different.

Properly managed (which Talos Linux does), `etcd` should never have split brain
and should never encounter noticeable down time.
In order to do this, though, `etcd` maintains the concept of "membership" and of
"quorum".
In order to perform _any_ operation, read _or_ write, the database requires
quorum to be sustained.
That is, a _strict_ majority must agree on the current leader, and absenteeism
counts as a negative.
In other words, if there are three registered members (voters), at least two out
of the three must be actively asserting that the current master _is_ the master.
If any two disagree or even fail to answer, the `etcd` database will lock itself
until quorum is again achieved in order to protect itself and the integrity of
the data.
This is fantastically important for handling distributed systems and the various
types of contention which may arise therein.

This design means, however, that having an incorrect number of members can be
devastating.
Having only two controlplane nodes, for instance, is mostly _worse_ than having
only one, because if _either_ goes down, your entire database will lock.
You would be better off just making periodic snapshots of the data and restoring
it when necessary.

Another common situation occurs when replacing controlplane nodes.
If you have three controlplane nodes and replace one, you will not have three
members, you will have four, and one of those will never be available again.
Thus, if _any_ of your three remaining nodes goes down, your database will lock,
because only two out of the four members will be available:  four nodes is
_worse_ than three nodes!
So it is critical that controlplane members which are replaced be removed.
Luckily, the Talos API makes this easy.

## Bootstrap once

In the old days, Talos Linux had the idea of an `init` node.
The `init` node was a "special" controlplane node which was designated as the
founder of the cluster.
It was the first, was guaranteed to be the elector, and was authorised to create
a cluster...
even if one already existed.
This made the formation of a cluster cluster really easy, but it had a lot of
down sides.
Mostly, these related to rebuilding or replacing that `init` node:
you could easily end up with a split-brain scenario in which you had two different clusters:
a single node one and a two-node one.
Needless to say, this was an unhappy arrangement.

Fortunately, `init` nodes are gone, but that means that the critical operation
of forming a cluster is a manual process.
It's an _easy_ process, consisting of a single API call, but it can be a
confusing one, until you understand what it does.

Every new cluster must be bootstrapped exactly and only once.
This means you do NOT bootstrap each node in a cluster, not even each
controlplane node.
You bootstrap only a _single_ controlplane node, because you are bootstrapping the
_cluster_, not the node.

It doesn't matter _which_ controlplane node is told to bootstrap, but it must be
a controlplane node, and it must be only one.

Bootstrapping is _fast_ and sure.
Even if your Kubernetes cluster fails to form for other reasons (say, a bad
configuration option or unavailable container repository), if the bootstrap API
call returns successfully, you do NOT need to bootstrap again:
just fix the config or let Kubernetes retry.

Bootstrapping itself does not do anything with Kubernetes.
Bootstrapping only tells `etcd` to form a cluster, so don't judge the success of
a bootstrap by the failure of Kubernetes to start.
Kubernetes relies on `etcd`, so bootstrapping is _required_, but it is not
_sufficient_ for Kubernetes to start.

[comment]: <>(!-- TODO: how to check if a cluster has already been bootstrapped
successfully.)
