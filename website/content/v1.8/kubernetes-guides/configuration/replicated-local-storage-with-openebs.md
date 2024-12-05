---
title: "Replicated Local Storage"
description: "Using local storage with OpenEBS"
aliases:
  - ../../guides/storage
  - ./replicated-local-storage-with-openebs-jiva
---

If you want to use replicated storage leveraging disk space from a local disk with Talos Linux installed, OpenEBS is a great option.

Since OpenEBS is a replicated storage, it's recommended to have at least three nodes where sufficient local disk space is available.
The documentation will follow installing OpenEBS via the offical Helm chart.
Since Talos is different from standard Operating Systems, the OpenEBS components need a little tweaking after the Helm installation.
Refer to the OpenEBS [documentation](https://openebs.io/docs/quickstart-guide/installation) if you need further customization.

> NB: Also note that the Talos nodes need to be upgraded with `--preserve` set while running OpenEBS, otherwise you risk losing data.
> Even though it's possible to recover data from other replicas if the node is wiped during an upgrade, this can require extra operational knowledge to recover, so it's highly recommended to use `--preserve` to avoid data loss.

## Preparing the nodes

Create a machine config patch with the contents below and save as `patch.yaml`

```yaml
machine:
  sysctls:
    vm.nr_hugepages: "1024"
  nodeLabels:
    openebs.io/engine: mayastor
  kubelet:
    extraMounts:
      - destination: /var/local/openebs
        type: bind
        source: /var/local/openebs
        options:
          - rbind
          - rshared
          - rw
```

Apply the machine config to all the nodes using talosctl:

```bash
talosctl -e <endpoint ip/hostname> -n <node ip/hostname> patch mc -p @patch.yaml
```

## Install OpenEBS

```bash
helm repo add openebs https://openebs.github.io/openebs
helm repo update
helm upgrade --install openebs \
  --create-namespace \
  --namespace openebs \
  --set engines.local.lvm.enabled=false \
  --set engines.local.zfs.enabled=false \
  --set mayastor.csi.node.initContainers.enabled=false \
  openebs/openebs
```

This will create 4 storage classes.
The storage class named `openebs-hostpath` is used to create storage that is replicated across all of your nodes.
The storage class named `openebs-single-replica` is used to create hostpath PVCs that are not replicated.
The other 2 storageclasses, `mayastor-etcd-localpv` and `mayastor-loki-localpv`, are used by `OpenEBS` to create persistent volumes on nodes.

## Patching the Namespace

when using the default Pod Security Admissions created by Talos you need the following labels on your namespace:

```yaml
pod-security.kubernetes.io/audit: privileged
pod-security.kubernetes.io/enforce: privileged
pod-security.kubernetes.io/warn: privileged
```

or via kubectl:

```bash
kubectl label ns openebs \
  pod-security.kubernetes.io/audit=privileged \
  pod-security.kubernetes.io/enforce=privileged \
  pod-security.kubernetes.io/warn=privileged
```

## Testing a simple workload

In order to test the OpenEBS installation, let's first create a PVC referencing the `openebs-hostpath` storage class:

```yaml
kind: PersistentVolumeClaim
apiVersion: v1
metadata:
  name: example-openebs-pvc
spec:
  storageClassName: openebs-hostpath
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 4Gi
```

and then create a deployment using the above PVC:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: fio
spec:
  selector:
    matchLabels:
      name: fio
  replicas: 1
  strategy:
    type: Recreate
    rollingUpdate: null
  template:
    metadata:
      labels:
        name: fio
    spec:
      containers:
        - name: perfrunner
          image: openebs/tests-fio
          command: ["/bin/bash"]
          args: ["-c", "while true ;do sleep 50; done"]
          volumeMounts:
            - mountPath: /datadir
              name: fio-vol
      volumes:
        - name: fio-vol
          persistentVolumeClaim:
            claimName: example-openebs-pvc
```

You can clean up the test resources by running the following command:

```bash
kubectl delete deployment fio
kubectl delete pvc example-openebs-pvc
```
