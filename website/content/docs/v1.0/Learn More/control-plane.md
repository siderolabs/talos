---
title: "Control Plane"
weight: 8
---

This guide provides details on how Talos runs and bootstraps the Kubernetes control plane.

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

One of the control plane nodes is chosen as the bootstrap node.
The node's type can be either `init` or `controlplane`, where the `controlplane` type is promoted using the bootstrap API (`talosctl bootstrap`).
The bootstrap node initiates the `etcd` bootstrap process by initializing `etcd` as the first member of the cluster.

> Note: there should be only one bootstrap node for the cluster lifetime.
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
When using `talosctl reset` command, the targeted control plane node leaves the `etcd` cluster as part of the reset sequence.

### Upgrading Control Plane Nodes

When a control plane node is upgraded, Talos leaves `etcd`, wipes the system disk, installs a new version of itself, and reboots.
The upgraded node then joins the `etcd` cluster on reboot.
So upgrading a control plane node is equivalent to scaling down the control plane node followed by scaling up with a new version of Talos.
