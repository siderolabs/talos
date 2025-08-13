---
title: "Storage"
description: "Setting up storage for a Kubernetes cluster"
aliases:
  - ../../guides/storage
---

In Kubernetes, using storage in the right way is well-facilitated by the API.
However, unless you are running in a major public cloud, that API may not be hooked up to anything.
There are a _lot_ of options out there, and it can be fairly bewildering.

For Talos, we have some recommendations to make the decision easier.

## Public Cloud

If you are running on a major public cloud, use their block storage.
It is easy and automatic.

## Storage Clusters

> **Sidero Labs** recommends having separate disks (separate from the Talos install disk) dedicated for storage.

Redundancy, scaling capabilities, reliability, speed, maintenance load, and ease of use are all factors you must consider when managing your own storage.

Running a storage cluster can be a very good choice when managing your own storage.
The following projects are known to work with Talos Linux and provide good options, depending on your situation.

**MayaStor**: Ultra-low latency and high-performance workloads.

**Longhorn**: Simple, reliable, easy-to-use Kubernetes storage with easy replication and snapshots.

**Rook/Ceph**: Enterprise-scale, distributed, multi-tenant storage (block, file, and object storage)

Also, if you need _both_ mount-once _and_ mount-many capabilities, Ceph is your answer.

> Please note that _most_ people should not use mount-many semantics.
> NFS is pervasive because it is old and easy, _not_ because it is a good idea.
> There are all manner of locking, performance, change control, and reliability concerns inherent in _any_ mount-many situation, so we **strongly** recommend you avoid this method.

### Longhorn

Documentation for installing Longhorn on Talos Linux is available on the [Longhorn site](https://longhorn.io/docs/1.9.0/advanced-resources/os-distro-specific/talos-linux-support/).

### Rook/Ceph

[Ceph](https://ceph.io) is a mature open source storage system, that can provide almost any type of storage.
It scales well, and enables the operator to easily add and remove storage with no downtime.
It comes bundled with an S3-compatible object store; CephFS, a NFS-like clustered filesystem; and RBD, a block storage system.

With the help of [Rook](https://rook.io), the vast majority of the complexity of Ceph is hidden away, allowing you to control almost everything about your Ceph cluster from fairly simple Kubernetes CRDs.

However, Ceph can be rather slow for small clusters.
It relies heavily on CPUs and massive parallelization for performance.
If your cluster is small, just running Ceph may eat up a significant amount of the resources you have available.

Troubleshooting Ceph can be difficult if you do not understand its architecture.
There are very good tools for inspection and debugging, but this is still frequently seen as a concern.

### OpenEBS Mayastor replicated storage

[Mayastor](https://github.com/openebs/Mayastor) is an OpenEBS project built in Rust utilizing the modern NVMEoF system.

#### Deploy Mayastor

Mayastor has documentation specific to installing on Talos Linux in their official [documentation](https://openebs.io/docs/Solutioning/openebs-on-kubernetes-platforms/talos)

Installing on Talos Linux requires patching the Pod Security policies, enabling Huge Page support, and labels.
This is all covered in the Mayastor documentation,

We need to disable the init container that checks for the `nvme_tcp` module, since Talos has that module built-in.

Create a helm values file `mayastor-values.yaml` with the following contents:

```yaml
mayastor:
  csi:
    node:
      initContainers:
        enabled: false
```

If you do not need to use the LVM and ZFS engines they can be disabled in the values file:

```yaml
engines:
  local:
    lvm:
      enabled: false
    zfs:
      enabled: false
```

Continue setting up [Mayastor](https://openebs.io/docs/quickstart-guide/installation#installation-via-helm) using the official documentation, passing the values file.

Follow the Post-Installation from official [documentation](https://openebs.io/docs/quickstart-guide/installation#post-installation-considerations) to use Local Storage or Replicated Storage.

### Piraeus / LINSTOR

* [Piraeus-Operator](https://piraeus.io/)
* [LINSTOR](https://linbit.com/drbd/)
* [DRBD Extension](https://github.com/siderolabs/extensions#storage)

#### Install Piraeus Operator V2

There is already a how-to for Talos: [Link](https://piraeus.io/docs/stable/how-to/talos/)

#### Create first storage pool and PVC

Before proceeding, install linstor plugin for kubectl:
https://github.com/piraeusdatastore/kubectl-linstor

Or use [krew](https://krew.sigs.k8s.io/): `kubectl krew install linstor`

```sh
# Create device pool on a blank (no partition table!) disk on node01
kubectl linstor physical-storage create-device-pool --pool-name nvme_lvm_pool LVM node01 /dev/nvme0n1 --storage-pool nvme_pool
```

piraeus-sc.yml

```yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: simple-nvme
parameters:
  csi.storage.k8s.io/fstype: xfs
  linstor.csi.linbit.com/autoPlace: "3"
  linstor.csi.linbit.com/storagePool: nvme_pool
provisioner: linstor.csi.linbit.com
volumeBindingMode: WaitForFirstConsumer
```

```sh
# Create storage class
kubectl apply -f piraeus-sc.yml
```

## NFS

NFS is slow, has all kinds of bottlenecks involving contention, distributed locking, single points of service, and more.
However, it is supported by a wide variety of systems, such as NetApp storage arrays.

The NFS client is part of the [`kubelet` image](https://github.com/talos-systems/kubelet) maintained by the Talos team.
This means that the version installed in your running `kubelet` is the version of NFS supported by Talos.
You can reduce some of the contention problems by parceling Persistent Volumes from separate underlying directories.

## Object storage

Ceph comes with an S3-compatible object store, but there are other options, as
well.
These can often be built on top of other storage backends.
For instance, you may have your block storage running with Mayastor but assign a
Pod a large Persistent Volume to serve your object store.

One of the most popular open source add-on object stores is [MinIO](https://min.io/).

## Others (iSCSI)

The most common remaining systems involve iSCSI in one form or another.
iSCSI in Linux is facilitated by [open-iscsi](https://github.com/open-iscsi/open-iscsi).

iSCSI support in Talos is now supported via the [iscsi-tools](https://github.com/siderolabs/extensions/pkgs/container/iscsi-tools) [system extension]({{< relref "../../talos-guides/configuration/system-extensions" >}}) installed.
