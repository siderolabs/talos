---
title: "Storage"
description: ""
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

> **Talos** recommends having a separate disks (apart from the Talos install disk) to be used for storage.

Redundancy in storage is usually very important.
Scaling capabilities, reliability, speed, maintenance load, and ease of use are all factors you must consider when managing your own storage.

Running a storage cluster can be a very good choice when managing your own storage, and there are two project we recommend, depending on your situation.

If you need vast amounts of storage composed of more than a dozen or so disks, just use Rook to manage Ceph.
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
It comes bundled with RadosGW, an S3-compatible object store.
It comes with CephFS, a NFS-like clustered filesystem.
And of course, it comes with RBD, a block storage system.

With the help of [Rook](https://rook.io), the vast majority of the complexity of Ceph is hidden away by a very robust operator, allowing you to control almost everything about your Ceph cluster from fairly simple Kubernetes CRDs.

So if Ceph is so great, why not use it for everything?

Ceph can be rather slow for small clusters.
It relies heavily on CPUs and massive parallelisation to provide good cluster performance, so if you don't have much of those dedicated to Ceph, it is not going to be well-optimised for you.
Also, if your cluster is small, just running Ceph may eat up a significant amount of the resources you have available.

Troubleshooting Ceph can be difficult if you do not understand its architecture.
There are lots of acronyms and the documentation assumes a fair level of knowledge.
There are very good tools for inspection and debugging, but this is still frequently seen as a concern.

### Mayastor

[Mayastor](https://github.com/openebs/Mayastor) is an OpenEBS project built in Rust utilising the modern NVMEoF system.
(Despite the name, Mayastor does _not_ require you to have NVME drives.)
It is fast and lean but still cluster-oriented and cloud native.
Unlike most of the other OpenEBS project, it is _not_ built on the ancient iSCSI system.

Unlike Ceph, Mayastor is _just_ a block store.
It focuses on block storage and does it well.
It is much less complicated to set up than Ceph, but you probably wouldn't want to use it for more than a few dozen disks.

Mayastor is new, maybe _too_ new.
If you're looking for something well-tested and battle-hardened, this is not it.
If you're looking for something lean, future-oriented, and simpler than Ceph, it might be a great choice.

### Video Walkthrough

To see a live demo of this section, see the video below:

<iframe width="560" height="315" src="https://www.youtube.com/embed/q86Kidk81xE" frameborder="0" allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture" allowfullscreen></iframe>

### Prep Nodes

Either during initial cluster creation or on running worker nodes, several machine config values should be edited.
This information is gathered from the Mayastor [documentation](https://mayastor.gitbook.io/introduction/quickstart/preparing-the-cluster).
We need to set the `vm.nr_hugepages` sysctl and add `openebs.io/engine=mayastor` labels to the nodes which are meant to be storage nodes
This can be done with `talosctl patch machineconfig` or via config patches during `talosctl gen config`.

Some examples are shown below, modify as needed.

Using gen config

```bash
talosctl gen config my-cluster https://mycluster.local:6443 --config-patch '[{"op": "add", "path": "/machine/sysctls", "value": {"vm.nr_hugepages": "1024"}}, {"op": "add", "path": "/machine/kubelet/extraArgs", "value": {"node-labels": "openebs.io/engine=mayastor"}}]'
```

Patching an existing node

```bash
talosctl patch --mode=no-reboot machineconfig -n <node ip> --patch '[{"op": "add", "path": "/machine/sysctls", "value": {"vm.nr_hugepages": "1024"}}, {"op": "add", "path": "/machine/kubelet/extraArgs", "value": {"node-labels": "openebs.io/engine=mayastor"}}]'
```

> Note: If you are adding/updating the `vm.nr_hugepages` on a node which already had the `openebs.io/engine=mayastor` label set, you'd need to restart kubelet so that it picks up the new value, by issuing the following command

```bash
talosctl -n <node ip> service kubelet restart
```

### Deploy Mayastor

Continue setting up [Mayastor](https://mayastor.gitbook.io/introduction/quickstart/deploy-mayastor) using the official documentation.

## NFS

NFS is an old pack animal long past its prime.
However, it is supported by a wide variety of systems.
You don't want to use it unless you have to, but unfortunately, that "have to" is too frequent.

NFS is slow, has all kinds of bottlenecks involving contention, distributed locking, single points of service, and more.

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
This includes things like the original OpenEBS, Rancher's Longhorn, and many proprietary systems.
Unfortunately, Talos does _not_ support iSCSI-based systems.
iSCSI in Linux is facilitated by [open-iscsi](https://github.com/open-iscsi/open-iscsi).
This system was designed long before containers caught on, and it is not well
suited to the task, especially when coupled with a read-only host operating
system.

One day, we hope to work out a solution for facilitating iSCSI-based systems, but this is not yet available.
