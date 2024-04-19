---
title: Philosophy
weight: 10
description: "Learn about the philosophy behind the need for Talos Linux."
---

## Distributed

Talos is intended to be operated in a distributed manner: it is built for a high-availability dataplane _first_.
Its `etcd` cluster is built in an ad-hoc manner, with each appointed node joining on its own directive (with proper security validations enforced, of course).
Like Kubernetes, workloads are intended to be distributed across any number of compute nodes.

There should be no single points of failure, and the level of required coordination is as low as each platform allows.

## Immutable

Talos takes immutability very seriously.
Talos itself, even when installed on a disk, always runs from a SquashFS image, meaning that even if a directory is mounted to be writable, the image itself is never modified.
All images are signed and delivered as single, versioned files.
We can always run integrity checks on our image to verify that it has not been modified.

While Talos does allow a few, highly-controlled write points to the filesystem, we strive to make them as non-unique and non-critical as possible.
We call the writable partition the "ephemeral" partition precisely because we want to make sure none of us ever uses it for unique, non-replicated, non-recreatable data.
Thus, if all else fails, we can always wipe the disk and get back up and running.

## Minimal

We are always trying to reduce Talos' footprint.
Because nearly the entire OS is built from scratch in Go, we are
in a good position.
We have no shell.
We have no SSH.
We have none of the GNU utilities, not even a rollup tool such as busybox.
Everything in Talos is there because it is necessary, and
nothing is included which isn't.

As a result, the OS right now produces a SquashFS image size of less than **80 MB**.

## Ephemeral

Everything Talos writes to its disk is either replicated or reconstructable.
Since the controlplane is highly available, the loss of any node will cause
neither service disruption nor loss of data.
No writes are even allowed to the vast majority of the filesystem.
We even call the writable partition "ephemeral" to keep this idea always in
focus.

## Secure

Talos has always been designed with security in mind.
With its immutability, its minimalism, its signing, and its componenture, we are
able to simply bypass huge classes of vulnerabilities.
Moreover, because of the way we have designed Talos, we are able to take
advantage of a number of additional settings, such as the recommendations of the Kernel Self Protection Project (kspp) and completely disabling dynamic modules.

There are no passwords in Talos.
All networked communication is encrypted and key-authenticated.
The Talos certificates are short-lived and automatically-rotating.
Kubernetes is always constructed with its own separate PKI structure which is
enforced.

## Declarative

Everything which can be configured in Talos is done through a single YAML
manifest.
There is no scripting and no procedural steps.
Everything is defined by the one declarative YAML file.
This configuration includes that of both Talos itself and the Kubernetes which
it forms.

This is achievable because Talos is tightly focused to do one thing: run
Kubernetes, in the easiest, most secure, most reliable way it can.

## Not based on X distro

Talos Linux _isn't_ based on any other distribution.
We think of ourselves as being the second-generation of
container-optimised operating systems, where things like CoreOS, Flatcar, and Rancher represent the first generation (but the technology is not derived from any of those.)

Talos Linux is actually a ground-up rewrite of the userspace, from PID 1.
We run the Linux kernel, but everything downstream of that is our own custom
code, written in Go, rigorously-tested, and published as an immutable,
integrated image.
The Linux kernel launches what we call `machined`, for instance, not `systemd`.
There is no `systemd` on our system.
There are no GNU utilities, no shell, no SSH, no packages, nothing you could associate with
any other distribution.

## An Operating System designed for Kubernetes

Technically, Talos Linux installs to a computer like any other operating system.
_Unlike_ other operating systems, Talos is not meant to run alone, on a
single machine.
A design goal of Talos Linux is eliminating the management
of individual nodes as much as possible.
In order to do that, Talos Linux operates as a cluster of machines, with lots of
checking and coordination between them, at all levels.

There is only a cluster.
Talos is meant to do one thing:  maintain a Kubernetes cluster, and it does this
very, very well.

The entirety of the configuration of any machine is specified by a single
configuration file, which can often be the _same_ configuration file used
across _many_ machines.
Much like a biological system, if some component misbehaves, just cut it out and
let a replacement grow.
Rebuilds of Talos are remarkably fast, whether they be new machines, upgrades,
or reinstalls.
Never get hung up on an individual machine.
