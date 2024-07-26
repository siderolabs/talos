---
title: Docker
description: "Creating Talos Kubernetes cluster using Docker."
aliases:
  - ../../../local-platforms/docker
---

In this guide we will create a Kubernetes cluster in Docker, using a containerized version of Talos.

Running Talos in Docker is intended to be used in CI pipelines, and local testing when you need a quick and easy cluster.
Furthermore, if you are running Talos in production, it provides an excellent way for developers to develop against the same version of Talos.

## Requirements

The follow are requirements for running Talos in Docker:

- Docker 18.03 or greater
- a recent version of [`talosctl`](https://github.com/siderolabs/talos/releases)

{{% alert title="Note" color="info" %}}
If you are using Docker Desktop on a macOS computer, and you encounter the error: *Cannot connect to the Docker daemon at unix:///var/run/docker.sock. Is the docker daemon running?* you may need to manually create the link for the Docker socket:
```sudo ln -s "$HOME/.docker/run/docker.sock" /var/run/docker.sock```

{{% /alert %}}

## Caveats

Due to the fact that Talos will be running in a container, certain APIs are not available.
For example `upgrade`, `reset`, and similar APIs don't apply in container mode.
Further, when running on a Mac in docker, due to networking limitations, VIPs are not supported.

## Create the Cluster

Creating a local cluster is as simple as:

```bash
talosctl cluster create
```

Once the above finishes successfully, your `talosconfig` (`~/.talos/config`)  and `kubeconfig` (`~/.kube/config`) will be configured to point to the new cluster.

> Note: Startup times can take up to a minute or more before the cluster is available.

Finally, we just need to specify which nodes you want to communicate with using `talosctl`.
Talosctl can operate on one or all the nodes in the cluster – this makes cluster wide commands much easier.

`talosctl config nodes 10.5.0.2 10.5.0.3`

Talos and Kubernetes API are mapped to a random port on the host machine, the retrieved `talosconfig` and `kubeconfig` are configured automatically to point to the new cluster.
Talos API endpoint can be found using `talosctl config info`:

```bash
$ talosctl config info
...
Endpoints:           127.0.0.1:38423
```

Kubernetes API endpoint is available with `talosctl cluster show`:

```bash
$ talosctl cluster show
...
KUBERNETES ENDPOINT   https://127.0.0.1:43083
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

## Multiple Clusters

Multiple Talos Linux cluster can be created on the same host, each cluster will need to have:

- a unique name (default is `talos-default`)
- a unique network CIDR (default is `10.5.0.0/24`)

To create a new cluster, run:

```bash
talosctl cluster create --name cluster2 --cidr 10.6.0.0/24
```

To destroy a specific cluster, run:

```bash
talosctl cluster destroy --name cluster2
```

To switch between clusters, use `--context` flag:

```bash
talosctl --context cluster2 version
kubectl --context admin@cluster2 get nodes
```

## Running Talos in Docker Manually

To run Talos in a container manually, run:

```bash
docker run --rm -it \
  --name tutorial \
  --hostname talos-cp \
  --read-only \
  --privileged \
  --security-opt seccomp=unconfined \
  --mount type=tmpfs,destination=/run \
  --mount type=tmpfs,destination=/system \
  --mount type=tmpfs,destination=/tmp \
  --mount type=volume,destination=/system/state \
  --mount type=volume,destination=/var \
  --mount type=volume,destination=/etc/cni \
  --mount type=volume,destination=/etc/kubernetes \
  --mount type=volume,destination=/usr/libexec/kubernetes \
  --mount type=volume,destination=/opt \
  -e PLATFORM=container \
  ghcr.io/siderolabs/talos:{{< release >}}
```

The machine configuration submitted to the container should have a [host DNS feature]({{< relref "../../../reference/configuration/v1alpha1/config#Config.machine.features.hostDNS"  >}}) enabled with `forwardKubeDNSToHost` enabled.
It is used to forward DNS requests to the resolver provided by Docker (or other container runtime).
