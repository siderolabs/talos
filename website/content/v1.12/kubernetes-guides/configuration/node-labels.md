---
title: "Node Labels"
description: "How to configure and use node labels with Talos."
---

Talos can propagate labels from `machine.nodeLabels` to the Kubernetes Node object.
These labels are written using the node’s kubelet identity, which is restricted by the Kubernetes [NodeRestriction admission controller](https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/#noderestriction).

With NodeRestriction in place, a kubelet is only allowed to modify a small, whitelisted set of labels, such as:

* `topology.kubernetes.io/region`
* `topology.kubernetes.io/zone`
* `kubernetes.io/hostname`
* `kubernetes.io/arch`
* `kubernetes.io/os`
* some `node.kubernetes.io/*` labels

Labels outside that set, including the conventional role labels `node-role.kubernetes.io/<role>`, are rejected by the API server when requested by the node itself.

This prevents a worker node from assigning itself a privileged role.

### Apply nodeLabels

You can add labels to a node by specifying them under `machine.nodeLabels` in the machine configuration. For example:

```yaml
machine:
  nodeLabels:
    topology.kubernetes.io/zone: "pve03"
    topology.kubernetes.io/region: "Region-1"
```

After you patch and reboot, the nodes will have the labels applied. Verify them with

```bash
kubectl describe node <node-name>
```

### Role Labels

If you need to assign role labels, for example, `node-role.kubernetes.io/worker` or `node-role.kubernetes.io/ingress`, you must set them with a cluster-admin account:

```bash
kubectl label node <node-name> node-role.kubernetes.io/worker
```

Alternatively, you can use the [Talos Cloud Controller Manager](https://github.com/siderolabs/talos-cloud-controller-manager/blob/main/docs/config.md) or your own controller to translate custom domain labels into the conventional `node-role.kubernetes.io/*` form if required.
