---
title: "Resources"
description: "Resources created by Talos Linux while processing volume configuration."
weight: 200
---

This section contains information about the resources created by Talos Linux to manage volumes on the machine.
This information is useful for understanding how Talos Linux manages volumes and how to debug issues related to volumes.

## Volumes

The configuration of volumes is defined using the `VolumeConfig` resource, while the current state of volumes is stored in the `VolumeStatus` resource.

### Volume Configuration

The volume configuration is managed by Talos Linux based on machine configuration.
To see configured volumes, use the following command:

```bash
$ talosctl get volumeconfigs
NODE         NAMESPACE   TYPE           ID                                  VERSION
172.20.0.2   runtime     VolumeConfig   /etc/cni                            2
172.20.0.2   runtime     VolumeConfig   /var/run                            2
172.20.0.2   runtime     VolumeConfig   EPHEMERAL                           2
172.20.0.2   runtime     VolumeConfig   ETCD                                2
172.20.0.2   runtime     VolumeConfig   META                                2
172.20.0.2   runtime     VolumeConfig   STATE                               3
172.20.0.2   runtime     VolumeConfig   u-extra                             2
172.20.0.2   runtime     VolumeConfig   u-p1                                2
172.20.0.2   runtime     VolumeConfig   u-p2                                2
```

In the provided output, the volumes `EPHEMERAL`, `META`, and `STATE` are system volumes managed by Talos, while `u-extra`, `u-p1` and `u-p2` are user configured volumes.

To get details about a specific volume configuration, use the following command:

```yaml
# talosctl get volumeconfig STATE -o yaml
node: 172.20.0.5
metadata:
    namespace: runtime
    type: VolumeConfigs.block.talos.dev
    id: STATE
    version: 4
    owner: block.VolumeConfigController
    phase: running
    created: 2024-08-29T13:22:04Z
    updated: 2024-08-29T13:22:17Z
    finalizers:
        - block.VolumeManagerController
spec:
    type: partition
    provisioning:
        wave: -1
        diskSelector:
            match: system_disk
        partitionSpec:
            minSize: 104857600
            maxSize: 104857600
            grow: false
            label: STATE
            typeUUID: 0FC63DAF-8483-4772-8E79-3D69D8477DE4
        filesystemSpec:
            type: xfs
            label: STATE
    encryption:
        provider: luks2
        keys:
            - slot: 0
              type: nodeID
    locator:
        match: volume.partition_label == "STATE"
    mount:
        targetPath: /system/state
```

### Volume Status

Current volume status can be obtained using the following command:

```bash
$ talosctl get volumestatus
NODE         NAMESPACE   TYPE           ID                                  VERSION   TYPE        PHASE   LOCATION    SIZE
172.20.0.2   runtime     VolumeStatus   /etc/cni                            3         overlay     ready
172.20.0.2   runtime     VolumeStatus   EPHEMERAL                           6         partition   ready   /dev/vda4   5.2 GB
172.20.0.2   runtime     VolumeStatus   ETCD                                2         directory   ready
172.20.0.2   runtime     VolumeStatus   META                                3         partition   ready   /dev/vda2   1.0 MB
172.20.0.2   runtime     VolumeStatus   STATE                               6         partition   ready   /dev/vda3   105 MB
172.20.0.2   runtime     VolumeStatus   u-extra                             2         partition   ready   /dev/sda1   350 MB
172.20.0.2   runtime     VolumeStatus   u-p1                                2         partition   ready   /dev/sdb1   350 MB
172.20.0.2   runtime     VolumeStatus   u-p2                                2         partition   ready   /dev/sdb2   350 MB
```

Each volume goes through different phases during its lifecycle:

- `waiting`: the volume is waiting to be provisioned
- `missing`: all disks have been discovered, but the volume cannot be found
- `located`: the volume is found without prior provisioning
- `provisioned`: the volume has been provisioned (e.g., partitioned, resized if necessary)
- `prepared`: the encrypted volume is open
- `ready`: the volume is formatted and ready to be mounted
- `closed`: the encrypted volume is closed, and ready to be unmounted

## Mounts

Volumes are mounted when they are ready to be used, mounts are tracked in two resources: `MountRequest` describes the desired mount, while `MountStatus` describes the current state of the mount.

### Mount Request

Mount requests are created automatically by Talos Linux based on the volume configuration, service configuration, etc.

To see the current mount requests, you can use the following command:

```bash
$ talosctl get mountrequests
NODE         NAMESPACE   TYPE           ID                                  VERSION   VOLUME                              PARENT                     REQUESTERS
172.20.0.5   runtime     MountRequest   /etc/cni                            2         /etc/cni                                                       ["service/cri"]
172.20.0.5   runtime     MountRequest   /etc/kubernetes                     2         /etc/kubernetes                                                ["service/cri"]
172.20.0.5   runtime     MountRequest   /opt                                2         /opt                                                           ["service/cri"]
172.20.0.5   runtime     MountRequest   /usr/libexec/kubernetes             2         /usr/libexec/kubernetes                                        ["service/cri"]
172.20.0.5   runtime     MountRequest   /var/lib                            3         /var/lib                            EPHEMERAL                  ["service/cri","service/kubelet"]
172.20.0.5   runtime     MountRequest   /var/lib/cni                        2         /var/lib/cni                        /var/lib                   ["service/cri"]
172.20.0.5   runtime     MountRequest   /var/lib/containerd                 2         /var/lib/containerd                 /var/lib                   ["service/cri"]
172.20.0.5   runtime     MountRequest   /var/lib/kubelet                    2         /var/lib/kubelet                    /var/lib                   ["service/kubelet"]
172.20.0.5   runtime     MountRequest   /var/lib/kubelet/seccomp            2         /var/lib/kubelet/seccomp            /var/lib/kubelet           ["service/kubelet"]
172.20.0.5   runtime     MountRequest   /var/lib/kubelet/seccomp/profiles   2         /var/lib/kubelet/seccomp/profiles   /var/lib/kubelet/seccomp   ["service/kubelet"]
172.20.0.5   runtime     MountRequest   /var/log                            2         /var/log                            EPHEMERAL                  ["service/kubelet"]
172.20.0.5   runtime     MountRequest   /var/log/audit                      2         /var/log/audit                      /var/log                   ["service/kubelet"]
172.20.0.5   runtime     MountRequest   /var/log/audit/kube                 2         /var/log/audit/kube                 /var/log/audit             ["service/kubelet"]
172.20.0.5   runtime     MountRequest   /var/log/containers                 2         /var/log/containers                 /var/log                   ["service/kubelet"]
172.20.0.5   runtime     MountRequest   /var/log/pods                       2         /var/log/pods                       /var/log                   ["service/kubelet"]
172.20.0.5   runtime     MountRequest   /var/mnt                            3         /var/mnt                            EPHEMERAL                  ["block.UserVolumeConfigController","service/kubelet"]
172.20.0.5   runtime     MountRequest   /var/run                            2         /var/run                                                       ["service/cri"]
172.20.0.5   runtime     MountRequest   /var/run/lock                       2         /var/run/lock                       /var/run                   ["service/cri"]
172.20.0.5   runtime     MountRequest   EPHEMERAL                           2         EPHEMERAL                                                      ["sequencer"]
```

### Mount Status

As the volumes are mounted, the status of the mounts is updated in the `MountStatus` resource:

```bash
$ talosctl get mountstatus
NODE         NAMESPACE   TYPE          ID                                  VERSION   SOURCE      TARGET                              FILESYSTEM   VOLUME
172.20.0.5   runtime     MountStatus   /etc/cni                            2                     /etc/cni                            none         /etc/cni
172.20.0.5   runtime     MountStatus   /etc/kubernetes                     2                     /etc/kubernetes                     none         /etc/kubernetes
172.20.0.5   runtime     MountStatus   /opt                                2                     /opt                                none         /opt
172.20.0.5   runtime     MountStatus   /usr/libexec/kubernetes             2                     /usr/libexec/kubernetes             none         /usr/libexec/kubernetes
172.20.0.5   runtime     MountStatus   /var/lib                            6                     /var/lib                            none         /var/lib
172.20.0.5   runtime     MountStatus   /var/lib/cni                        2                     /var/lib/cni                        none         /var/lib/cni
172.20.0.5   runtime     MountStatus   /var/lib/containerd                 2                     /var/lib/containerd                 none         /var/lib/containerd
172.20.0.5   runtime     MountStatus   /var/lib/kubelet                    3                     /var/lib/kubelet                    none         /var/lib/kubelet
172.20.0.5   runtime     MountStatus   /var/lib/kubelet/seccomp            3                     /var/lib/kubelet/seccomp            none         /var/lib/kubelet/seccomp
172.20.0.5   runtime     MountStatus   /var/lib/kubelet/seccomp/profiles   2                     /var/lib/kubelet/seccomp/profiles   none         /var/lib/kubelet/seccomp/profiles
172.20.0.5   runtime     MountStatus   /var/log                            5                     /var/log                            none         /var/log
172.20.0.5   runtime     MountStatus   /var/log/audit                      3                     /var/log/audit                      none         /var/log/audit
172.20.0.5   runtime     MountStatus   /var/log/audit/kube                 2                     /var/log/audit/kube                 none         /var/log/audit/kube
172.20.0.5   runtime     MountStatus   /var/log/containers                 2                     /var/log/containers                 none         /var/log/containers
172.20.0.5   runtime     MountStatus   /var/log/pods                       2                     /var/log/pods                       none         /var/log/pods
172.20.0.5   runtime     MountStatus   /var/mnt                            3                     /var/mnt                            none         /var/mnt
172.20.0.5   runtime     MountStatus   /var/run                            3                     /var/run                            none         /var/run
172.20.0.5   runtime     MountStatus   /var/run/lock                       2                     /var/run/lock                       none         /var/run/lock
172.20.0.5   runtime     MountStatus   EPHEMERAL                           5         /dev/vda4   /var                                xfs          EPHEMERAL
```
