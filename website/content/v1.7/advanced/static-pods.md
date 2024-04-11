---
title: "Static Pods"
description: "Using Talos Linux to set up static pods in Kubernetes."
aliases:
  - ../guides/static-pods
---

## Static Pods

Static pods are run directly by the `kubelet` bypassing the Kubernetes API server checks and validations.
Most of the time `DaemonSet` is a better alternative to static pods, but some workloads need to run
before the Kubernetes API server is available or might need to bypass security restrictions imposed by the API server.

See [Kubernetes documentation](https://kubernetes.io/docs/tasks/configure-pod-container/static-pod/) for more information on static pods.

## Configuration

Static pod definitions are specified in the Talos machine configuration:

```yaml
machine:
  pods:
    - apiVersion: v1
       kind: Pod
       metadata:
         name: nginx
       spec:
         containers:
           - name: nginx
             image: nginx
```

Talos renders static pod definitions to the `kubelet` using a local HTTP server, `kubelet` picks up the definition and launches the pod.

Talos accepts changes to the static pod configuration without a reboot.

To see a full list of static pods, use `talosctl get staticpods`, and to see the status of the static pods (as reported by the `kubelet`), use `talosctl get staticpodstatus`.

## Usage

Kubelet mirrors pod definition to the API server state, so static pods can be inspected with `kubectl get pods`, logs can be retrieved with `kubectl logs`, etc.

```bash
$ kubectl get pods
NAME                           READY   STATUS    RESTARTS   AGE
nginx-talos-default-controlplane-2   1/1     Running   0          17s
```

If the API server is not available, status of the static pod can also be inspected with `talosctl containers --kubernetes`:

```bash
$ talosctl containers --kubernetes
NODE         NAMESPACE   ID                                                                                      IMAGE                                                   PID    STATUS
172.20.0.3   k8s.io      default/nginx-talos-default-controlplane-2                                              registry.k8s.io/pause:3.6                               4886   SANDBOX_READY
172.20.0.3   k8s.io      └─ default/nginx-talos-default-controlplane-2:nginx:4183a7d7a771                        docker.io/library/nginx:latest
...
```

Logs of static pods can be retrieved with `talosctl logs --kubernetes`:

```bash
$ talosctl logs --kubernetes default/nginx-talos-default-controlplane-2:nginx:4183a7d7a771
172.20.0.3: 2022-02-10T15:26:01.289208227Z stderr F 2022/02/10 15:26:01 [notice] 1#1: using the "epoll" event method
172.20.0.3: 2022-02-10T15:26:01.2892466Z stderr F 2022/02/10 15:26:01 [notice] 1#1: nginx/1.21.6
172.20.0.3: 2022-02-10T15:26:01.28925723Z stderr F 2022/02/10 15:26:01 [notice] 1#1: built by gcc 10.2.1 20210110 (Debian 10.2.1-6)
```

## Troubleshooting

Talos doesn't perform any validation on the static pod definitions.
If the pod isn't running, use `kubelet` logs (`talosctl logs kubelet`) to find the problem:

```bash
$ talosctl logs kubelet
172.20.0.2: {"ts":1644505520281.427,"caller":"config/file.go:187","msg":"Could not process manifest file","path":"/etc/kubernetes/manifests/talos-default-nginx-gvisor.yaml","err":"invalid pod: [spec.containers: Required value]"}
```

## Resource Definitions

Static pod definitions are available as `StaticPod` resources combined with Talos-generated control plane static pods:

```bash
$ talosctl get staticpods
NODE         NAMESPACE   TYPE        ID                        VERSION
172.20.0.3   k8s         StaticPod   default-nginx             1
172.20.0.3   k8s         StaticPod   kube-apiserver            1
172.20.0.3   k8s         StaticPod   kube-controller-manager   1
172.20.0.3   k8s         StaticPod   kube-scheduler            1
```

Talos assigns ID `<namespace>-<name>` to the static pods specified in the machine configuration.

On control plane nodes status of the running static pods is available in the `StaticPodStatus` resource:

```bash
$ talosctl get staticpodstatus
NODE         NAMESPACE   TYPE              ID                                                           VERSION   READY
172.20.0.3   k8s         StaticPodStatus   default/nginx-talos-default-controlplane-2                         2         True
172.20.0.3   k8s         StaticPodStatus   kube-system/kube-apiserver-talos-default-controlplane-2            2         True
172.20.0.3   k8s         StaticPodStatus   kube-system/kube-controller-manager-talos-default-controlplane-2   3         True
172.20.0.3   k8s         StaticPodStatus   kube-system/kube-scheduler-talos-default-controlplane-2            3         True
```
