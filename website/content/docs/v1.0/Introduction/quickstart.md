---
title: Quickstart
weight: 2
---

The easiest way to try Talos is by using the CLI (`talosctl`) to create a cluster on a machine with `docker` installed.

## Prerequisites

### `talosctl`

Download `talosctl`:

#### `amd64`

```bash
curl -Lo /usr/local/bin/talosctl https://github.com/talos-systems/talos/releases/latest/download/talosctl-$(uname -s | tr "[:upper:]" "[:lower:]")-amd64
chmod +x /usr/local/bin/talosctl
```

#### `arm64`

For `linux` and `darwin` operating systems `talosctl` is also available for the `arm64` processor architecture.

```bash
curl -Lo /usr/local/bin/talosctl https://github.com/talos-systems/talos/releases/latest/download/talosctl-$(uname -s | tr "[:upper:]" "[:lower:]")-arm64
chmod +x /usr/local/bin/talosctl
```

### `kubectl`

Download `kubectl` via one of methods outlined in the [documentation](https://kubernetes.io/docs/tasks/tools/install-kubectl/).

## Create the Cluster

Now run the following:

```bash
talosctl cluster create
```

Verify that you can reach Kubernetes:

```bash
$ kubectl get nodes -o wide
NAME                     STATUS   ROLES    AGE    VERSION   INTERNAL-IP   EXTERNAL-IP   OS-IMAGE         KERNEL-VERSION   CONTAINER-RUNTIME
talos-default-master-1   Ready    master   115s   v1.20.2   10.5.0.2      <none>        Talos (v0.15.0)   <host kernel>    containerd://1.5.5
talos-default-worker-1   Ready    <none>   115s   v1.20.2   10.5.0.3      <none>        Talos (v0.15.0)   <host kernel>    containerd://1.5.5
```

## Destroy the Cluster

When you are all done, remove the cluster:

```bash
talosctl cluster destroy
```
