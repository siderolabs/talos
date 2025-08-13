---
title: Quickstart
weight: 20
description: "A short guide on setting up a simple Talos Linux cluster locally with Docker."
---

{{< youtube IO2Yo3N46nk >}}

## Local Docker Cluster

The easiest way to try Talos is by using the CLI (`talosctl`) to create a cluster on a machine with `docker` installed.

### Prerequisites

#### `talosctl`

Download `talosctl` (macOS or Linux):

```bash
brew install siderolabs/tap/talosctl
```

#### `kubectl`

Download `kubectl` via one of methods outlined in the [documentation](https://kubernetes.io/docs/tasks/tools/install-kubectl/).

### Create the Cluster

Now run the following:

```bash
talosctl cluster create
```

{{% alert title="Note" color="info" %}}
If you are using Docker Desktop on a macOS computer, if you encounter the error: *Cannot connect to the Docker daemon at unix:///var/run/docker.sock. Is the docker daemon running?* you may need to manually create the link for the Docker socket:
```sudo ln -s "$HOME/.docker/run/docker.sock" /var/run/docker.sock```

{{% /alert %}}

You can explore using Talos API commands:

```bash
talosctl dashboard --nodes 10.5.0.2
```

Verify that you can reach Kubernetes:

```bash
kubectl get nodes -o wide
NAME                           STATUS   ROLES    AGE    VERSION          INTERNAL-IP   EXTERNAL-IP   OS-IMAGE                 KERNEL-VERSION   CONTAINER-RUNTIME
talos-default-controlplane-1   Ready    master   115s   v{{< k8s_release >}}   10.5.0.2      <none>        Talos ({{< release >}})   <host kernel>    containerd://1.5.5
talos-default-worker-1         Ready    <none>   115s   v{{< k8s_release >}}   10.5.0.3      <none>        Talos ({{< release >}})   <host kernel>    containerd://1.5.5
```

### Destroy the Cluster

When you are all done, remove the cluster:

```bash
talosctl cluster destroy
```
