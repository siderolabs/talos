---
title: "Containerd"
description: "Customize Containerd Settings"
aliases:
  - ../../guides/configuring-containerd
---

The base containerd configuration expects to merge in any additional configs present in `/etc/cri/conf.d/20-customization.part`.

## Examples

### Exposing Metrics

Patch the machine config by adding the following:

```yaml
machine:
  files:
    - content: |
        [metrics]
          address = "0.0.0.0:11234"
      path: /etc/cri/conf.d/20-customization.part
      op: create
```

Once the server reboots, metrics are now available:

```bash
$ curl ${IP}:11234/v1/metrics
# HELP container_blkio_io_service_bytes_recursive_bytes The blkio io service bytes recursive
# TYPE container_blkio_io_service_bytes_recursive_bytes gauge
container_blkio_io_service_bytes_recursive_bytes{container_id="0677d73196f5f4be1d408aab1c4125cf9e6c458a4bea39e590ac779709ffbe14",device="/dev/dm-0",major="253",minor="0",namespace="k8s.io",op="Async"} 0
container_blkio_io_service_bytes_recursive_bytes{container_id="0677d73196f5f4be1d408aab1c4125cf9e6c458a4bea39e590ac779709ffbe14",device="/dev/dm-0",major="253",minor="0",namespace="k8s.io",op="Discard"} 0
...
...
```

### Pause Image

This change is often required for air-gapped environments, as `containerd` CRI plugin has a reference to the `pause` image which is used
to create pods, and it can't be controlled with Kubernetes pod definitions.

```yaml
machine:
  files:
    - content: |
        [plugins]
          [plugins."io.containerd.grpc.v1.cri"]
            sandbox_image = "registry.k8s.io/pause:3.8"
      path: /etc/cri/conf.d/20-customization.part
      op: create
```

Now the `pause` image is set to `registry.k8s.io/pause:3.8`:

```bash
$ talosctl containers --kubernetes
NODE         NAMESPACE   ID                                                              IMAGE                                                      PID    STATUS
172.20.0.5   k8s.io      kube-system/kube-flannel-6hfck                                  registry.k8s.io/pause:3.8                                  1773   SANDBOX_READY
172.20.0.5   k8s.io      └─ kube-system/kube-flannel-6hfck:install-cni:bc39fec3cbac      ghcr.io/siderolabs/install-cni:v1.3.0-alpha.0-2-gb155fa0   0      CONTAINER_EXITED
172.20.0.5   k8s.io      └─ kube-system/kube-flannel-6hfck:install-config:5c3989353b98   ghcr.io/siderolabs/flannel:v0.20.1                         0      CONTAINER_EXITED
172.20.0.5   k8s.io      └─ kube-system/kube-flannel-6hfck:kube-flannel:116c67b50da8     ghcr.io/siderolabs/flannel:v0.20.1                         2092   CONTAINER_RUNNING
172.20.0.5   k8s.io      kube-system/kube-proxy-xp7jq                                    registry.k8s.io/pause:3.8                                  1780   SANDBOX_READY
172.20.0.5   k8s.io      └─ kube-system/kube-proxy-xp7jq:kube-proxy:84fc77c59e17         registry.k8s.io/kube-proxy:v1.26.0-alpha.3                 1843   CONTAINER_RUNNING
```
