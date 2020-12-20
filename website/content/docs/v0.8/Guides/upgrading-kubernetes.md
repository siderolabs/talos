---
title: Upgrading Kubernetes
---

## Video Walkthrough

To see a live demo of this writeup, see the video below:

<!-- TODO: update the video for 0.8 -->

<iframe width="560" height="315" src="https://www.youtube.com/embed/sw78qS8vBGc" frameborder="0" allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture" allowfullscreen></iframe>

## Kubeconfig

In order to edit the control plane, we will need a working `kubectl` config.
If you don't already have one, you can get one by running:

```bash
talosctl --nodes <master node> kubeconfig
```

### Automated Kubernetes Upgrade

To upgrade from Kubernetes v1.19.4 to v1.20.1 run:

```bash
$ talosctl --nodes <master node> upgrade-k8s --from 1.19.4 --to 1.20.1
patched kube-apiserver secrets for "service-account.key"
updating pod-checkpointer grace period to "0m"
sleeping 5m0s to let the pod-checkpointer self-checkpoint be updated
temporarily taking "kube-apiserver" out of pod-checkpointer control
updating daemonset "kube-apiserver" to version "1.20.1"
updating daemonset "kube-controller-manager" to version "1.20.1"
updating daemonset "kube-scheduler" to version "1.20.1"
updating daemonset "kube-proxy" to version "1.20.1"
updating pod-checkpointer grace period to "5m0s"
```

### Manual Kubernetes Upgrade

Kubernetes can be upgraded manually as well by following the steps outlined below.
They are equivalent to the steps performed by the `talosctl upgrade-k8s` command.

#### Patching `kube-apiserver` Secrets

Copy secret value `service-account.key` from the secret `kube-controller-manager` in `kube-system` namespace to the
secret `kube-apiserver`.

After these changes, `kube-apiserver` secret should contain the following entries:

```bash
Data
====
service-account.key:
apiserver.key:
ca.crt:
front-proxy-client.crt:
apiserver-kubelet-client.crt:
encryptionconfig.yaml:
etcd-client.crt:
front-proxy-client.key:
service-account.pub:
apiserver.crt:
auditpolicy.yaml:
etcd-client.key:
apiserver-kubelet-client.key:
front-proxy-ca.crt:
etcd-client-ca.crt:
```

#### pod-checkpointer

Talos runs `pod-checkpointer` component which helps to recover control plane components (specifically, API server) if control plane is not healthy.

However, the way checkpoints interact with API server upgrade may make an upgrade take a lot longer due to a race condition on API server listen port.

In order to speed up upgrades, first lower `pod-checkpointer` grace period to zero (`kubectl -n kube-system edit daemonset pod-checkpointer`), change:

```yaml
kind: DaemonSet
...
spec:
  ...
  template:
    ...
    spec:
      containers:
      - name: pod-checkpointer
        command:
        ...
        - --checkpoint-grace-period=5m0s
```

to:

```yaml
kind: DaemonSet
...
spec:
  ...
  template:
    ...
    spec:
      containers:
      - name: pod-checkpointer
        command:
        ...
        - --checkpoint-grace-period=0s
```

Wait for 5 minutes to let `pod-checkpointer` update self-checkpoint to the new grace period.

#### API Server

In the API server's `DaemonSet`, change:

```yaml
kind: DaemonSet
...
spec:
  ...
  template:
    ...
    spec:
      containers:
        - name: kube-apiserver
          image: k8s.gcr.io/kube-apiserver:v1.19.4
          command:
            - /go-runner
            - /usr/local/bin/kube-apiserver
      tolerations:
        - ...
```

to:

```yaml
kind: DaemonSet
...
spec:
  ...
  template:
    ...
    spec:
      containers:
        - name: kube-apiserver
          image: k8s.gcr.io/kube-apiserver:v1.20.1
          command:
            - /go-runner
            - /usr/local/bin/kube-apiserver
            - ...
            - --api-audiences=<control plane endpoint>
            - --service-account-issuer=<control plane endpoint>
            - --service-account-signing-key-file=/etc/kubernetes/secrets/service-account.key
      tolerations:
        - ...
        - key: node-role.kubernetes.io/control-plane
          operator: Exists
          effect: NoSchedule
```

Summary of the changes:

* update image version
* add new toleration
* add three new flags (replace `<control plane endpoint>` with the actual endpoint of the cluster, e.g. `https://10.5.0.1:6443`)

To edit the `DaemonSet`, run:

```bash
kubectl edit daemonsets -n kube-system kube-apiserver
```

#### Controller Manager

In the controller manager's `DaemonSet`, change:

```yaml
kind: DaemonSet
...
spec:
  ...
  template:
    ...
    spec:
      containers:
        - name: kube-controller-manager
          image: k8s.gcr.io/kube-controller-manager:v1.19.4
      tolerations:
        - ...
```

to:

```yaml
kind: DaemonSet
...
spec:
  ...
  template:
    ...
    spec:
      containers:
        - name: kube-controller-manager
          image: k8s.gcr.io/kube-controller-manager:v1.20.1
      tolerations:
        - ...
        - key: node-role.kubernetes.io/control-plane
          operator: Exists
          effect: NoSchedule
```

To edit the `DaemonSet`, run:

```bash
kubectl edit daemonsets -n kube-system kube-controller-manager
```

#### Scheduler

In the scheduler's `DaemonSet`, change:

```yaml
kind: DaemonSet
...
spec:
  ...
  template:
    ...
    spec:
      containers:
        - name: kube-scheduler
          image: k8s.gcr.io/kube-scheduler:v1.19.4
      tolerations:
        - ...
```

to:

```yaml
kind: DaemonSet
...
spec:
  ...
  template:
    ...
    spec:
      containers:
        - name: kube-sceduler
          image: k8s.gcr.io/kube-scheduler:v1.20.1
      tolerations:
        - ...
        - key: node-role.kubernetes.io/control-plane
          operator: Exists
          effect: NoSchedule
```

To edit the `DaemonSet`, run:

```bash
kubectl edit daemonsets -n kube-system kube-scheduler
```

#### Proxy

In the proxy's `DaemonSet`, change:

```yaml
kind: DaemonSet
...
spec:
  ...
  template:
    ...
    spec:
      containers:
        - name: kube-proxy
          image: k8s.gcr.io/kube-proxy:v1.19.4
      tolerations:
        - ...
```

to:

```yaml
kind: DaemonSet
...
spec:
  ...
  template:
    ...
    spec:
      containers:
        - name: kube-proxy
          image: k8s.gcr.io/kube-proxy:v1.20.1
      tolerations:
        - ...
        - key: node-role.kubernetes.io/control-plane
          operator: Exists
          effect: NoSchedule
```

To edit the `DaemonSet`, run:

```bash
kubectl edit daemonsets -n kube-system kube-proxy
```

#### Restoring pod-checkpointer

Restore grace period of 5 minutes (`kubectl -n kube-system edit daemonset pod-checkpointer`) and add new toleration, change:

```yaml
kind: DaemonSet
...
spec:
  ...
  template:
    ...
    spec:
      containers:
      - name: pod-checkpointer
        command:
        ...
        - --checkpoint-grace-period=0s
      tolerations:
        - ...
```

to:

```yaml
kind: DaemonSet
...
spec:
  ...
  template:
    ...
    spec:
      containers:
      - name: pod-checkpointer
        command:
        ...
        - --checkpoint-grace-period=5m0s
      tolerations:
        - ...
        - key: node-role.kubernetes.io/control-plane
          operator: Exists
          effect: NoSchedule
```

### Kubelet

The Talos team now maintains an image for the `kubelet` that should be used starting with Kubernetes 1.20.
The image for this release is `ghcr.io/talos-systems/kubelet:v1.20.1`.
To explicitly set the image, we can use the [official documentation](/v0.8/en/configuration/v1alpha1#kubelet).
For example:

```yaml
machine:
  ...
  kubelet:
    image: ghcr.io/talos-systems/kubelet:v1.20.1
```
