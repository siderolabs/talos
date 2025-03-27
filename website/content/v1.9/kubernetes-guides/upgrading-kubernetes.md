---
title: "Upgrading Kubernetes"
description: "Guide on how to upgrade the Kubernetes cluster from Talos Linux."
aliases:
  - guides/upgrading-kubernetes
---

This guide covers upgrading Kubernetes on Talos Linux clusters.

For a list of Kubernetes versions compatible with each Talos release, see the [Support Matrix]({{< relref "../introduction/support-matrix" >}}).

For upgrading the Talos Linux operating system, see [Upgrading Talos]({{< relref "../talos-guides/upgrading-talos" >}})

## Video Walkthrough

To see a demo of this process, watch this video:

<iframe width="560" height="315" src="https://www.youtube.com/embed/uOKveKbD8MQ" title="YouTube video player" frameborder="0" allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture" allowfullscreen></iframe>

## Automated Kubernetes Upgrade

The recommended method to upgrade Kubernetes is to use the `talosctl upgrade-k8s` command.
This will automatically update the components needed to upgrade Kubernetes safely.
Upgrading Kubernetes is non-disruptive to the cluster workloads.

To trigger a Kubernetes upgrade, issue a command specifying the version of Kubernetes to ugprade to, such as:

`talosctl --nodes <controlplane node> upgrade-k8s --to {{< k8s_release >}}`

Note that the `--nodes` parameter specifies the control plane node to send the API call to, but all members of the cluster will be upgraded.

To check what will be upgraded you can run `talosctl upgrade-k8s` with the `--dry-run` flag:

```bash
$ talosctl --nodes <controlplane node> upgrade-k8s --to {{< k8s_release >}} --dry-run
WARNING: found resources which are going to be deprecated/migrated in the version {{< k8s_release >}}
RESOURCE                                                               COUNT
validatingwebhookconfigurations.v1beta1.admissionregistration.k8s.io   4
mutatingwebhookconfigurations.v1beta1.admissionregistration.k8s.io     3
customresourcedefinitions.v1beta1.apiextensions.k8s.io                 25
apiservices.v1beta1.apiregistration.k8s.io                             54
leases.v1beta1.coordination.k8s.io                                     4
automatically detected the lowest Kubernetes version {{< k8s_prev_release >}}
checking for resource APIs to be deprecated in version {{< k8s_release >}}
discovered controlplane nodes ["172.20.0.2" "172.20.0.3" "172.20.0.4"]
discovered worker nodes ["172.20.0.5" "172.20.0.6"]
updating "kube-apiserver" to version "{{< k8s_release >}}"
 > "172.20.0.2": starting update
 > update kube-apiserver: v{{< k8s_prev_release >}} -> {{< k8s_release >}}
 > skipped in dry-run
 > "172.20.0.3": starting update
 > update kube-apiserver: v{{< k8s_prev_release >}} -> {{< k8s_release >}}
 > skipped in dry-run
 > "172.20.0.4": starting update
 > update kube-apiserver: v{{< k8s_prev_release >}} -> {{< k8s_release >}}
 > skipped in dry-run
updating "kube-controller-manager" to version "{{< k8s_release >}}"
 > "172.20.0.2": starting update
 > update kube-controller-manager: v{{< k8s_prev_release >}} -> {{< k8s_release >}}
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

To upgrade Kubernetes from v{{< k8s_prev_release >}} to v{{< k8s_release >}} run:

```bash
$ talosctl --nodes <controlplane node> upgrade-k8s --to {{< k8s_release >}}
automatically detected the lowest Kubernetes version {{< k8s_prev_release >}}
checking for resource APIs to be deprecated in version {{< k8s_release >}}
discovered controlplane nodes ["172.20.0.2" "172.20.0.3" "172.20.0.4"]
discovered worker nodes ["172.20.0.5" "172.20.0.6"]
updating "kube-apiserver" to version "{{< k8s_release >}}"
 > "172.20.0.2": starting update
 > update kube-apiserver: v{{< k8s_prev_release >}} -> {{< k8s_release >}}
 > "172.20.0.2": machine configuration patched
 > "172.20.0.2": waiting for API server state pod update
 < "172.20.0.2": successfully updated
 > "172.20.0.3": starting update
 > update kube-apiserver: v{{< k8s_prev_release >}} -> {{< k8s_release >}}
<snip>
```

This command runs in several phases:

1. Images for new Kubernetes components are pre-pulled to the nodes to minimize downtime and test for image availability.
2. Every control plane node machine configuration is patched with the new image version for each control plane component.
   Talos renders new static pod definitions on the configuration update which is picked up by the kubelet.
   The command waits for the change to propagate to the API server state.
3. The command updates the `kube-proxy` daemonset with the new image version.
4. On every node in the cluster, the `kubelet` version is updated.
   The command then waits for the `kubelet` service to be restarted and become healthy.
   The update is verified by checking the `Node` resource state.
5. Kubernetes bootstrap manifests are re-applied to the cluster.
   Updated bootstrap manifests might come with a new Talos version (e.g. CoreDNS version update), or might be the result of machine configuration change.

> Note: The `upgrade-k8s` command never deletes any resources from the cluster: they should be deleted manually.

If the command fails for any reason, it can be safely restarted to continue the upgrade process from the moment of the failure.

> Note: When using custom/overridden Kubernetes component images, use flags `--*-image` to override the default image names.

## Manual Kubernetes Upgrade

Kubernetes can be upgraded manually by following the steps outlined below.
They are equivalent to the steps performed by the `talosctl upgrade-k8s` command.

### Kubeconfig

In order to edit the control plane, you need a working `kubectl` config.
If you don't already have one, you can get one by running:

```bash
talosctl --nodes <controlplane node> kubeconfig
```

### API Server

Patch machine configuration using `talosctl patch` command:

```bash
$ talosctl -n <CONTROL_PLANE_IP_1> patch mc --mode=no-reboot -p '[{"op": "replace", "path": "/cluster/apiServer/image", "value": "registry.k8s.io/kube-apiserver:v{{< k8s_release >}}"}]'
patched mc at the node 172.20.0.2
```

The JSON patch might need to be adjusted if current machine configuration is missing `.cluster.apiServer.image` key.

Also the machine configuration can be edited manually with `talosctl -n <IP>  edit mc --mode=no-reboot`.

Capture the new version of `kube-apiserver` config with:

```bash
$ talosctl -n <CONTROL_PLANE_IP_1> get apiserverconfig -o yaml
node: 172.20.0.2
metadata:
    namespace: controlplane
    type: APIServerConfigs.kubernetes.talos.dev
    id: kube-apiserver
    version: 5
    owner: k8s.ControlPlaneAPIServerController
    phase: running
spec:
    image: registry.k8s.io/kube-apiserver:v{{< k8s_release >}}
    cloudProvider: ""
    controlPlaneEndpoint: https://172.20.0.1:6443
    etcdServers:
        - https://localhost:2379
    localPort: 6443
    serviceCIDR:
        - 10.96.0.0/12
    extraArgs: {}
    extraVolumes: []
    environmentVariables: {}
    podSecurityPolicyEnabled: false
    advertisedAddress: $(POD_IP)
    resources:
        requests:
            cpu: ""
            memory: ""
        limits: {}
```

In this example, the new version is `5`.
Wait for the new pod definition to propagate to the API server state (replace `talos-default-controlplane-1` with the node name):

```bash
$ kubectl get pod -n kube-system -l k8s-app=kube-apiserver --field-selector spec.nodeName=talos-default-controlplane-1 -o jsonpath='{.items[0].metadata.annotations.talos\.dev/config\-version}'
5
```

Check that the pod is running:

```bash
$ kubectl get pod -n kube-system -l k8s-app=kube-apiserver --field-selector spec.nodeName=talos-default-controlplane-1
NAME                                    READY   STATUS    RESTARTS   AGE
kube-apiserver-talos-default-controlplane-1   1/1     Running   0          16m
```

Repeat this process for every control plane node, verifying that state got propagated successfully between each node update.

### Controller Manager

Patch machine configuration using `talosctl patch` command:

```bash
$ talosctl -n <CONTROL_PLANE_IP_1> patch mc --mode=no-reboot -p '[{"op": "replace", "path": "/cluster/controllerManager/image", "value": "registry.k8s.io/kube-controller-manager:v{{< k8s_release >}}"}]'
patched mc at the node 172.20.0.2
```

The JSON patch might need be adjusted if current machine configuration is missing `.cluster.controllerManager.image` key.

Capture new version of `kube-controller-manager` config with:

```bash
$ talosctl -n <CONTROL_PLANE_IP_1> get kcpc controllermanagerconfig -o yaml
node: 172.20.0.2
metadata:
    namespace: controlplane
    type: ControllerManagerConfigs.kubernetes.talos.dev
    id: kube-controller-manager
    version: 3
    owner: k8s.ControlPlaneControllerManagerController
    phase: running
spec:
    enabled: true
    image: registry.k8s.io/kube-controller-manager:v{{< k8s_release >}}
    cloudProvider: ""
    podCIDRs:
        - 10.244.0.0/16
    serviceCIDRs:
        - 10.96.0.0/12
    extraArgs: {}
    extraVolumes: []
    environmentVariables: {}
    resources:
        requests:
            cpu: ""
            memory: ""
        limits: {}
```

In this example, new version is `3`.
Wait for the new pod definition to propagate to the API server state (replace `talos-default-controlplane-1` with the node name):

```bash
$ kubectl get pod -n kube-system -l k8s-app=kube-controller-manager --field-selector spec.nodeName=talos-default-controlplane-1 -o jsonpath='{.items[0].metadata.annotations.talos\.dev/config\-version}'
3
```

Check that the pod is running:

```bash
$ kubectl get pod -n kube-system -l k8s-app=kube-controller-manager --field-selector spec.nodeName=talos-default-controlplane-1
NAME                                             READY   STATUS    RESTARTS   AGE
kube-controller-manager-talos-default-controlplane-1   1/1     Running   0          35m
```

Repeat this process for every control plane node, verifying that state propagated successfully between each node update.

### Scheduler

Patch machine configuration using `talosctl patch` command:

```bash
$ talosctl -n <CONTROL_PLANE_IP_1> patch mc --mode=no-reboot -p '[{"op": "replace", "path": "/cluster/scheduler/image", "value": "registry.k8s.io/kube-scheduler:v{{< k8s_release >}}"}]'
patched mc at the node 172.20.0.2
```

JSON patch might need be adjusted if current machine configuration is missing `.cluster.scheduler.image` key.

Capture new version of `kube-scheduler` config with:

```bash
$ talosctl -n <CONTROL_PLANE_IP_1> get schedulerconfig -o yaml
node: 172.20.0.2
metadata:
    namespace: controlplane
    type: SchedulerConfigs.kubernetes.talos.dev
    id: kube-scheduler
    version: 3
    owner: k8s.ControlPlaneSchedulerController
    phase: running
    created: 2024-11-06T12:37:22Z
    updated: 2024-11-06T12:37:20Z
spec:
    enabled: true
    image: registry.k8s.io/kube-scheduler:v{{< k8s_release >}}
    extraArgs: {}
    extraVolumes: []
    environmentVariables: {}
    resources:
        requests:
            cpu: ""
            memory: ""
        limits: {}
    config: {}
```

In this example, new version is `3`.
Wait for the new pod definition to propagate to the API server state (replace `talos-default-controlplane-1` with the node name):

```bash
$ kubectl get pod -n kube-system -l k8s-app=kube-scheduler --field-selector spec.nodeName=talos-default-controlplane-1 -o jsonpath='{.items[0].metadata.annotations.talos\.dev/config\-version}'
3
```

Check that the pod is running:

```bash
$ kubectl get pod -n kube-system -l k8s-app=kube-scheduler --field-selector spec.nodeName=talos-default-controlplane-1
NAME                                    READY   STATUS    RESTARTS   AGE
kube-scheduler-talos-default-controlplane-1   1/1     Running   0          39m
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
          image: registry.k8s.io/kube-proxy:v{{< k8s_prev_release >}}
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
          image: registry.k8s.io/kube-proxy:v{{< k8s_release >}}
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
talosctl -n <controlplane IP> get manifests -o yaml | yq eval-all '.spec | .[] | splitDoc' - > manifests.yaml
```

Diff the manifests with the cluster:

```bash
kubectl diff -f manifests.yaml
```

Apply the manifests:

```bash
kubectl apply -f manifests.yaml
```

> Note: if some bootstrap resources were removed, they have to be removed from the cluster manually.

### kubelet

For every node, patch machine configuration with new kubelet version, wait for the kubelet to restart with new version:

```bash
$ talosctl -n <IP> patch mc --mode=no-reboot -p '[{"op": "replace", "path": "/machine/kubelet/image", "value": "ghcr.io/siderolabs/kubelet:v{{< k8s_release >}}"}]'
patched mc at the node 172.20.0.2
```

Once `kubelet` restarts with the new configuration, confirm upgrade with `kubectl get nodes <name>`:

```bash
$ kubectl get nodes talos-default-controlplane-1
NAME                           STATUS   ROLES                  AGE    VERSION
talos-default-controlplane-1   Ready    control-plane          123m   v{{< k8s_release >}}
```
