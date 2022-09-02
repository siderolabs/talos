---
title: "Concepts"
weight: 30
description: "Summary of Talos Linux."
---

When people come across Talos, they frequently want a nice, bite-sized summary
of it.
This is surprisingly difficult when Talos represents such a
fundamentally-rethought operating system.

## Not based on X distro

A useful way to summarize an operating system is to say that it is based on X, but focused on Y.
For instance, Mint was originally based on Ubuntu, but focused on Gnome 2 (instead of, at the time, Unity).
Or maybe something like Raspbian is based on Debian, but it is focused on the Raspberry Pi.
CentOS is RHEL, but made license-free.

Talos Linux _isn't_ based on any other distribution.
We often think of ourselves as being the second-generation of
container-optimised operating systems, where things like CoreOS, Flatcar, and Rancher represent the first generation, but that implies heredity where there is none.

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
_Unlike_ other operating systems, Talos is not meant to run alone, on a
single machine.
Talos Linux comes with tooling from the very foundation to form clusters, even
before Kubernetes comes into play.
A design goal of Talos Linux is eliminating the management
of individual nodes as much as possible.
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
