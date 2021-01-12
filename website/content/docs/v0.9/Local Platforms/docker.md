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

Due to the fact that Talos runs in a container, certain APIs are not available when running in Docker.
For example `upgrade`, `reset`, and APIs like these don't apply in container mode.

## Create the Cluster

Creating a local cluster is as simple as:

```bash
talosctl cluster create --wait
```

Once the above finishes successfully, your talosconfig(`~/.talos/config`) will be configured to point to the new cluster.

If you are running on MacOS, an additional command is required:

```bash
talosctl config --endpoints 127.0.0.1
```

> Note: Startup times can take up to a minute before the cluster is available.

## Retrieve and Configure the `kubeconfig`

```bash
talosctl kubeconfig .
kubectl --kubeconfig kubeconfig config set-cluster talos-default --server https://127.0.0.1:6443
```

## Using the Cluster

Once the cluster is available, you can make use of `talosctl` and `kubectl` to interact with the cluster.
For example, to view current running containers, run `talosctl containers` for a list of containers in the `system` namespace, or `talosctl containers -k` for the `k8s.io` namespace.
To view the logs of a container, use `talosctl logs <container>` or `talosctl logs -k <container>`.

## Cleaning Up

To cleanup, run:

```bash
talosctl cluster destroy
```
