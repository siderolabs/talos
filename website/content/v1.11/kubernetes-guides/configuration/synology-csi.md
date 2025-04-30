---
title: "iSCSI Storage with Synology CSI"
description: "Automatically provision iSCSI volumes on a Synology NAS with the synology-csi driver."
aliases:
  - ../../guides/synology-csi
---

## Background

Synology is a company that specializes in Network Attached Storage (NAS) devices.
They provide a number of features within a simple web OS, including an LDAP server, Docker support, and (perhaps most relevant to this guide) function as an iSCSI host.
The focus of this guide is to allow a Kubernetes cluster running on Talos to provision Kubernetes storage (both dynamic or static) on a Synology NAS using a direct integration, rather than relying on an intermediary layer like Rook/Ceph or Maystor.

This guide assumes a very basic familiarity with iSCSI terminology (LUN, iSCSI target, etc.).

## Prerequisites

* Synology NAS running DSM 7.0 or above
* Provisioned Talos cluster running Kubernetes v1.20 or above with `siderolabs/iscsi-tools` extension installed
* (Optional) Both [Volume Snapshot CRDs](https://github.com/kubernetes-csi/external-snapshotter/tree/v4.0.0/client/config/crd) and the [common snapshot controller](https://github.com/kubernetes-csi/external-snapshotter/tree/v4.0.0/deploy/kubernetes/snapshot-controller) must be installed in your Kubernetes cluster if you want to use the **Snapshot** feature

## Setting up the Synology user account

The `synology-csi` controller interacts with your NAS in two different ways: via the API and via the iSCSI protocol.
Actions such as creating a new iSCSI target or deleting an old one are accomplished via the Synology API, and require administrator access.
On the other hand, mounting the disk to a pod and reading from / writing to it will utilize iSCSI.
Because you can only authenticate with one account per DSM configured, that account needs to have admin privileges.
In order to minimize access in the case of these credentials being compromised, you should configure the account with the lease possible amount of access – explicitly specify "No Access" on all volumes when configuring the user permissions.

## Setting up the Synology CSI

> Note: this guide is paraphrased from the Synology CSI [readme](https://github.com/zebernst/synology-csi-talos).
> Please consult the readme for more in-depth instructions and explanations.

Clone the git repository.

```bash
git clone https://github.com/zebernst/synology-csi-talos.git
```

While Synology provides some automated scripts to deploy the CSI driver, they can be finicky especially when making changes to the source code.
We will be configuring and deploying things manually in this guide.

The relevant files we will be touching are in the following locations:

```text
.
├── Dockerfile
├── Makefile
├── config
│   └── client-info-template.yml
└── deploy
    └── kubernetes
        └── v1.20
            ├── controller.yml
            ├── csi-driver.yml
            ├── namespace.yml
            ├── node.yml
            ├── snapshotter
            │   ├── snapshotter.yaml
            │   └── volume-snapshot-class.yml
            └── storage-class.yml
```

### Configure connection info

Use `config/client-info-template.yml` as an example to configure the connection information for DSM.
You can specify **one or more** storage systems on which the CSI volumes will be created.
See below for an example:

```yaml
---
clients:
- host: 192.168.1.1   # ipv4 address or domain of the DSM
  port: 5000          # port for connecting to the DSM
  https: false        # set this true to use https. you need to specify the port to DSM HTTPS port as well
  username: username  # username
  password: password  # password
```

Create a Kubernetes secret using the client information config file.

```bash
kubectl create secret -n synology-csi generic client-info-secret --from-file=config/client-info.yml
```

Note that if you rename the secret to something other than `client-info-secret`, make sure you update the corresponding references in the deployment manifests as well.

### Build the Talos-compatible image

Modify the `Makefile` so that the image is built and tagged under your GitHub Container Registry username:

```makefile
REGISTRY_NAME=ghcr.io/<username>
```

When you run `make docker-build` or `make docker-build-multiarch`, it will push the resulting image to `ghcr.io/<username>/synology-csi:v1.1.0`.
Ensure that you find and change any reference to `synology/synology-csi:v1.1.0` to point to your newly-pushed image within the deployment manifests.

### Configure the CSI driver

By default, the deployment manifests include one storage class and one volume snapshot class.
See below for examples:

```yaml
---
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  annotations:
    storageclass.kubernetes.io/is-default-class: "false"
  name: syno-storage
provisioner: csi.san.synology.com
parameters:
  fsType: 'ext4'
  dsm: '192.168.1.1'
  location: '/volume1'
reclaimPolicy: Retain
allowVolumeExpansion: true
---
apiVersion: snapshot.storage.k8s.io/v1
kind: VolumeSnapshotClass
metadata:
  name: syno-snapshot
  annotations:
    storageclass.kubernetes.io/is-default-class: "false"
driver: csi.san.synology.com
deletionPolicy: Delete
parameters:
  description: 'Kubernetes CSI'
```

It can be useful to configure multiple different StorageClasses.
For example, a popular strategy is to create two nearly identical StorageClasses, with one configured with `reclaimPolicy: Retain` and the other with `reclaimPolicy: Delete`.
Alternately, a workload may require a specific filesystem, such as `ext4`.
If a Synology NAS is going to be the most common way to configure storage on your cluster, it can be convenient to add the `storageclass.kubernetes.io/is-default-class: "true"` annotation to one of your StorageClasses.

The following table details the configurable parameters for the Synology StorageClass.

| Name                                             | Type   | Description                                                                                                                                                        | Default | Supported protocols |
| ------------------------------------------------ | ------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------ | ------- | ------------------- |
| *dsm*                                            | string | The IPv4 address of your DSM, which must be included in the `client-info.yml` for the CSI driver to log in to DSM                                                  | -       | iSCSI, SMB          |
| *location*                                       | string | The location (/volume1, /volume2, ...) on DSM where the LUN for *PersistentVolume* will be created                                                                 | -       | iSCSI, SMB          |
| *fsType*                                         | string | The formatting file system of the *PersistentVolumes* when you mount them on the pods. This parameter only works with iSCSI. For SMB, the fsType is always ‘cifs‘. | `ext4`  | iSCSI               |
| *protocol*                                       | string | The backing storage protocol. Enter ‘iscsi’ to create LUNs or ‘smb‘ to create shared folders on DSM.                                                               | `iscsi` | iSCSI, SMB          |
| *csi.storage.k8s.io/node-stage-secret-name*      | string | The name of node-stage-secret. Required if DSM shared folder is accessed via SMB.                                                                                  | -       | SMB                 |
| *csi.storage.k8s.io/node-stage-secret-namespace* | string | The namespace of node-stage-secret. Required if DSM shared folder is accessed via SMB.                                                                             | -       | SMB                 |

The VolumeSnapshotClass can be similarly configured with the following parameters:

| Name          | Type   | Description                                  | Default | Supported protocols |
| ------------- | ------ | -------------------------------------------- | ------- | ------------------- |
| *description* | string | The description of the snapshot on DSM       | -       | iSCSI               |
| *is_locked*   | string | Whether you want to lock the snapshot on DSM | `false` | iSCSI, SMB          |

### Apply YAML manifests

Once you have created the desired StorageClass(es) and VolumeSnapshotClass(es), the final step is to apply the Kubernetes manifests against the cluster.
The easiest way to apply them all at once is to create a `kustomization.yaml` file in the same directory as the manifests and use Kustomize to apply:

```bash
kubectl apply -k path/to/manifest/directory
```

Alternately, you can apply each manifest one-by-one:

```bash
kubectl apply -f <file>
```

## Run performance tests

In order to test the provisioning, mounting, and performance of using a Synology NAS as Kubernetes persistent storage, use the following command:

```bash
kubectl apply -f speedtest.yaml
```

Content of speedtest.yaml ([source](https://github.com/phnmnl/k8s-volume-test))

```yaml
kind: PersistentVolumeClaim
apiVersion: v1
metadata:
  name: test-claim
spec:
#  storageClassName: syno-storage
  accessModes:
  - ReadWriteMany
  resources:
    requests:
      storage: 5G
---
apiVersion: batch/v1
kind: Job
metadata:
  name: read
spec:
  template:
    metadata:
      name: read
      labels:
        app: speedtest
        job: read
    spec:
      containers:
      - name: read
        image: ubuntu:xenial
        command: ["dd","if=/mnt/pv/test.img","of=/dev/null","bs=8k"]
        volumeMounts:
        - mountPath: "/mnt/pv"
          name: test-volume
      volumes:
      - name: test-volume
        persistentVolumeClaim:
          claimName: test-claim
      restartPolicy: Never
---
apiVersion: batch/v1
kind: Job
metadata:
  name: write
spec:
  template:
    metadata:
      name: write
      labels:
        app: speedtest
        job: write
    spec:
      containers:
      - name: write
        image: ubuntu:xenial
        command: ["dd","if=/dev/zero","of=/mnt/pv/test.img","bs=1G","count=1","oflag=dsync"]
        volumeMounts:
        - mountPath: "/mnt/pv"
          name: test-volume
      volumes:
      - name: test-volume
        persistentVolumeClaim:
          claimName: test-claim
      restartPolicy: Never
```

If these two jobs complete successfully, use the following commands to get the results of the speed tests:

```bash
# Pod logs for read test:
kubectl logs -l app=speedtest,job=read

# Pod logs for write test:
kubectl logs -l app=speedtest,job=write
```

When you're satisfied with the results of the test, delete the artifacts created from the speedtest:

```bash
kubectl delete -f speedtest.yaml
```
