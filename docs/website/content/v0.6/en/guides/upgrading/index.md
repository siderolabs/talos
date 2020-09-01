---
title: Upgrading
---

In Kubernetes 1.19, the official `hyperkube` image was removed.
This means that in order to upgrade Kubernetes, Talos users will have to change the `command` field of each control plane component.
The `kubelet` image will also have to be updated, if you wish to specify the `kubelet` image explicitly.
The default used by Talos is sufficient in most cases.

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

### Kubelet

The Talos team now maintains an image for the `kubelet` that should be used starting with Kubernetes 1.19.
The image for this release is `docker.io/autonomy/kubelet:v1.19.0`.
