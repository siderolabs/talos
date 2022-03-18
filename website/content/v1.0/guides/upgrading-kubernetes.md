---
title: Upgrading Kubernetes
---

This guide covers upgrading Kubernetes on Talos Linux clusters.
For upgrading the Talos Linux operating system, see [Upgrading Talos](../upgrading-talos/)

## Video Walkthrough

To see a demo of this process, watch this video:

<iframe width="560" height="315" src="https://www.youtube.com/embed/uOKveKbD8MQ" title="YouTube video player" frameborder="0" allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture" allowfullscreen></iframe>

## Automated Kubernetes Upgrade

The recommended method to upgrade Kubernetes is to use the `talosctl upgrade-k8s` command.
This will automatically update the components needed to upgrade Kubernetes safely.
Upgrading Kubernetes is non-disruptive to the cluster workloads.

To trigger a Kubernetes upgrade, issue a command specifiying the version of Kubernetes to ugprade to, such as:

`talosctl --nodes <master node> upgrade-k8s --to 1.23.0`

Note that the `--nodes` parameter specifies the control plane node to send the API call to, but all members of the cluster will be upgraded.

To check what will be upgraded you can run `talosctl upgrade-k8s` with the `--dry-run` flag:

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

<snip>

updating manifests
 > apply manifest Secret bootstrap-token-3lb63t
 > apply skipped in dry run
 > apply manifest ClusterRoleBinding system-bootstrap-approve-node-client-csr
 > apply skipped in dry run
<snip>
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
<snip>
```

This command runs in several phases:

1. Every control plane node machine configuration is patched with the new image version for each control plane component.
   Talos renders new static pod definitions on the configuration update which is picked up by the kubelet.
   The command waits for the change to propagate to the API server state.
2. The command updates the `kube-proxy` daemonset with the new image version.
3. On every node in the cluster, the `kubelet` version is updated.
   The command then waits for the `kubelet` service to be restarted and become healthy.
   The update is verified by checking the `Node` resource state.
4. Kubernetes bootstrap manifests are re-applied to the cluster.
   Updated bootstrap manifests might come with a new Talos version (e.g. CoreDNS version update), or might be the result of machine configuration change.
   Note: The `upgrade-k8s` command never deletes any resources from the cluster: they should be deleted manually.

If the command fails for any reason, it can be safely restarted to continue the upgrade process from the moment of the failure.

## Manual Kubernetes Upgrade

Kubernetes can be upgraded manually by following the steps outlined below.
They are equivalent to the steps performed by the `talosctl upgrade-k8s` command.

### Kubeconfig

In order to edit the control plane, you need a working `kubectl` config.
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

The JSON patch might need to be adjusted if current machine configuration is missing `.cluster.apiServer.image` key.

Also the machine configuration can be edited manually with `talosctl -n <IP>  edit mc --mode=no-reboot`.

Capture the new version of `kube-apiserver` config with:

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

In this example, the new version is `5`.
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

The JSON patch might need be adjusted if current machine configuration is missing `.cluster.controllerManager.image` key.

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

Repeat this process for every control plane node, verifying that state propagated successfully between each node update.

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
