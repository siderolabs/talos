---
title: "Storage"
description: ""
---

Talos is known to work with Rook, Mayastor (OpenEBS) and NFS.

## Rook

We recommend at least Rook v1.5.

## NFS

The NFS client is part of the [`kubelet` image](https://github.com/talos-systems/kubelet) maintained by the Talos team.
This means that the version installed in your running `kubelet` is the version of NFS supported by Talos.

## Mayastor (OpenEBS)

### Video Walkthrough

To see a live demo of this section, see the video below:

<iframe width="560" height="315" src="https://www.youtube.com/embed/q86Kidk81xE" frameborder="0" allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture" allowfullscreen></iframe>

### Prep Nodes

Either during initial cluster creation or on running worker nodes, several machine config values should be edited.
This can be done with `talosctl edit machineconfig` or via config patches during `talosctl gen config`.

- Under `/machine/sysctls`, add `vm.nr_hugepages: "512"`
- Under `/machine/kubelet/extraMounts`, add `/var/local` like so:

```yaml
...
extraMounts:
  - destination: /var/local
    type: bind
    source: /var/local
    options:
    - rbind
    - rshared
    - rw
...
```

- Either using `kubectl taint node` in a pre-existing cluster or by updating `/machine/kubelet/extraArgs` in machine config, add `openebs.io/engine=mayastor` as a node label.
If being done via machine config, `extraArgs` may look like:

```yaml
...
extraArgs:
  node-labels: openebs.io/engine=mayastor
...
```

### Deploy Mayastor

Using the [Mayastor docs](https://mayastor.gitbook.io/introduction/quickstart/deploy-mayastor) as a reference, apply all YAML files necessary.
At the time of writing this looked like:

```bash
kubectl create namespace mayastor

kubectl apply -f https://raw.githubusercontent.com/openebs/Mayastor/master/deploy/moac-rbac.yaml

kubectl apply -f https://raw.githubusercontent.com/openebs/Mayastor/master/deploy/nats-deployment.yaml

kubectl apply -f https://raw.githubusercontent.com/openebs/Mayastor/master/csi/moac/crds/mayastorpool.yaml

kubectl apply -f https://raw.githubusercontent.com/openebs/Mayastor/master/deploy/csi-daemonset.yaml

kubectl apply -f https://raw.githubusercontent.com/openebs/Mayastor/master/deploy/moac-deployment.yaml

kubectl apply -f https://raw.githubusercontent.com/openebs/Mayastor/master/deploy/mayastor-daemonset.yaml
```

### Create Pools

Each "storage" node should have a "MayastorPool" that defines the local disks to use for storage.
These are later considered during scheduling and replication of data.
Create the pool by issuing the following, updating as necessary:

```bash
cat <<EOF | kubectl create -f -
apiVersion: "openebs.io/v1alpha1"
kind: MayastorPool
metadata:
  name: pool-on-talos-xxx
  namespace: mayastor
spec:
  node: talos-xxx
  disks: ["/dev/sdx"]
EOF
```

### Create StorageClass

With the pools created for each node, create a storage class that uses the `nvmf` protocol, updating the number of replicas as necessary:

```bash
cat <<EOF | kubectl create -f -
kind: StorageClass
apiVersion: storage.k8s.io/v1
metadata:
  name: mayastor-nvmf
parameters:
  repl: '1'
  protocol: 'nvmf'
provisioner: io.openebs.csi-mayastor
EOF
```

### Consume Storage

The storage can now be consumed by creating a PersistentVolumeClaim (PVC) that references the StorageClass.
The PVC can then be used by a Pod or Deployment.
An example of creating a PersistentVolumeClaim may look like:

```bash
cat <<EOF | kubectl create -f -
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: mayastor-volume-claim
spec:
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi
  storageClassName: mayastor-nvmf
EOF
```
