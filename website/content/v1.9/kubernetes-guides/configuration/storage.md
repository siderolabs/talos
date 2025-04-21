---
title: "Storage"
description: "Setting up storage for a Kubernetes cluster"
aliases:
  - ../../guides/storage
---

In Kubernetes, using storage in the right way is well-facilitated by the API.
However, unless you are running in a major public cloud, that API may not be hooked up to anything.
This frequently sends users down a rabbit hole of researching all the various options for storage backends for their platform, for Kubernetes, and for their workloads.
There are a _lot_ of options out there, and it can be fairly bewildering.

For Talos, we try to limit the options somewhat to make the decision-making easier.

## Public Cloud

If you are running on a major public cloud, use their block storage.
It is easy and automatic.

## Storage Clusters

> **Sidero Labs** recommends having separate disks (apart from the Talos install disk) to be used for storage.

Redundancy, scaling capabilities, reliability, speed, maintenance load, and ease of use are all factors you must consider when managing your own storage.

Running a storage cluster can be a very good choice when managing your own storage, and there are two projects we recommend, depending on your situation.

If you need vast amounts of storage composed of more than a dozen or so disks, we recommend you use Rook to manage Ceph.
Also, if you need _both_ mount-once _and_ mount-many capabilities, Ceph is your answer.
Ceph also bundles in an S3-compatible object store.
The down side of Ceph is that there are a lot of moving parts.

> Please note that _most_ people should _never_ use mount-many semantics.
> NFS is pervasive because it is old and easy, _not_ because it is a good idea.
> While it may seem like a convenience at first, there are all manner of locking, performance, change control, and reliability concerns inherent in _any_ mount-many situation, so we **strongly** recommend you avoid this method.

If your storage needs are small enough to not need Ceph, use Mayastor.

### Rook/Ceph

[Ceph](https://ceph.io) is the grandfather of open source storage clusters.
It is big, has a lot of pieces, and will do just about anything.
It scales better than almost any other system out there, open source or proprietary, being able to easily add and remove storage over time with no downtime, safely and easily.
It comes bundled with RadosGW, an S3-compatible object store; CephFS, a NFS-like clustered filesystem; and RBD, a block storage system.

With the help of [Rook](https://rook.io), the vast majority of the complexity of Ceph is hidden away by a very robust operator, allowing you to control almost everything about your Ceph cluster from fairly simple Kubernetes CRDs.

So if Ceph is so great, why not use it for everything?

Ceph can be rather slow for small clusters.
It relies heavily on CPUs and massive parallelisation to provide good cluster performance, so if you don't have much of those dedicated to Ceph, it is not going to be well-optimised for you.
Also, if your cluster is small, just running Ceph may eat up a significant amount of the resources you have available.

Troubleshooting Ceph can be difficult if you do not understand its architecture.
There are lots of acronyms and the documentation assumes a fair level of knowledge.
There are very good tools for inspection and debugging, but this is still frequently seen as a concern.

### OpenEBS Mayastor replicated storage

[Mayastor](https://github.com/openebs/Mayastor) is an OpenEBS project built in Rust utilising the modern NVMEoF system.
It is fast and lean but still cluster-oriented and cloud native.

#### Video Walkthrough

To see a live demo of this section, see the video below:

<iframe width="560" height="315" src="https://www.youtube.com/embed/q86Kidk81xE" frameborder="0" allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture" allowfullscreen></iframe>

#### Prep Nodes

Either during initial cluster creation or on running worker nodes, several machine config values should be edited.
(This information is gathered from the OpenEBS Replicated PV Mayastor [documentation](https://openebs.io/docs/Solutioning/openebs-on-kubernetes-platforms/talos).)

This can be done with `talosctl patch machineconfig` or via config patches during `talosctl gen config`.

Some examples are shown below: modify as needed.

First create a config patch file named `mayastor-patch.yaml` with the following contents:

```yaml
machine:
  sysctls:
    vm.nr_hugepages: "1024"
  nodeLabels:
    openebs.io/engine: "mayastor"
  kubelet:
    extraMounts:
      - destination: /var/local
        type: bind
        source: /var/local
        options:
          - rbind
          - rshared
          - rw
```

Create another config patch file named `mayastor-patch-cp.yaml` with the following contents:

```yaml
cluster:
  apiServer:
    admissionControl:
      - name: PodSecurity
        configuration:
          apiVersion: pod-security.admission.config.k8s.io/v1beta1
          kind: PodSecurityConfiguration
          exemptions:
            namespaces:
              - openebs
```

Using gen config

```bash
talosctl gen config my-cluster https://mycluster.local:6443 --config-patch-control-plane=@mayastor-patch-cp.yaml --config-patch-worker @mayastor-patch.yaml
```

Patching an existing node

```bash
talosctl patch machineconfig -n <node ip> --patch @mayastor-patch.yaml
```

> Note: If you are adding/updating the `vm.nr_hugepages` on a node which already had the `openebs.io/engine=mayastor` label set, you'd need to restart kubelet so that it picks up the new value, by issuing the following command

```bash
talosctl -n <node ip> service kubelet restart
```

#### Deploy Mayastor

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

Follow the Post-Installation from official [documentation](https://openebs.io/docs/quickstart-guide/installation#post-installation-considerations) to either create use Local Storage or Replicated Storage.

### Piraeus / LINSTOR

* [Piraeus-Operator](https://piraeus.io/)
* [LINSTOR](https://linbit.com/drbd/)
* [DRBD Extension](https://github.com/siderolabs/extensions#storage)

#### Install Piraeus Operator V2

There is already a how-to for Talos: [Link](https://github.com/piraeusdatastore/piraeus-operator/blob/v2/docs/how-to/talos.md)

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

NFS is an old pack animal long past its prime.
NFS is slow, has all kinds of bottlenecks involving contention, distributed locking, single points of service, and more.
However, it is supported by a wide variety of systems.
You don't want to use it unless you have to, but unfortunately, that "have to" is too frequent.

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
These include the original OpenEBS, Rancher's Longhorn, and many proprietary systems.
iSCSI in Linux is facilitated by [open-iscsi](https://github.com/open-iscsi/open-iscsi).
This system was designed long before containers caught on, and it is not well
suited to the task, especially when coupled with a read-only host operating
system.

iSCSI support in Talos is now supported via the [iscsi-tools](https://github.com/siderolabs/extensions/pkgs/container/iscsi-tools) [system extension]({{< relref "../../talos-guides/configuration/system-extensions" >}}) installed.
The extension enables compatibility with OpenEBS Jiva - refer to the [local storage]({{< relref "replicated-local-storage-with-openebs" >}}) installation guide for more information.
