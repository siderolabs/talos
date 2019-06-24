---
title: containerd
menu:
  docs:
    parent: components
---

[Containerd](https://github.com/containerd/containerd) provides the container runtime to launch workloads on Talos as well as Kubernetes.

Talos services are namespaced under the `system` namespace in containerd whereas the Kubernetes services are namespaced under the `k8s.io` namespace.
