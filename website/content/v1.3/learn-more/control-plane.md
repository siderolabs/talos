---
title: "Control Plane"
weight: 50
description: "Understand the Kubernetes Control Plane."
---

This guide provides information about the Kubernetes control plane, and details on how Talos runs and bootstraps the Kubernetes control plane.

<!-- markdownlint-disable MD026 -->

## What is a control plane node?

A control plane node is a node which:

- runs etcd, the Kubernetes database
- runs the Kubernetes control plane
  - kube-apiserver
  - kube-controller-manager
  - kube-scheduler
- serves as an administrative proxy to the worker nodes

These nodes are critical to the operation of your cluster.
Without control plane nodes, Kubernetes will not respond to changes in the
system, and certain central services may not be available.

Talos nodes which have `.machine.type` of `controlplane` are control plane nodes.
(check via `talosctl get member`)

Control plane nodes are tainted by default to prevent workloads from being scheduled onto them.
This is both to protect the control plane from workloads consuming resources and starving the control plane processes, and also to reduce the risk of a vulnerability exposes the control plane's credentials to a workload.

## The Control Plane and Etcd

A critical design concept of Kubernetes (and Talos) is the `etcd` database.

Properly managed (which Talos Linux does), `etcd` should never have split brain or noticeable down time.
In order to do this, `etcd` maintains the concept of "membership" and of
"quorum".
To perform any operation, read or write, the database requires
quorum.
That is, a majority of members must agree on the current leader, and absenteeism (members that are down, or not reachable)
counts as a negative.
For example, if there are three members, at least two out
of the three must agree on the current leader.
If two disagree or fail to answer, the `etcd` database will lock itself
until quorum is achieved in order to protect the integrity of
the data.

This design means that having two controlplane nodes  is _worse_ than having only one, because if _either_ goes down, your database will lock (and the chance of one of two nodes going down is greater than the chance of just a single node going down).
Similarly, a 4 node etcd cluster is worse than a 3 node etcd cluster - a 4 node cluster requires 3 nodes to be up to achieve quorum (in order to have a majority), while the 3 node cluster requires 2 nodes:
i.e. both can support a single node failure and keep running - but the chance of a node failing in a 4 node cluster is higher than that in a 3 node cluster.

Another note about etcd: due to the need to replicate data amongst members, performance of etcd _decreases_ as the cluster scales.
A 5 node cluster can commit about 5% less writes per second than a 3 node cluster running on the same hardware.

## Recommendations for your control plane

- Run your clusters with three or five control plane nodes.
  Three is enough for most use cases.
  Five will give you better availability (in that it can tolerate two node failures simultaneously), but cost you more both in the number of nodes required, and also as each node may require more hardware resources to offset the performance degradation seen in larger clusters.
- Implement good monitoring and put processes in place to deal with a failed node in a timely manner (and test them!)
- Even with robust monitoring and procedures for replacing failed nodes in place, backup etcd and your control plane node configuration to guard against unforeseen disasters.
- Monitor the performance of your etcd clusters.
  If etcd performance is slow, vertically scale the nodes, not the number of nodes.
- If a control plane node fails, remove it first, then add the replacement node.
  (This ensures that the failed node does not "vote" when adding in the new node, minimizing the chances of a quorum violation.)
- If replacing a node that has not failed, add the new one, then remove the old.

## Bootstrapping the Control Plane

Every new cluster must be bootstrapped only once, which is achieved by telling a single control plane node to initiate the bootstrap.

Bootstrapping itself does not do anything with Kubernetes.
Bootstrapping only tells `etcd` to form a cluster, so don't judge the success of
a bootstrap by the failure of Kubernetes to start.
Kubernetes relies on `etcd`, so bootstrapping is _required_, but it is not
_sufficient_ for Kubernetes to start.
If your Kubernetes cluster fails to form for other reasons (say, a bad
configuration option or unavailable container repository), if the bootstrap API
call returns successfully, you do NOT need to bootstrap again:
just fix the config or let Kubernetes retry.

### High-level Overview

Talos cluster bootstrap flow:

1. The `etcd` service is started on control plane nodes.
   Instances of `etcd` on control plane nodes build the `etcd` cluster.
2. The `kubelet` service is started.
3. Control plane components are started as static pods via the `kubelet`, and the `kube-apiserver` component connects to the local (running on the same node) `etcd` instance.
4. The `kubelet` issues client certificate using the bootstrap token using the control plane endpoint (via `kube-apiserver` and `kube-controller-manager`).
5. The `kubelet` registers the node in the API server.
6. Kubernetes control plane schedules pods on the nodes.

### Cluster Bootstrapping

All nodes start the `kubelet` service.
The `kubelet` tries to contact the control plane endpoint, but as it is not up yet, it keeps retrying.

One of the control plane nodes is chosen as the bootstrap node, and promoted using the bootstrap API (`talosctl bootstrap`).
The bootstrap node initiates the `etcd` bootstrap process by initializing `etcd` as the first member of the cluster.

> Once `etcd` is bootstrapped, the bootstrap node has no special role and acts the same way as other control plane nodes.

Services `etcd` on non-bootstrap nodes try to get `Endpoints` resource via control plane endpoint, but that request fails as control plane endpoint is not up yet.

As soon as `etcd` is up on the bootstrap node, static pod definitions for the Kubernetes control plane components (`kube-apiserver`, `kube-controller-manager`, `kube-scheduler`) are rendered to disk.
The `kubelet` service on the bootstrap node picks up the static pod definitions and starts the Kubernetes control plane components.
As soon as `kube-apiserver` is launched, the control plane endpoint comes up.

The bootstrap node acquires an `etcd` mutex and injects the bootstrap manifests into the API server.
The set of the bootstrap manifests specify the Kubernetes join token and kubelet CSR auto-approval.
The `kubelet` service on all the nodes is now able to issue client certificates for themselves and register nodes in the API server.

Other bootstrap manifests specify additional resources critical for Kubernetes operations (i.e. CNI, PSP, etc.)

The `etcd` service on non-bootstrap nodes is now able to discover other members of the `etcd` cluster via the Kubernetes `Endpoints` resource.
The `etcd` cluster is now formed and consists of all control plane nodes.

All control plane nodes render static pod manifests for the control plane components.
Each node now runs a full set of components to make the control plane HA.

The `kubelet` service on worker nodes is now able to issue the client certificate and register itself with the API server.

### Scaling Up the Control Plane

When new nodes are added to the control plane, the process is the same as the bootstrap process above: the `etcd` service discovers existing members of the control plane via the
control plane endpoint, joins the `etcd` cluster, and the control plane components are scheduled on the node.

### Scaling Down the Control Plane

Scaling down the control plane involves removing a node from the cluster.
The most critical part is making sure that the node which is being removed leaves the etcd cluster.
The recommended way to do this is to use:

- `talosctl -n IP.of.node.to.remove reset`
- `kubectl delete node`

When using `talosctl reset` command, the targeted control plane node leaves the `etcd` cluster as part of the reset sequence, and its disks are erased.

### Upgrading Talos on Control Plane Nodes

When a control plane node is upgraded, Talos leaves `etcd`, wipes the system disk, installs a new version of itself, and reboots.
The upgraded node then joins the `etcd` cluster on reboot.
So upgrading a control plane node is equivalent to scaling down the control plane node followed by scaling up with a new version of Talos.
