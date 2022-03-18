---
title: Philosophy
weight: 1
---

## Distributed

Talos is intended to be operated in a distributed manner.
That is, it is built for a high-availability dataplane _first_.
Its `etcd` cluster is built in an ad-hoc manner, with each appointed node joining on its own directive (with proper security validations enforced, of course).
Like as kubernetes itself, workloads are intended to be distributed across any number of compute nodes.

There should be no single points of failure, and the level of required coordination is as low as each platform allows.

## Immutable

Talos takes immutability very seriously.
Talos itself, even when installed on a disk, always runs from a SquashFS image, meaning that even if a directory is mounted to be writable, the image itself is never modified.
All images are signed and delivered as single, versioned files.
We can always run integrity checks on our image to verify that it has not been modified.

While Talos does allow a few, highly-controlled write points to the filesystem, we strive to make them as non-unique and non-critical as possible.
In fact, we call the writable partition the "ephemeral" partition precisely because we want to make sure none of us ever uses it for unique, non-replicated, non-recreatable data.
Thus, if all else fails, we can always wipe the disk and get back up and running.

## Minimal

We are always trying to reduce and keep small Talos' footprint.
Because nearly the entire OS is built from scratch in Go, we are already
starting out in a good position.
We have no shell.
We have no SSH.
We have none of the GNU utilities, not even a rollup tool such as busybox.
Everything which is included in Talos is there because it is necessary, and
nothing is included which isn't.

As a result, the OS right now produces a SquashFS image size of less than **80 MB**.

## Ephemeral

Everything Talos writes to its disk is either replicated or reconstructable.
Since the controlplane is high availability, the loss of any node will cause
neither service disruption nor loss of data.
No writes are even allowed to the vast majority of the filesystem.
We even call the writable partition "ephemeral" to keep this idea always in
focus.

## Secure

Talos has always been designed with security in mind.
With its immutability, its minimalism, its signing, and its componenture, we are
able to simply bypass huge classes of vulnerabilities.
Moreover, because of the way we have designed Talos, we are able to take
advantage of a number of additional settings, such as the recommendations of the Kernel Self Protection Project (kspp) and the complete disablement of dynamic modules.

There are no passwords in Talos.
All networked communication is encrypted and key-authenticated.
The Talos certificates are short-lived and automatically-rotating.
Kubernetes is always constructed with its own separate PKI structure which is
enforced.

## Declarative

Everything which can be configured in Talos is done so through a single YAML
manifest.
There is no scripting and no procedural steps.
Everything is defined by the one declarative YAML file.
This configuration includes that of both Talos itself and the Kubernetes which
it forms.

This is achievable because Talos is tightly focused to do one thing: run
kubernetes, in the easiest, most secure, most reliable way it can.
