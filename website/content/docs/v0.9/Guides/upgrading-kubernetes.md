---
title: Upgrading Kubernetes
---

This guide covers Kubernetes control plane upgrade for clusters running Talos-managed control plane.
If the cluster is still running self-hosted control plane (after upgrade from Talos 0.8), please
refer to 0.8 docs.

## Video Walkthrough

To see a live demo of this writeup, see the video below:

<iframe width="560" height="315" src="https://www.youtube.com/embed/_N_vhB_ZI2c" frameborder="0" allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture" allowfullscreen></iframe>

## Automated Kubernetes Upgrade

To upgrade from Kubernetes v1.20.1 to v1.20.4 run:

```bash
$ talosctl --nodes <master node> upgrade-k8s --from 1.20.1 --to 1.20.4
discovered master nodes ["172.20.0.2" "172.20.0.3" "172.20.0.4"]
updating "kube-apiserver" to version "1.20.4"
 > updating node "172.20.0.2"
2021/03/09 19:55:01 retrying error: config version mismatch: got "2", expected "3"
 > updating node "172.20.0.3"
2021/03/09 19:55:05 retrying error: config version mismatch: got "2", expected "3"
 > updating node "172.20.0.4"
2021/03/09 19:55:07 retrying error: config version mismatch: got "2", expected "3"
updating "kube-controller-manager" to version "1.20.4"
 > updating node "172.20.0.2"
2021/03/09 19:55:27 retrying error: config version mismatch: got "2", expected "3"
 > updating node "172.20.0.3"
2021/03/09 19:55:47 retrying error: config version mismatch: got "2", expected "3"
 > updating node "172.20.0.4"
2021/03/09 19:56:07 retrying error: config version mismatch: got "2", expected "3"
updating "kube-scheduler" to version "1.20.4"
 > updating node "172.20.0.2"
2021/03/09 19:56:27 retrying error: config version mismatch: got "2", expected "3"
 > updating node "172.20.0.3"
2021/03/09 19:56:47 retrying error: config version mismatch: got "2", expected "3"
 > updating node "172.20.0.4"
2021/03/09 19:57:08 retrying error: config version mismatch: got "2", expected "3"
updating daemonset "kube-proxy" to version "1.20.4"
```

Script runs in two phases:

1. In the first phase every control plane node machine configuration is patched with new image version for each control plane component.
   Talos renders new static pod definition on configuration update which is picked up by the kubelet.
   Script waits for the change to propagate to the API server state.
   Messages `config version mismatch` indicate that script is waiting for the updated container to be registered in the API server.
2. In the second phase script updates `kube-proxy` daemonset with the new image version.

If script fails for any reason, it can be safely restarted to continue upgrade process.

## Manual Kubernetes Upgrade

Kubernetes can be upgraded manually as well by following the steps outlined below.
They are equivalent to the steps performed by the `talosctl upgrade-k8s` command.

### Kubeconfig

In order to edit the control plane, we will need a working `kubectl` config.
If you don't already have one, you can get one by running:

```bash
talosctl --nodes <master node> kubeconfig
```

### API Server

Patch machine configuration using `talosctl patch` command:

```bash
$ talosctl -n <CONTROL_PLANE_IP_1> patch mc --immediate -p '[{"op": "replace", "path": "/cluster/apiServer/image", "value": "k8s.gcr.io/kube-apiserver:v1.20.4"}]'
patched mc at the node 172.20.0.2
```

JSON patch might need to be adjusted if current machine configuration is missing `.cluster.apiServer.image` key.

Also machine configuration can be edited manually with `talosctl -n <IP>  edit mc --immediate`.

Capture new version of `kube-apiserver` config with:

```bash
$ talosctl -n <CONTROL_PLANE_IP_1> get kcpc kube-apiserver -o yaml
node: 172.20.0.2
metadata:
    namespace: config
    type: KubernetesControlPlaneConfigs.config.talos.dev
    id: kube-apiserver
    version: 5
    phase: running
spec:
    image: k8s.gcr.io/kube-apiserver:v1.20.4
    cloudProvider: ""
    controlPlaneEndpoint: https://172.20.0.1:6443
    etcdServers:
        - https://127.0.0.1:2379
    localPort: 6443
    serviceCIDR: 10.96.0.0/12
    extraArgs: {}
    extraVolumes: []
```

In this example, new version is `5`.
Wait for the new pod definition to propagate to the API server state (replace `talos-default-master-1` with the node name):

```bash
$ kubectl get pod -n kube-system -l k8s-app=kube-apiserver --field-selector spec.nodeName=talos-default-master-1 -o jsonpath='{.items[0].metadata.annotations.talos\.dev/config\-version}'
5
```

Check that the pod is running:

```bash
$ kubectl get pod -n kube-system -l k8s-app=kube-apiserver --field-selector spec.nodeName=talos-default-master-1
NAME                                    READY   STATUS    RESTARTS   AGE
kube-apiserver-talos-default-master-1   1/1     Running   0          16m
```

Repeat this process for every control plane node, verifying that state got propagated successfully between each node update.

### Controller Manager

Patch machine configuration using `talosctl patch` command:

```bash
$ talosctl -n <CONTROL_PLANE_IP_1> patch mc --immediate -p '[{"op": "replace", "path": "/cluster/controllerManager/image", "value": "k8s.gcr.io/kube-controller-manager:v1.20.4"}]'
patched mc at the node 172.20.0.2
```

JSON patch might need be adjusted if current machine configuration is missing `.cluster.controllerManager.image` key.

Capture new version of `kube-controller-manager` config with:

```bash
$ talosctl -n <CONTROL_PLANE_IP_1> get kcpc kube-controller-manager -o yaml
node: 172.20.0.2
metadata:
    namespace: config
    type: KubernetesControlPlaneConfigs.config.talos.dev
    id: kube-controller-manager
    version: 3
    phase: running
spec:
    image: k8s.gcr.io/kube-controller-manager:v1.20.4
    cloudProvider: ""
    podCIDR: 10.244.0.0/16
    serviceCIDR: 10.96.0.0/12
    extraArgs: {}
    extraVolumes: []
```

In this example, new version is `3`.
Wait for the new pod definition to propagate to the API server state (replace `talos-default-master-1` with the node name):

```bash
$ kubectl get pod -n kube-system -l k8s-app=kube-controller-manager --field-selector spec.nodeName=talos-default-master-1 -o jsonpath='{.items[0].metadata.annotations.talos\.dev/config\-version}'
3
```

Check that the pod is running:

```bash
$ kubectl get pod -n kube-system -l k8s-app=kube-controller-manager --field-selector spec.nodeName=talos-default-master-1
NAME                                             READY   STATUS    RESTARTS   AGE
kube-controller-manager-talos-default-master-1   1/1     Running   0          35m
```

Repeat this process for every control plane node, verifying that state got propagated successfully between each node update.

### Scheduler

Patch machine configuration using `talosctl patch` command:

```bash
$ talosctl -n <CONTROL_PLANE_IP_1> patch mc --immediate -p '[{"op": "replace", "path": "/cluster/scheduler/image", "value": "k8s.gcr.io/kube-scheduler:v1.20.4"}]'
patched mc at the node 172.20.0.2
```

JSON patch might need be adjusted if current machine configuration is missing `.cluster.scheduler.image` key.

Capture new version of `kube-scheduler` config with:

```bash
$ talosctl -n <CONTROL_PLANE_IP_1> get kcpc kube-scheduler -o yaml
node: 172.20.0.2
metadata:
    namespace: config
    type: KubernetesControlPlaneConfigs.config.talos.dev
    id: kube-scheduler
    version: 3
    phase: running
spec:
    image: k8s.gcr.io/kube-scheduler:v1.20.4
    extraArgs: {}
    extraVolumes: []
```

In this example, new version is `3`.
Wait for the new pod definition to propagate to the API server state (replace `talos-default-master-1` with the node name):

```bash
$ kubectl get pod -n kube-system -l k8s-app=kube-scheduler --field-selector spec.nodeName=talos-default-master-1 -o jsonpath='{.items[0].metadata.annotations.talos\.dev/config\-version}'
3
```

Check that the pod is running:

```bash
$ kubectl get pod -n kube-system -l k8s-app=kube-scheduler --field-selector spec.nodeName=talos-default-master-1
NAME                                    READY   STATUS    RESTARTS   AGE
kube-scheduler-talos-default-master-1   1/1     Running   0          39m
```

Repeat this process for every control plane node, verifying that state got propagated successfully between each node update.

### Proxy

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
          image: k8s.gcr.io/kube-proxy:v1.20.1
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
          image: k8s.gcr.io/kube-proxy:v1.20.4
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

## Kubelet

Upgrading Kubelet version requires Talos node reboot after machine configuration change.

For every node, patch machine configuration with new kubelet version, wait for the node to reboot:

```bash
$ talosctl -n <IP> patch mc -p '[{"op": "replace", "path": "/machine/kubelet/image", "value": "ghcr.io/talos-systems/kubelet:v1.20.4"}]'
patched mc at the node 172.20.0.2
```

Once node boots with the new configuration, confirm upgrade with `kubectl get nodes <name>`:

```bash
$ kubectl get nodes talos-default-master-1
NAME                     STATUS   ROLES                  AGE    VERSION
talos-default-master-1   Ready    control-plane,master   123m   v1.20.4
```
