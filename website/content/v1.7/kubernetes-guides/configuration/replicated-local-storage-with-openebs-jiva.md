---
title: "Replicated Local Storage"
description: "Using local storage with OpenEBS Jiva"
aliases:
  - ../../guides/storage
---

If you want to use replicated storage leveraging disk space from a local disk with Talos Linux installed, OpenEBS Jiva is a great option.
This requires installing the `iscsi-tools` [system extension]({{< relref "../../talos-guides/configuration/system-extensions" >}}).

Since OpenEBS Jiva is a replicated storage, it's recommended to have at least three nodes where sufficient local disk space is available.
The documentation will follow installing OpenEBS Jiva via the offical Helm chart.
Since Talos is different from standard Operating Systems, the OpenEBS components need a little tweaking after the Helm installation.
Refer to the OpenEBS Jiva [documentation](https://github.com/openebs/jiva-operator/blob/develop/docs/quickstart.md) if you need further customization.

> NB: Also note that the Talos nodes need to be upgraded with `--preserve` set while running OpenEBS Jiva, otherwise you risk losing data.
> Even though it's possible to recover data from other replicas if the node is wiped during an upgrade, this can require extra operational knowledge to recover, so it's highly recommended to use `--preserve` to avoid data loss.

## Preparing the nodes

Create the [boot assets]({{< relref "../../talos-guides/install/boot-assets" >}}) which includes the `iscsi-tools` system extensions (or create a custom installer and perform a machine upgrade if Talos is already installed).

Create a machine config patch with the contents below and save as `patch.yaml`

```yaml
machine:
  kubelet:
    extraMounts:
      - destination: /var/openebs/local
        type: bind
        source: /var/openebs/local
        options:
          - bind
          - rshared
          - rw
```

Apply the machine config to all the nodes using talosctl:

```bash
talosctl -e <endpoint ip/hostname> -n <node ip/hostname> patch mc -p @patch.yaml
```

The extension status can be verified by running the following command:

```bash
talosctl -e <endpoint ip/hostname> -n <node ip/hostname> get extensions
```

An output similar to below can be observed:

```text
NODE            NAMESPACE   TYPE              ID                                          VERSION   NAME          VERSION
192.168.20.61   runtime     ExtensionStatus   000.ghcr.io-siderolabs-iscsi-tools-v0.1.1   1         iscsi-tools   v0.1.1
```

The service status can be checked by running the following command:

```bash
talosctl -e <endpoint ip/hostname> -n <node ip/hostname> services
```

You should see that the `ext-tgtd` and the `ext-iscsid` services are running.

```text
NODE            SERVICE      STATE     HEALTH   LAST CHANGE     LAST EVENT
192.168.20.51   apid         Running   OK       64h57m15s ago   Health check successful
192.168.20.51   containerd   Running   OK       64h57m23s ago   Health check successful
192.168.20.51   cri          Running   OK       64h57m20s ago   Health check successful
192.168.20.51   etcd         Running   OK       64h55m29s ago   Health check successful
192.168.20.51   ext-iscsid   Running   ?        64h57m19s ago   Started task ext-iscsid (PID 4040) for container ext-iscsid
192.168.20.51   ext-tgtd     Running   ?        64h57m19s ago   Started task ext-tgtd (PID 3999) for container ext-tgtd
192.168.20.51   kubelet      Running   OK       38h14m10s ago   Health check successful
192.168.20.51   machined     Running   ?        64h57m29s ago   Service started as goroutine
192.168.20.51   trustd       Running   OK       64h57m19s ago   Health check successful
192.168.20.51   udevd        Running   OK       64h57m21s ago   Health check successful

```

## Install OpenEBS Jiva

```bash
helm repo add openebs-jiva https://openebs-archive.github.io/jiva-operator
helm repo update
helm upgrade --install --create-namespace --namespace openebs --version 3.2.0 openebs-jiva openebs-jiva/jiva
```

This will create a storage class named `openebs-jiva-csi-default` which can be used for workloads.
The storage class named `openebs-hostpath` is used by jiva to create persistent volumes backed by local storage and then used for replicated storage by the jiva controller.

## Patching the Namespace

when using the default Pod Security Admissions created by Talos you need the following labels on your namespace:

```yaml
pod-security.kubernetes.io/audit: privileged
pod-security.kubernetes.io/enforce: privileged
pod-security.kubernetes.io/warn: privileged
```

or via kubectl:

```bash
kubectl label ns openebs pod-security.kubernetes.io/audit=privileged pod-security.kubernetes.io/enforce=privileged pod-security.kubernetes.io/warn=privileged
```

## Number of Replicas

By Default Jiva uses 3 replicas if your cluster consists of lesser nodes consider setting `defaultPolicy.replicas` to the number of nodes in your cluster e.g. 2.

## Patching the jiva installation

Since Jiva assumes `iscisd` to be running natively on the host and not as a Talos [extension service]({{< relref "../../advanced/extension-services.md" >}}), we need to modify the CSI node daemonset to enable it to find the PID of the `iscsid` service.
The default config map used by Jiva also needs to be modified so that it can execute `iscsiadm` commands inside the PID namespace of the `iscsid` service.

Start by creating a configmap definition named `config.yaml` as below:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  labels:
    app.kubernetes.io/managed-by: pulumi
  name: openebs-jiva-csi-iscsiadm
  namespace: openebs
data:
  iscsiadm: |
    #!/bin/sh
    iscsid_pid=$(pgrep iscsid)

    nsenter --mount="/proc/${iscsid_pid}/ns/mnt" --net="/proc/${iscsid_pid}/ns/net" -- /usr/local/sbin/iscsiadm "$@"
```

Replace the existing config map with the above config map by running the following command:

```bash
kubectl --namespace openebs apply --filename config.yaml
```

Now we need to update the jiva CSI daemonset to run with `hostPID: true` so it can find the PID of the `iscsid` service, by running the following command:

```bash
kubectl --namespace openebs patch daemonset openebs-jiva-csi-node --type=json --patch '[{"op": "add", "path": "/spec/template/spec/hostPID", "value": true}]'
```

## Testing a simple workload

In order to test the Jiva installation, let's first create a PVC referencing the `openebs-jiva-csi-default` storage class:

```yaml
kind: PersistentVolumeClaim
apiVersion: v1
metadata:
  name: example-jiva-csi-pvc
spec:
  storageClassName: openebs-jiva-csi-default
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
            claimName: example-jiva-csi-pvc
```

You can clean up the test resources by running the following command:

```bash
kubectl delete deployment fio
kubectl delete pvc example-jiva-csi-pvc
```
