---
title: Upgrading Kubernetes
---

## Video Walkthrough

To see a live demo of this writeup, see the video below:

<iframe width="560" height="315" src="https://www.youtube.com/embed/sw78qS8vBGc" frameborder="0" allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture" allowfullscreen></iframe>

## Kubelet Image

In Kubernetes 1.19, the official `hyperkube` image was removed.
This means that in order to upgrade Kubernetes, Talos users will have to change the `command`, and `image` fields of each control plane component.
The `kubelet` image will also have to be updated, if you wish to specify the `kubelet` image explicitly.
The default used by Talos is sufficient in most cases.

## Kubeconfig

In order to edit the control plane, we will need a working `kubectl` config.
If you don't already have one, you can get one by running:

```bash
talosctl --nodes <master node> kubeconfig
```

### Automated Kubernetes Upgrade

In Talos v0.6.1 we introduced the `upgrade-k8s` command in `talosctl`.
This command can be used to automate the Kubernetes upgrade process.
For example, to upgrade from Kubernetes v1.18.6 to v1.19.0 run:

```bash
$ talosctl --nodes <master node> upgrade-k8s --from 1.18.6 --to 1.19.0
updating pod-checkpointer grace period to "0m"
sleeping 5m0s to let the pod-checkpointer self-checkpoint be updated
temporarily taking "kube-apiserver" out of pod-checkpointer control
updating daemonset "kube-apiserver" to version "1.19.0"
updating daemonset "kube-controller-manager" to version "1.19.0"
updating daemonset "kube-scheduler" to version "1.19.0"
updating daemonset "kube-proxy" to version "1.19.0"
updating pod-checkpointer grace period to "5m0s"
```

### Manual Kubernetes Upgrade

Kubernetes can be upgraded manually as well by following the steps outlined below.
They are equivalent to the steps performed by the `talosctl upgrade-k8s` command.

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
          image: ...
          command:
            - ./hyperkube
            - kube-apiserver
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
          image: k8s.gcr.io/kube-apiserver:v1.19.0
          command:
            - /go-runner
            - /usr/local/bin/kube-apiserver
```

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
          image: ...
          command:
            - ./hyperkube
            - kube-controller-manager
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
          image: k8s.gcr.io/kube-controller-manager:v1.19.0
          command:
            - /go-runner
            - /usr/local/bin/kube-controller-manager
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
          image: ...
          command:
            - ./hyperkube
            - kube-scheduler
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
          image: k8s.gcr.io/kube-scheduler:v1.19.0
          command:
            - /go-runner
            - /usr/local/bin/kube-scheduler
```

To edit the `DaemonSet`, run:

```bash
kubectl edit daemonsets -n kube-system kube-scheduler
```

#### Restoring pod-checkpointer

Restore grace period of 5 minutes (`kubectl -n kube-system edit daemonset pod-checkpointer`), change:

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
```

### Kubelet

The Talos team now maintains an image for the `kubelet` that should be used starting with Kubernetes 1.19.
The image for this release is `docker.io/autonomy/kubelet:v1.19.0`.
To explicitly set the image, we can use the [official documentation](https://www.talos.dev/docs/v0.6/en/configuration/v1alpha1#kubelet).
For example:

```yaml
machine:
  ...
  kubelet:
    image: docker.io/autonomy/kubelet:v1.19.0
```
