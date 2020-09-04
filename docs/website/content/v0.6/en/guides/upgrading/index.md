---
title: Upgrading
---

## Talos

In an effort to create more production ready clusters, Talos will now taint control plane nodes as unschedulable.
This means that any application you might have deployed must tolerate this taint if you intend on running the application on control plane nodes.

Another feature you will notice is the automatic uncordoning of nodes that have been upgraded.
Talos will now uncordon a node if the cordon was initiated by the upgrade process.

## Talosctl

The `talosctl` CLI now requires an explicit set of nodes.
This can be configured with `talos config nodes` or set on the fly with `talos --nodes`.

## Kubernetes

In Kubernetes 1.19, the official `hyperkube` image was removed.
This means that in order to upgrade Kubernetes, Talos users will have to change the `command`, and `image` fields of each control plane component.
The `kubelet` image will also have to be updated, if you wish to specify the `kubelet` image explicitly.
The default used by Talos is sufficient in most cases.

In order to edit the control plane, we will need a working `kubectl` config.
If you don't already have one, you can get one by running:

```bash
talosctl kubeconfig
```

### API Server

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

### Controller Manager

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

### Scheduler

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
