---
title: "Local Storage"
description: "Using local storage for Kubernetes workloads."
---

Using local storage for Kubernetes workloads implies that the pod will be bound to the node where the local storage is available.
Local storage is not replicated, so in case of a machine failure contents of the local storage will be lost.

> Note: when using `EPHEMERAL` Talos partition (`/var`), make sure to use `--preserve` set while performing upgrades, otherwise you risk losing data.

## `hostPath` mounts

The simplest way to use local storage is to use `hostPath` mounts.
When using `hostPath` mounts, make sure the root directory of the mount is mounted into the `kubelet` container:

```yaml
machine:
  kubelet:
    extraMounts:
      - destination: /var/mnt
        type: bind
        source: /var/mnt
        options:
          - bind
          - rshared
          - rw
```

Both `EPHEMERAL` partition and user disks can be used for `hostPath` mounts.

## Local Path Provisioner

[Local Path Provisioner](https://github.com/rancher/local-path-provisioner) can be used to dynamically provision local storage.
Make sure to update its configuration to use a path under `/var`, e.g. `/var/local-path-provisioner` as the root path for the local storage.
(In Talos Linux default local path provisioner path `/opt/local-path-provisioner` is read-only).
