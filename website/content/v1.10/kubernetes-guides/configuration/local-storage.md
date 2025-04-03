---
title: "Local Storage"
description: "Using local storage for Kubernetes workloads."
---

Using local storage for Kubernetes workloads implies that the pod will be bound to the node where the local storage is available.
Local storage is not replicated, so in case of a machine failure contents of the local storage will be lost.

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

Both `EPHEMERAL` partition and [user volumes]({{< relref "../../talos-guides/configuration/disk-management#user-volumes" >}}) can be used for `hostPath` mounts.

## Local Path Provisioner

[Local Path Provisioner](https://github.com/rancher/local-path-provisioner) can be used to dynamically provision local storage.

First, we will create a separate [user volume]({{< relref "../../talos-guides/configuration/disk-management#user-volumes" >}}) for the Local Path Provisioner to use.
Apply the following machine configuration patch:

> Note: make sure you have [enough space]({{< relref "../../talos-guides/configuration/disk-management#disk-layout" >}}) available to provision the user volume.

```yaml
apiVersion: v1alpha1
kind: UserVolumeConfig
name: local-path-provisioner
provisioning:
  diskSelector:
    match: disk.transport == 'nvme'
  minSize: 200GB
  maxSize: 200GB
```

Make sure to update Local Path Provisioner configuration to use a the user volume path `/var/mnt/local-path-provisioner` as the root path for the local storage.

For example, Local Path Provisioner can be installed using [kustomize](https://kustomize.io/) with the following configuration:

```yaml
# kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
- github.com/rancher/local-path-provisioner/deploy?ref=v0.0.26
patches:
- patch: |-
    kind: ConfigMap
    apiVersion: v1
    metadata:
      name: local-path-config
      namespace: local-path-storage
    data:
      config.json: |-
        {
                "nodePathMap":[
                {
                        "node":"DEFAULT_PATH_FOR_NON_LISTED_NODES",
                        "paths":["/var/mnt/local-path-provisioner"]
                }
                ]
        }
- patch: |-
    apiVersion: storage.k8s.io/v1
    kind: StorageClass
    metadata:
      name: local-path
      annotations:
        storageclass.kubernetes.io/is-default-class: "true"
- patch: |-
    apiVersion: v1
    kind: Namespace
    metadata:
      name: local-path-storage
      labels:
        pod-security.kubernetes.io/enforce: privileged
```

Put `kustomization.yaml` into a new directory, and run `kustomize build | kubectl apply -f -` to install Local Path Provisioner to a Talos Linux cluster.
There are three patches applied:

* change default `/opt/local-path-provisioner` path to `/var/mnt/local-path-provisioner`
* make `local-path` storage class the default storage class (optional)
* label the `local-path-storage` namespace as privileged to allow privileged pods to be scheduled there

To test the Local Path Provisioner, you can refer to the [Usage section of the official guide](https://github.com/rancher/local-path-provisioner?tab=readme-ov-file#usage).

You can check that directories for PVCs are created on the node's filesystem with the `talosctl ls /var/mnt/local-path-provisioner` command.
