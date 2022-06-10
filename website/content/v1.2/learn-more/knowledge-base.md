---
title: "Knowledge Base"
weight: 1999
description: "Recipes for common configuration tasks with Talos Linux."
---

## Disabling `GracefulNodeShutdown` on a node

Talos Linux enables [Graceful Node Shutdown](https://kubernetes.io/docs/concepts/architecture/nodes/#graceful-node-shutdown) Kubernetes feature by default.

If this feature should be disabled, modify the `kubelet` part of the machine configuration with:

```yaml
machine:
  kubelet:
    extraArgs:
      feature-gates: GracefulNodeShutdown=false
    extraConfig:
      shutdownGracePeriod: 0s
      shutdownGracePeriodCriticalPods: 0s
```

## Generating Talos Linux ISO image with custom kernel arguments

Pass additional kernel arguments using `--extra-kernel-arg` flag:

```shell
$ docker run --rm -i ghcr.io/siderolabs/imager:{{< release >}} iso --arch amd64 --tar-to-stdout --extra-kernel-arg console=ttyS1 --extra-kernel-arg console=tty0 | tar xz
2022/05/25 13:18:47 copying /usr/install/amd64/vmlinuz to /mnt/boot/vmlinuz
2022/05/25 13:18:47 copying /usr/install/amd64/initramfs.xz to /mnt/boot/initramfs.xz
2022/05/25 13:18:47 creating grub.cfg
2022/05/25 13:18:47 creating ISO
```

ISO will be output to the file `talos-<arch>.iso` in the current directory.
