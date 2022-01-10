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

To check what is going to be upgraded you can run `talosctl upgrade-k8s` with `--dry-run` flag:

```bash
$ talosctl --nodes <master node> upgrade-k8s --to 1.23.0 --dry-run
WARNING: found resources which are going to be deprecated/migrated in the version 1.22.0
RESOURCE                                                               COUNT
validatingwebhookconfigurations.v1beta1.admissionregistration.k8s.io   4
mutatingwebhookconfigurations.v1beta1.admissionregistration.k8s.io     3
customresourcedefinitions.v1beta1.apiextensions.k8s.io                 25
apiservices.v1beta1.apiregistration.k8s.io                             54
leases.v1beta1.coordination.k8s.io                                     4
automatically detected the lowest Kubernetes version 1.22.4
checking for resource APIs to be deprecated in version 1.23.0
discovered master nodes ["172.20.0.2" "172.20.0.3" "172.20.0.4"]
discovered worker nodes ["172.20.0.5" "172.20.0.6"]
updating "kube-apiserver" to version "1.23.0"
 > "172.20.0.2": starting update
 > update kube-apiserver: v1.22.4 -> 1.23.0
 > skipped in dry-run
 > "172.20.0.3": starting update
 > update kube-apiserver: v1.22.4 -> 1.23.0
 > skipped in dry-run
 > "172.20.0.4": starting update
 > update kube-apiserver: v1.22.4 -> 1.23.0
 > skipped in dry-run
updating "kube-controller-manager" to version "1.23.0"
 > "172.20.0.2": starting update
 > update kube-controller-manager: v1.22.4 -> 1.23.0
 > skipped in dry-run
 > "172.20.0.3": starting update
 > update kube-controller-manager: v1.22.4 -> 1.23.0
 > skipped in dry-run
 > "172.20.0.4": starting update
 > update kube-controller-manager: v1.22.4 -> 1.23.0
 > skipped in dry-run
updating "kube-scheduler" to version "1.23.0"
 > "172.20.0.2": starting update
 > update kube-scheduler: v1.22.4 -> 1.23.0
 > skipped in dry-run
 > "172.20.0.3": starting update
 > update kube-scheduler: v1.22.4 -> 1.23.0
 > skipped in dry-run
 > "172.20.0.4": starting update
 > update kube-scheduler: v1.22.4 -> 1.23.0
 > skipped in dry-run
updating daemonset "kube-proxy" to version "1.23.0"
skipped in dry-run
updating kubelet to version "1.23.0"
 > "172.20.0.2": starting update
 > update kubelet: v1.22.4 -> 1.23.0
 > skipped in dry-run
 > "172.20.0.3": starting update
 > update kubelet: v1.22.4 -> 1.23.0
 > skipped in dry-run
 > "172.20.0.4": starting update
 > update kubelet: v1.22.4 -> 1.23.0
 > skipped in dry-run
 > "172.20.0.5": starting update
 > update kubelet: v1.22.4 -> 1.23.0
 > skipped in dry-run
 > "172.20.0.6": starting update
 > update kubelet: v1.22.4 -> 1.23.0
 > skipped in dry-run
updating manifests
 > apply manifest Secret bootstrap-token-3lb63t
 > apply skipped in dry run
 > apply manifest ClusterRoleBinding system-bootstrap-approve-node-client-csr
 > apply skipped in dry run
 > apply manifest ClusterRoleBinding system-bootstrap-node-bootstrapper
 > apply skipped in dry run
 > apply manifest ClusterRoleBinding system-bootstrap-node-renewal
 > apply skipped in dry run
 > apply manifest ClusterRoleBinding system:default-sa
 > apply skipped in dry run
 > apply manifest ClusterRole psp:privileged
 > apply skipped in dry run
 > apply manifest ClusterRoleBinding psp:privileged
 > apply skipped in dry run
 > apply manifest PodSecurityPolicy privileged
 > apply skipped in dry run
 > apply manifest ClusterRole flannel
 > apply skipped in dry run
 > apply manifest ClusterRoleBinding flannel
 > apply skipped in dry run
 > apply manifest ServiceAccount flannel
 > apply skipped in dry run
 > apply manifest ConfigMap kube-flannel-cfg
 > apply skipped in dry run
 > apply manifest DaemonSet kube-flannel
 > apply skipped in dry run
 > apply manifest ServiceAccount kube-proxy
 > apply skipped in dry run
 > apply manifest ClusterRoleBinding kube-proxy
 > apply skipped in dry run
 > apply manifest ServiceAccount coredns
 > apply skipped in dry run
 > apply manifest ClusterRoleBinding system:coredns
 > apply skipped in dry run
 > apply manifest ClusterRole system:coredns
 > apply skipped in dry run
 > apply manifest ConfigMap coredns
 > apply skipped in dry run
 > apply manifest Deployment coredns
 > apply skipped in dry run
 > apply manifest Service kube-dns
 > apply skipped in dry run
 > apply manifest ConfigMap kubeconfig-in-cluster
 > apply skipped in dry run
```

To upgrade Kubernetes from v1.22.4 to v1.23.0 run:

```bash
$ talosctl --nodes <master node> upgrade-k8s --to 1.24.0
automatically detected the lowest Kubernetes version 1.22.4
checking for resource APIs to be deprecated in version 1.23.0
discovered master nodes ["172.20.0.2" "172.20.0.3" "172.20.0.4"]
discovered worker nodes ["172.20.0.5" "172.20.0.6"]
updating "kube-apiserver" to version "1.23.0"
 > "172.20.0.2": starting update
 > update kube-apiserver: v1.22.4 -> 1.23.0
 > "172.20.0.2": machine configuration patched
 > "172.20.0.2": waiting for API server state pod update
 < "172.20.0.2": successfully updated
 > "172.20.0.3": starting update
 > update kube-apiserver: v1.22.4 -> 1.23.0
 > "172.20.0.3": machine configuration patched
 > "172.20.0.3": waiting for API server state pod update
 < "172.20.0.3": successfully updated
 > "172.20.0.4": starting update
 > update kube-apiserver: v1.22.4 -> 1.23.0
 > "172.20.0.4": machine configuration patched
 > "172.20.0.4": waiting for API server state pod update
 < "172.20.0.4": successfully updated
updating "kube-controller-manager" to version "1.23.0"
 > "172.20.0.2": starting update
 > update kube-controller-manager: v1.22.4 -> 1.23.0
 > "172.20.0.2": machine configuration patched
 > "172.20.0.2": waiting for API server state pod update
 < "172.20.0.2": successfully updated
 > "172.20.0.3": starting update
 > update kube-controller-manager: v1.22.4 -> 1.23.0
 > "172.20.0.3": machine configuration patched
 > "172.20.0.3": waiting for API server state pod update
 < "172.20.0.3": successfully updated
 > "172.20.0.4": starting update
 > update kube-controller-manager: v1.22.4 -> 1.23.0
 > "172.20.0.4": machine configuration patched
 > "172.20.0.4": waiting for API server state pod update
 < "172.20.0.4": successfully updated
updating "kube-scheduler" to version "1.23.0"
 > "172.20.0.2": starting update
 > update kube-scheduler: v1.22.4 -> 1.23.0
 > "172.20.0.2": machine configuration patched
 > "172.20.0.2": waiting for API server state pod update
 < "172.20.0.2": successfully updated
 > "172.20.0.3": starting update
 > update kube-scheduler: v1.22.4 -> 1.23.0
 > "172.20.0.3": machine configuration patched
 > "172.20.0.3": waiting for API server state pod update
 < "172.20.0.3": successfully updated
 > "172.20.0.4": starting update
 > update kube-scheduler: v1.22.4 -> 1.23.0
 > "172.20.0.4": machine configuration patched
 > "172.20.0.4": waiting for API server state pod update
 < "172.20.0.4": successfully updated
updating daemonset "kube-proxy" to version "1.23.0"
updating kubelet to version "1.23.0"
 > "172.20.0.2": starting update
 > update kubelet: v1.22.4 -> 1.23.0
 > "172.20.0.2": machine configuration patched
 > "172.20.0.2": waiting for kubelet restart
 > "172.20.0.2": waiting for node update
 < "172.20.0.2": successfully updated
 > "172.20.0.3": starting update
 > update kubelet: v1.22.4 -> 1.23.0
 > "172.20.0.3": machine configuration patched
 > "172.20.0.3": waiting for kubelet restart
 > "172.20.0.3": waiting for node update
 < "172.20.0.3": successfully updated
 > "172.20.0.4": starting update
 > update kubelet: v1.22.4 -> 1.23.0
 > "172.20.0.4": machine configuration patched
 > "172.20.0.4": waiting for kubelet restart
 > "172.20.0.4": waiting for node update
 < "172.20.0.4": successfully updated
 > "172.20.0.5": starting update
 > update kubelet: v1.22.4 -> 1.23.0
 > "172.20.0.5": machine configuration patched
 > "172.20.0.5": waiting for kubelet restart
 > "172.20.0.5": waiting for node update
 < "172.20.0.5": successfully updated
 > "172.20.0.6": starting update
 > update kubelet: v1.22.4 -> 1.23.0
 > "172.20.0.6": machine configuration patched
 > "172.20.0.6": waiting for kubelet restart
 > "172.20.0.6": waiting for node update
 < "172.20.0.6": successfully updated
updating manifests
 > apply manifest Secret bootstrap-token-3lb63t
 > apply skipped: nothing to update
 > apply manifest ClusterRoleBinding system-bootstrap-approve-node-client-csr
 > apply skipped: nothing to update
 > apply manifest ClusterRoleBinding system-bootstrap-node-bootstrapper
 > apply skipped: nothing to update
 > apply manifest ClusterRoleBinding system-bootstrap-node-renewal
 > apply skipped: nothing to update
 > apply manifest ClusterRoleBinding system:default-sa
 > apply skipped: nothing to update
 > apply manifest ClusterRole psp:privileged
 > apply skipped: nothing to update
 > apply manifest ClusterRoleBinding psp:privileged
 > apply skipped: nothing to update
 > apply manifest PodSecurityPolicy privileged
 > apply skipped: nothing to update
 > apply manifest ClusterRole flannel
 > apply skipped: nothing to update
 > apply manifest ClusterRoleBinding flannel
 > apply skipped: nothing to update
 > apply manifest ServiceAccount flannel
 > apply skipped: nothing to update
 > apply manifest ConfigMap kube-flannel-cfg
 > apply skipped: nothing to update
 > apply manifest DaemonSet kube-flannel
 > apply skipped: nothing to update
 > apply manifest ServiceAccount kube-proxy
 > apply skipped: nothing to update
 > apply manifest ClusterRoleBinding kube-proxy
 > apply skipped: nothing to update
 > apply manifest ServiceAccount coredns
 > apply skipped: nothing to update
 > apply manifest ClusterRoleBinding system:coredns
 > apply skipped: nothing to update
 > apply manifest ClusterRole system:coredns
 > apply skipped: nothing to update
 > apply manifest ConfigMap coredns
 > apply skipped: nothing to update
 > apply manifest Deployment coredns
 > apply skipped: nothing to update
 > apply manifest Service kube-dns
 > apply skipped: nothing to update
 > apply manifest ConfigMap kubeconfig-in-cluster
 > apply skipped: nothing to update
```

Script runs in several phases:

1. Every control plane node machine configuration is patched with new image version for each control plane component.
   Talos renders new static pod definition on configuration update which is picked up by the kubelet.
   Script waits for the change to propagate to the API server state.
2. The script updates `kube-proxy` daemonset with the new image version.
3. On every node in the cluster, `kubelet` version is updated.
   The script waits for the `kubelet` service to be restarted, become healthy.
   Update is verified with the `Node` resource state.
4. Kubernetes bootstrap manifests are re-applied to the cluster.
   The script never deletes any resources from the cluster, they should be deleted manually.
   Updated bootstrap manifests might come with new Talos version (e.g. CoreDNS version update), or might be result of machine configuration change.

If the script fails for any reason, it can be safely restarted to continue upgrade process from the moment of the failure.

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
$ talosctl -n <CONTROL_PLANE_IP_1> patch mc --mode=no-reboot -p '[{"op": "replace", "path": "/cluster/apiServer/image", "value": "k8s.gcr.io/kube-apiserver:v1.20.4"}]'
patched mc at the node 172.20.0.2
```

JSON patch might need to be adjusted if current machine configuration is missing `.cluster.apiServer.image` key.

Also machine configuration can be edited manually with `talosctl -n <IP>  edit mc --mode=no-reboot`.

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
$ talosctl -n <CONTROL_PLANE_IP_1> patch mc --mode=no-reboot -p '[{"op": "replace", "path": "/cluster/controllerManager/image", "value": "k8s.gcr.io/kube-controller-manager:v1.20.4"}]'
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
$ talosctl -n <CONTROL_PLANE_IP_1> patch mc --mode=no-reboot -p '[{"op": "replace", "path": "/cluster/scheduler/image", "value": "k8s.gcr.io/kube-scheduler:v1.20.4"}]'
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

### Bootstrap Manifests

Bootstrap manifests can be retrieved in a format which works for `kubectl` with the following command:

```bash
talosctl -n <master IP> get manifests -o yaml | yq eval-all '.spec | .[] | splitDoc' - > manifests.yaml
```

Diff the manifests with the cluster:

```bash
kubectl diff -f manifests.yaml
```

Apply the manifests:

```bash
kubectl apply -f manifests.yaml
```

> Note: if some boostrap resources were removed, they have to be removed from the cluster manually.

### kubelet

For every node, patch machine configuration with new kubelet version, wait for the kubelet to restart with new version:

```bash
$ talosctl -n <IP> patch mc --mode=no-reboot -p '[{"op": "replace", "path": "/machine/kubelet/image", "value": "ghcr.io/talos-systems/kubelet:v1.23.0"}]'
patched mc at the node 172.20.0.2
```

Once `kubelet` restarts with the new configuration, confirm upgrade with `kubectl get nodes <name>`:

```bash
$ kubectl get nodes talos-default-master-1
NAME                     STATUS   ROLES                  AGE    VERSION
talos-default-master-1   Ready    control-plane,master   123m   v1.23.0
```
