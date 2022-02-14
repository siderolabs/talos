---
title: Docker
description: "Creating Talos Kubernetes cluster using Docker."
---

In this guide we will create a Kubernetes cluster in Docker, using a containerized version of Talos.

Running Talos in Docker is intended to be used in CI pipelines, and local testing when you need a quick and easy cluster.
Furthermore, if you are running Talos in production, it provides an excellent way for developers to develop against the same version of Talos.

## Requirements

The follow are requirements for running Talos in Docker:

- Docker 18.03 or greater
- a recent version of [`talosctl`](https://github.com/talos-systems/talos/releases)

## Caveats

Due to the fact that Talos will be running in a container, certain APIs are not available.
For example `upgrade`, `reset`, and similar APIs don't apply in container mode.
Further, when running on a Mac in docker,  due to networking limitations, VIPs are not supported.

## Create the Cluster

Creating a local cluster is as simple as:

```bash
talosctl cluster create --wait
```

Once the above finishes successfully, your talosconfig(`~/.talos/config`) will be configured to point to the new cluster.

> Note: Startup times can take up to a minute or more before the cluster is available.

Finally, we just need to specify which nodes you want to communicate with using talosctl.
Talosctl can operate on one or all the nodes in the cluster – this makes cluster wide commands much easier.

`talosctl config nodes 10.5.0.2 10.5.0.3`

## Using the Cluster

Once the cluster is available, you can make use of `talosctl` and `kubectl` to interact with the cluster.
For example, to view current running containers, run `talosctl containers` for a list of containers in the `system` namespace, or `talosctl containers -k` for the `k8s.io` namespace.
To view the logs of a container, use `talosctl logs <container>` or `talosctl logs -k <container>`.

## Cleaning Up

To cleanup, run:

```bash
talosctl cluster destroy
```
