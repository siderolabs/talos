---
title: "Ceph Storage cluster with Rook"
description: "Guide on how to create a simple Ceph storage cluster with Rook for Kubernetes"
aliases:
  - ../../guides/configuring-ceph-with-rook
---

## Preparation

Talos Linux reserves an entire disk for the OS installation, so machines with multiple available disks are needed for a reliable Ceph cluster with Rook and Talos Linux.
Rook requires that the block devices or partitions used by Ceph have no partitions or formatted filesystems before use.
Rook also requires a minimum Kubernetes version of `v1.16` and Helm `v3.0` for installation of charts.
It is highly recommended that the [Rook Ceph overview](https://rook.io/docs/rook/v1.8/ceph-storage.html) is read and understood before deploying a Ceph cluster with Rook.

## Installation

Creating a Ceph cluster with Rook requires two steps; first the Rook Operator needs to be installed which can be done with a Helm Chart.
The example below installs the Rook Operator into the `rook-ceph` namespace, which is the default for a Ceph cluster with Rook.

```shell
$ helm repo add rook-release https://charts.rook.io/release
"rook-release" has been added to your repositories

$ helm install --create-namespace --namespace rook-ceph rook-ceph rook-release/rook-ceph
W0327 17:52:44.277830   54987 warnings.go:70] policy/v1beta1 PodSecurityPolicy is deprecated in v1.21+, unavailable in v1.25+
W0327 17:52:44.612243   54987 warnings.go:70] policy/v1beta1 PodSecurityPolicy is deprecated in v1.21+, unavailable in v1.25+
NAME: rook-ceph
LAST DEPLOYED: Sun Mar 27 17:52:42 2022
NAMESPACE: rook-ceph
STATUS: deployed
REVISION: 1
TEST SUITE: None
NOTES:
The Rook Operator has been installed. Check its status by running:
  kubectl --namespace rook-ceph get pods -l "app=rook-ceph-operator"

Visit https://rook.io/docs/rook/latest for instructions on how to create and configure Rook clusters

Important Notes:
- You must customize the 'CephCluster' resource in the sample manifests for your cluster.
- Each CephCluster must be deployed to its own namespace, the samples use `rook-ceph` for the namespace.
- The sample manifests assume you also installed the rook-ceph operator in the `rook-ceph` namespace.
- The helm chart includes all the RBAC required to create a CephCluster CRD in the same namespace.
- Any disk devices you add to the cluster in the 'CephCluster' must be empty (no filesystem and no partitions).
```

Default PodSecurity configuration prevents execution of priviledged pods.
Adding a label to the namespace will allow ceph to start.

```shell
kubectl label namespace rook-ceph pod-security.kubernetes.io/enforce=privileged
```

Once that is complete, the Ceph cluster can be installed with the official Helm Chart.
The Chart can be installed with default values, which will attempt to use all nodes in the Kubernetes cluster, and all unused disks on each node for Ceph storage, and make available block storage, object storage, as well as a shared filesystem.
Generally more specific node/device/cluster configuration is used, and the [Rook documentation](https://rook.io/docs/rook/v1.8/ceph-cluster-crd.html) explains all the available options in detail.
For this example the defaults will be adequate.

```shell
$ helm install --create-namespace --namespace rook-ceph rook-ceph-cluster --set operatorNamespace=rook-ceph rook-release/rook-ceph-cluster
NAME: rook-ceph-cluster
LAST DEPLOYED: Sun Mar 27 18:12:46 2022
NAMESPACE: rook-ceph
STATUS: deployed
REVISION: 1
TEST SUITE: None
NOTES:
The Ceph Cluster has been installed. Check its status by running:
  kubectl --namespace rook-ceph get cephcluster

Visit https://rook.github.io/docs/rook/latest/ceph-cluster-crd.html for more information about the Ceph CRD.

Important Notes:
- You can only deploy a single cluster per namespace
- If you wish to delete this cluster and start fresh, you will also have to wipe the OSD disks using `sfdisk`
```

Now the Ceph cluster configuration has been created, the Rook operator needs time to install the Ceph cluster and bring all the components online.
The progression of the Ceph cluster state can be followed with the following command.

```shell
$ watch kubectl --namespace rook-ceph get cephcluster rook-ceph
Every 2.0s: kubectl --namespace rook-ceph get cephcluster rook-ceph

NAME        DATADIRHOSTPATH   MONCOUNT   AGE   PHASE         MESSAGE                 HEALTH   EXTERNAL
rook-ceph   /var/lib/rook     3          57s   Progressing   Configuring Ceph Mons
```

Depending on the size of the Ceph cluster and the availability of resources the Ceph cluster should become available, and with it the storage classes that can be used with Kubernetes Physical Volumes.

```shell
$ kubectl --namespace rook-ceph get cephcluster rook-ceph
NAME        DATADIRHOSTPATH   MONCOUNT   AGE   PHASE   MESSAGE                        HEALTH      EXTERNAL
rook-ceph   /var/lib/rook     3          40m   Ready   Cluster created successfully   HEALTH_OK

$ kubectl  get storageclass
NAME                   PROVISIONER                     RECLAIMPOLICY   VOLUMEBINDINGMODE   ALLOWVOLUMEEXPANSION   AGE
ceph-block (default)   rook-ceph.rbd.csi.ceph.com      Delete          Immediate           true                   77m
ceph-bucket            rook-ceph.ceph.rook.io/bucket   Delete          Immediate           false                  77m
ceph-filesystem        rook-ceph.cephfs.csi.ceph.com   Delete          Immediate           true                   77m
```

## Talos Linux Considerations

By default, Rook configues Ceph to have 3 `mon` instances, in which case the data stored in `dataDirHostPath` can be regenerated from the other `mon` instances.
So when performing maintenance on a Talos Linux node with a Rook Ceph cluster (e.g. upgrading the Talos Linux version), it is imperative that care be taken to maintain the health of the Ceph cluster.
Before upgrading, you should always check the health status of the Ceph cluster to ensure that it is healthy.

```shell
$ kubectl --namespace rook-ceph get cephclusters.ceph.rook.io rook-ceph
NAME        DATADIRHOSTPATH   MONCOUNT   AGE   PHASE   MESSAGE                        HEALTH      EXTERNAL
rook-ceph   /var/lib/rook     3          98m   Ready   Cluster created successfully   HEALTH_OK
```

If it is, you can begin the upgrade process for the Talos Linux node, during which time the Ceph cluster will become unhealthy as the node is reconfigured.
Before performing any other action on the Talos Linux nodes, the Ceph cluster must return to a healthy status.

```shell
$ talosctl upgrade --nodes 172.20.15.5 --image ghcr.io/talos-systems/installer:v0.14.3
NODE          ACK                        STARTED
172.20.15.5   Upgrade request received   2022-03-27 20:29:55.292432887 +0200 CEST m=+10.050399758

$ kubectl --namespace rook-ceph get cephclusters.ceph.rook.io
NAME        DATADIRHOSTPATH   MONCOUNT   AGE   PHASE         MESSAGE                   HEALTH        EXTERNAL
rook-ceph   /var/lib/rook     3          99m   Progressing   Configuring Ceph Mgr(s)   HEALTH_WARN

$ kubectl --namespace rook-ceph wait --timeout=1800s --for=jsonpath='{.status.ceph.health}=HEALTH_OK' cephclusters.ceph.rook.io rook-ceph
cephcluster.ceph.rook.io/rook-ceph condition met
```

The above steps need to be performed for each Talos Linux node undergoing maintenance, one at a time.

## Cleaning Up

### Rook Ceph Cluster Removal

Removing a Rook Ceph cluster requires a few steps, starting with signalling to Rook that the Ceph cluster is really being destroyed.
Then all Persistent Volumes (and Claims) backed by the Ceph cluster must be deleted, followed by the Storage Classes and the Ceph storage types.

```shell
$ kubectl --namespace rook-ceph patch cephcluster rook-ceph --type merge -p '{"spec":{"cleanupPolicy":{"confirmation":"yes-really-destroy-data"}}}'
cephcluster.ceph.rook.io/rook-ceph patched

$ kubectl delete storageclasses ceph-block ceph-bucket ceph-filesystem
storageclass.storage.k8s.io "ceph-block" deleted
storageclass.storage.k8s.io "ceph-bucket" deleted
storageclass.storage.k8s.io "ceph-filesystem" deleted

$ kubectl --namespace rook-ceph delete cephblockpools ceph-blockpool
cephblockpool.ceph.rook.io "ceph-blockpool" deleted

$ kubectl --namespace rook-ceph delete cephobjectstore ceph-objectstore
cephobjectstore.ceph.rook.io "ceph-objectstore" deleted

$ kubectl --namespace rook-ceph delete cephfilesystem ceph-filesystem
cephfilesystem.ceph.rook.io "ceph-filesystem" deleted
```

Once that is complete, the Ceph cluster itself can be removed, along with the Rook Ceph cluster Helm chart installation.

```shell
$ kubectl --namespace rook-ceph delete cephcluster rook-ceph
cephcluster.ceph.rook.io "rook-ceph" deleted

$ helm --namespace rook-ceph uninstall rook-ceph-cluster
release "rook-ceph-cluster" uninstalled
```

If needed, the Rook Operator can also be removed along with all the Custom Resource Definitions that it created.

```shell
$ helm --namespace rook-ceph uninstall rook-ceph
W0328 12:41:14.998307  147203 warnings.go:70] policy/v1beta1 PodSecurityPolicy is deprecated in v1.21+, unavailable in v1.25+
These resources were kept due to the resource policy:
[CustomResourceDefinition] cephblockpools.ceph.rook.io
[CustomResourceDefinition] cephbucketnotifications.ceph.rook.io
[CustomResourceDefinition] cephbuckettopics.ceph.rook.io
[CustomResourceDefinition] cephclients.ceph.rook.io
[CustomResourceDefinition] cephclusters.ceph.rook.io
[CustomResourceDefinition] cephfilesystemmirrors.ceph.rook.io
[CustomResourceDefinition] cephfilesystems.ceph.rook.io
[CustomResourceDefinition] cephfilesystemsubvolumegroups.ceph.rook.io
[CustomResourceDefinition] cephnfses.ceph.rook.io
[CustomResourceDefinition] cephobjectrealms.ceph.rook.io
[CustomResourceDefinition] cephobjectstores.ceph.rook.io
[CustomResourceDefinition] cephobjectstoreusers.ceph.rook.io
[CustomResourceDefinition] cephobjectzonegroups.ceph.rook.io
[CustomResourceDefinition] cephobjectzones.ceph.rook.io
[CustomResourceDefinition] cephrbdmirrors.ceph.rook.io
[CustomResourceDefinition] objectbucketclaims.objectbucket.io
[CustomResourceDefinition] objectbuckets.objectbucket.io

release "rook-ceph" uninstalled

$ kubectl delete crds cephblockpools.ceph.rook.io cephbucketnotifications.ceph.rook.io cephbuckettopics.ceph.rook.io \
                      cephclients.ceph.rook.io cephclusters.ceph.rook.io cephfilesystemmirrors.ceph.rook.io \
                      cephfilesystems.ceph.rook.io cephfilesystemsubvolumegroups.ceph.rook.io \
                      cephnfses.ceph.rook.io cephobjectrealms.ceph.rook.io cephobjectstores.ceph.rook.io \
                      cephobjectstoreusers.ceph.rook.io cephobjectzonegroups.ceph.rook.io cephobjectzones.ceph.rook.io \
                      cephrbdmirrors.ceph.rook.io objectbucketclaims.objectbucket.io objectbuckets.objectbucket.io
customresourcedefinition.apiextensions.k8s.io "cephblockpools.ceph.rook.io" deleted
customresourcedefinition.apiextensions.k8s.io "cephbucketnotifications.ceph.rook.io" deleted
customresourcedefinition.apiextensions.k8s.io "cephbuckettopics.ceph.rook.io" deleted
customresourcedefinition.apiextensions.k8s.io "cephclients.ceph.rook.io" deleted
customresourcedefinition.apiextensions.k8s.io "cephclusters.ceph.rook.io" deleted
customresourcedefinition.apiextensions.k8s.io "cephfilesystemmirrors.ceph.rook.io" deleted
customresourcedefinition.apiextensions.k8s.io "cephfilesystems.ceph.rook.io" deleted
customresourcedefinition.apiextensions.k8s.io "cephfilesystemsubvolumegroups.ceph.rook.io" deleted
customresourcedefinition.apiextensions.k8s.io "cephnfses.ceph.rook.io" deleted
customresourcedefinition.apiextensions.k8s.io "cephobjectrealms.ceph.rook.io" deleted
customresourcedefinition.apiextensions.k8s.io "cephobjectstores.ceph.rook.io" deleted
customresourcedefinition.apiextensions.k8s.io "cephobjectstoreusers.ceph.rook.io" deleted
customresourcedefinition.apiextensions.k8s.io "cephobjectzonegroups.ceph.rook.io" deleted
customresourcedefinition.apiextensions.k8s.io "cephobjectzones.ceph.rook.io" deleted
customresourcedefinition.apiextensions.k8s.io "cephrbdmirrors.ceph.rook.io" deleted
customresourcedefinition.apiextensions.k8s.io "objectbucketclaims.objectbucket.io" deleted
customresourcedefinition.apiextensions.k8s.io "objectbuckets.objectbucket.io" deleted
```

### Talos Linux Rook Metadata Removal

If the Rook Operator is cleanly removed following the above process, the node metadata and disks should be clean and ready to be re-used.
In the case of an unclean cluster removal, there may be still a few instances of metadata stored on the system disk, as well as the partition information on the storage disks.
First the node metadata needs to be removed, make sure to update the `nodeName` with the actual name of a storage node that needs cleaning, and `path` with the Rook configuration `dataDirHostPath` (this is `/var/lib/rook` when using the default values.yaml) set when installing the chart.
The following will need to be repeated for each node used in the Rook Ceph cluster.

```shell
$ cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: disk-clean
spec:
  restartPolicy: Never
  nodeName: <storage-node-name>
  volumes:
  - name: rook-data-dir
    hostPath:
      path: <dataDirHostPath>
  containers:
  - name: disk-clean
    image: busybox
    securityContext:
      privileged: true
    volumeMounts:
    - name: rook-data-dir
      mountPath: /node/rook-data
    command: ["/bin/sh", "-c", "rm -rf /node/rook-data/*"]
EOF
pod/disk-clean created

$ kubectl wait --timeout=900s --for=jsonpath='{.status.phase}=Succeeded' pod disk-clean
pod/disk-clean condition met

$ kubectl delete pod disk-clean
pod "disk-clean" deleted
```

Lastly, the disks themselves need the partition and filesystem data wiped before they can be reused.
Again, the following as to be repeated for each node **and** disk used in the Rook Ceph cluster, updating `nodeName` and `of=` in the `command` as needed.

```shell
$ cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: disk-wipe
spec:
  restartPolicy: Never
  nodeName: <storage-node-name>
  containers:
  - name: disk-wipe
    image: busybox
    securityContext:
      privileged: true
    command: ["/bin/sh", "-c", "dd if=/dev/zero bs=1M count=100 oflag=direct of=<device>"]
EOF
pod/disk-wipe created

$ kubectl wait --timeout=900s --for=jsonpath='{.status.phase}=Succeeded' pod disk-wipe
pod/disk-wipe condition met

$ kubectl delete pod disk-wipe
pod "disk-wipe" deleted
```
