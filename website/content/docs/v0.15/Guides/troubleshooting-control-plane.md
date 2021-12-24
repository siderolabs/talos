---
title: "Troubleshooting Control Plane"
description: "Troubleshoot control plane failures for running cluster and bootstrap process."
---

<!-- markdownlint-disable MD026 -->

This guide is written as series of topics and detailed answers for each topic.
It starts with basics of control plane and goes into Talos specifics.

In this guide we assume that Talos client config is available and Talos API access is available.
Kubernetes client configuration can be pulled from control plane nodes with `talosctl -n <IP> kubeconfig`
(this command works before Kubernetes is fully booted).

### What is a control plane node?

Talos nodes which have `.machine.type` of `init` and `controlplane` are control plane nodes.

The only difference between `init` and `controlplane` nodes is that `init` node automatically
bootstraps a single-node `etcd` cluster on a first boot if the etcd data directory is empty.
A node with type `init` can be replaced with a `controlplane` node which is triggered to run etcd bootstrap
with `talosctl --nodes <IP> bootstrap` command.

Use of `init` type nodes is discouraged, as it might lead to split-brain scenario if one node in
existing cluster is reinstalled while config type is still `init`.

It is critical to make sure only one control plane runs in bootstrap mode (either with node type `init` or
via bootstrap API/`talosctl bootstrap`), as having more than node in bootstrap mode leads to split-brain
scenario (multiple etcd clusters are built instead of a single cluster).

### What is special about control plane node?

Control plane nodes in Talos run `etcd` which provides data store for Kubernetes and Kubernetes control plane
components (`kube-apiserver`, `kube-controller-manager` and `kube-scheduler`).

Control plane nodes are tainted by default to prevent workloads from being scheduled to control plane nodes.

### How many control plane nodes should be deployed?

With a single control plane node, cluster is not HA: if that single node experiences hardware failure, cluster
control plane is broken and can't be recovered.
Single control plane node clusters are still used as test clusters and in edge deployments, but it should be noted that this setup is not HA.

Number of control plane should be odd (1, 3, 5, ...), as with even number of nodes, etcd quorum doesn't tolerate
failures correctly: e.g. with 2 control plane nodes quorum is 2, so failure of any node breaks quorum, so this
setup is almost equivalent to single control plane node cluster.

With three control plane nodes cluster can tolerate a failure of any single control plane node.
With five control plane nodes cluster can tolerate failure of any two control plane nodes.

### What is control plane endpoint?

Kubernetes requires having a control plane endpoint which points to any healthy API server running on a control plane node.
Control plane endpoint is specified as URL like `https://endpoint:6443/`.
At any point in time, even during failures control plane endpoint should point to a healthy API server instance.
As `kube-apiserver` runs with host network, control plane endpoint should point to one of the control plane node IPs: `node1:6443`, `node2:6443`, ...

For single control plane node clusters, control plane endpoint might be `https://IP:6443/` or `https://DNS:6443/`, where `IP` is the IP of the control plane node and `DNS` points to `IP`.
DNS form of the endpoint allows to change the IP address of the control plane if that IP changes over time.

For HA clusters, control plane can be implemented as:

* TCP L7 loadbalancer with active health checks against port 6443
* round-robin DNS with active health checks against port 6443
* BGP anycast IP with health checks
* virtual shared L2 IP
<!-- TODO link to the guide -->

It is critical that control plane endpoint works correctly during cluster bootstrap phase, as nodes discover
each other using control plane endpoint.

### kubelet is not running on control plane node

Service `kubelet` should be running on control plane node as soon as networking is configured:

```bash
$ talosctl -n <IP> service kubelet
NODE     172.20.0.2
ID       kubelet
STATE    Running
HEALTH   OK
EVENTS   [Running]: Health check successful (2m54s ago)
         [Running]: Health check failed: Get "http://127.0.0.1:10248/healthz": dial tcp 127.0.0.1:10248: connect: connection refused (3m4s ago)
         [Running]: Started task kubelet (PID 2334) for container kubelet (3m6s ago)
         [Preparing]: Creating service runner (3m6s ago)
         [Preparing]: Running pre state (3m15s ago)
         [Waiting]: Waiting for service "timed" to be "up" (3m15s ago)
         [Waiting]: Waiting for service "cri" to be "up", service "timed" to be "up" (3m16s ago)
         [Waiting]: Waiting for service "cri" to be "up", service "networkd" to be "up", service "timed" to be "up" (3m18s ago)
```

If `kubelet` is not running, it might be caused by wrong configuration, check `kubelet` logs
with `talosctl logs`:

```bash
$ talosctl -n <IP> logs kubelet
172.20.0.2: I0305 20:45:07.756948    2334 controller.go:101] kubelet config controller: starting controller
172.20.0.2: I0305 20:45:07.756995    2334 controller.go:267] kubelet config controller: ensuring filesystem is set up correctly
172.20.0.2: I0305 20:45:07.757000    2334 fsstore.go:59] kubelet config controller: initializing config checkpoints directory "/etc/kubernetes/kubelet/store"
```

### etcd is not running on bootstrap node

`etcd` should be running on bootstrap node immediately (bootstrap node is either `init` node or `controlplane` node
after `talosctl bootstrap` command was issued).
When node boots for the first time, `etcd` data directory `/var/lib/etcd` directory is empty and Talos launches `etcd` in a mode to build the initial cluster of a single node.
At this time `/var/lib/etcd` directory becomes non-empty and `etcd` runs as usual.

If `etcd` is not running, check service `etcd` state:

```bash
$ talosctl -n <IP> service etcd
NODE     172.20.0.2
ID       etcd
STATE    Running
HEALTH   OK
EVENTS   [Running]: Health check successful (3m21s ago)
         [Running]: Started task etcd (PID 2343) for container etcd (3m26s ago)
         [Preparing]: Creating service runner (3m26s ago)
         [Preparing]: Running pre state (3m26s ago)
         [Waiting]: Waiting for service "cri" to be "up", service "networkd" to be "up", service "timed" to be "up" (3m26s ago)
```

If service is stuck in `Preparing` state for bootstrap node, it might be related to slow network - at this stage
Talos pulls `etcd` image from the container registry.

If `etcd` service is crashing and restarting, check service logs with `talosctl -n <IP> logs etcd`.
Most common reasons for crashes are:

* wrong arguments passed via `extraArgs` in the configuration;
* booting Talos on non-empty disk with previous Talos installation, `/var/lib/etcd` contains data from old cluster.

### etcd is not running on non-bootstrap control plane node

Service `etcd` on non-bootstrap control plane node waits for Kubernetes to boot successfully on bootstrap node to find
other peers to build a cluster.
As soon as bootstrap node boots Kubernetes control plane components, and `kubectl get endpoints` returns IP of bootstrap control plane node, other control plane nodes will start joining the cluster followed by Kubernetes control plane components on each control plane node.

### Kubernetes static pod definitions are not generated

Talos should write down static pod definitions for the Kubernetes control plane:

```bash
$ talosctl -n <IP> ls /etc/kubernetes/manifests
NODE         NAME
172.20.0.2   .
172.20.0.2   talos-kube-apiserver.yaml
172.20.0.2   talos-kube-controller-manager.yaml
172.20.0.2   talos-kube-scheduler.yaml
```

If static pod definitions are not rendered, check `etcd` and `kubelet` service health (see above),
and controller runtime logs (`talosctl logs controller-runtime`).

### Talos prints error `an error on the server ("") has prevented the request from succeeding`

This is expected during initial cluster bootstrap and sometimes after a reboot:

```bash
[   70.093289] [talos] task labelNodeAsMaster (1/1): starting
[   80.094038] [talos] retrying error: an error on the server ("") has prevented the request from succeeding (get nodes talos-default-master-1)
```

Initially `kube-apiserver` component is not running yet, and it takes some time before it becomes fully up
during bootstrap (image should be pulled from the Internet, etc.)
Once control plane endpoint is up Talos should proceed.

If Talos doesn't proceed further, it might be a configuration issue.

In any case, status of control plane components can be checked with `talosctl containers -k`:

```bash
$ talosctl -n <IP> containers --kubernetes
NODE         NAMESPACE   ID                                                                                      IMAGE                                        PID    STATUS
172.20.0.2   k8s.io      kube-system/kube-apiserver-talos-default-master-1                                       k8s.gcr.io/pause:3.2                         2539   SANDBOX_READY
172.20.0.2   k8s.io      └─ kube-system/kube-apiserver-talos-default-master-1:kube-apiserver                     k8s.gcr.io/kube-apiserver:v1.20.4            2572   CONTAINER_RUNNING
```

If `kube-apiserver` shows as `CONTAINER_EXITED`, it might have exited due to configuration error.
Logs can be checked with `taloctl logs --kubernetes` (or with `-k` as a shorthand):

```bash
$ talosctl -n <IP> logs -k kube-system/kube-apiserver-talos-default-master-1:kube-apiserver
172.20.0.2: 2021-03-05T20:46:13.133902064Z stderr F 2021/03/05 20:46:13 Running command:
172.20.0.2: 2021-03-05T20:46:13.133933824Z stderr F Command env: (log-file=, also-stdout=false, redirect-stderr=true)
172.20.0.2: 2021-03-05T20:46:13.133938524Z stderr F Run from directory:
172.20.0.2: 2021-03-05T20:46:13.13394154Z stderr F Executable path: /usr/local/bin/kube-apiserver
...
```

### Talos prints error `nodes "talos-default-master-1" not found`

This error means that `kube-apiserver` is up, and control plane endpoint is healthy, but `kubelet` hasn't got
its client certificate yet and wasn't able to register itself.

For the `kubelet` to get its client certificate, following conditions should apply:

* control plane endpoint is healthy (`kube-apiserver` is running)
* bootstrap manifests got successfully deployed (for CSR auto-approval)
* `kube-controller-manager` is running

CSR state can be checked with `kubectl get csr`:

```bash
$ kubectl get csr
NAME        AGE   SIGNERNAME                                    REQUESTOR                 CONDITION
csr-jcn9j   14m   kubernetes.io/kube-apiserver-client-kubelet   system:bootstrap:q9pyzr   Approved,Issued
csr-p6b9q   14m   kubernetes.io/kube-apiserver-client-kubelet   system:bootstrap:q9pyzr   Approved,Issued
csr-sw6rm   14m   kubernetes.io/kube-apiserver-client-kubelet   system:bootstrap:q9pyzr   Approved,Issued
csr-vlghg   14m   kubernetes.io/kube-apiserver-client-kubelet   system:bootstrap:q9pyzr   Approved,Issued
```

### Talos prints error `node not ready`

Node in Kubernetes is marked as `Ready` once CNI is up.
It takes a minute or two for the CNI images to be pulled and for the CNI to start.
If the node is stuck in this state for too long, check CNI pods and logs with `kubectl`, usually
CNI resources are created in `kube-system` namespace.
For example, for Talos default Flannel CNI:

```bash
$ kubectl -n kube-system get pods
NAME                                             READY   STATUS    RESTARTS   AGE
...
kube-flannel-25drx                               1/1     Running   0          23m
kube-flannel-8lmb6                               1/1     Running   0          23m
kube-flannel-gl7nx                               1/1     Running   0          23m
kube-flannel-jknt9                               1/1     Running   0          23m
...
```

### Talos prints error `x509: certificate signed by unknown authority`

Full error might look like:

```bash
x509: certificate signed by unknown authority (possiby because of crypto/rsa: verification error" while trying to verify candidate authority certificate "kubernetes"
```

Commonly, the control plane endpoint points to a different cluster, as the client certificate
generated by Talos doesn't match CA of the cluster at control plane endpoint.

### etcd is running on bootstrap node, but stuck in `pre` state on non-bootstrap nodes

Please see question `etcd is not running on non-bootstrap control plane node`.

### Checking `kube-controller-manager` and `kube-scheduler`

If control plane endpoint is up, status of the pods can be performed with `kubectl`:

```bash
$ kubectl get pods -n kube-system -l k8s-app=kube-controller-manager
NAME                                             READY   STATUS    RESTARTS   AGE
kube-controller-manager-talos-default-master-1   1/1     Running   0          28m
kube-controller-manager-talos-default-master-2   1/1     Running   0          28m
kube-controller-manager-talos-default-master-3   1/1     Running   0          28m
```

If control plane endpoint is not up yet, container status can be queried with
`talosctl containers --kubernetes`:

```bash
$ talosctl -n <IP> c -k
NODE         NAMESPACE   ID                                                                                      IMAGE                                        PID    STATUS
...
172.20.0.2   k8s.io      kube-system/kube-controller-manager-talos-default-master-1                              k8s.gcr.io/pause:3.2                         2547   SANDBOX_READY
172.20.0.2   k8s.io      └─ kube-system/kube-controller-manager-talos-default-master-1:kube-controller-manager   k8s.gcr.io/kube-controller-manager:v1.20.4   2580   CONTAINER_RUNNING
172.20.0.2   k8s.io      kube-system/kube-scheduler-talos-default-master-1                                       k8s.gcr.io/pause:3.2                         2638   SANDBOX_READY
172.20.0.2   k8s.io      └─ kube-system/kube-scheduler-talos-default-master-1:kube-scheduler                     k8s.gcr.io/kube-scheduler:v1.20.4            2670   CONTAINER_RUNNING
...
```

If some of the containers are not running, it could be that image is still being pulled.
Otherwise process might crashing, in that case logs can be checked with `talosctl logs --kubernetes <containerID>`:

```bash
$ talosctl -n <IP> logs -k kube-system/kube-controller-manager-talos-default-master-1:kube-controller-manager
172.20.0.3: 2021-03-09T13:59:34.291667526Z stderr F 2021/03/09 13:59:34 Running command:
172.20.0.3: 2021-03-09T13:59:34.291702262Z stderr F Command env: (log-file=, also-stdout=false, redirect-stderr=true)
172.20.0.3: 2021-03-09T13:59:34.291707121Z stderr F Run from directory:
172.20.0.3: 2021-03-09T13:59:34.291710908Z stderr F Executable path: /usr/local/bin/kube-controller-manager
172.20.0.3: 2021-03-09T13:59:34.291719163Z stderr F Args (comma-delimited): /usr/local/bin/kube-controller-manager,--allocate-node-cidrs=true,--cloud-provider=,--cluster-cidr=10.244.0.0/16,--service-cluster-ip-range=10.96.0.0/12,--cluster-signing-cert-file=/system/secrets/kubernetes/kube-controller-manager/ca.crt,--cluster-signing-key-file=/system/secrets/kubernetes/kube-controller-manager/ca.key,--configure-cloud-routes=false,--kubeconfig=/system/secrets/kubernetes/kube-controller-manager/kubeconfig,--leader-elect=true,--root-ca-file=/system/secrets/kubernetes/kube-controller-manager/ca.crt,--service-account-private-key-file=/system/secrets/kubernetes/kube-controller-manager/service-account.key,--profiling=false
172.20.0.3: 2021-03-09T13:59:34.293870359Z stderr F 2021/03/09 13:59:34 Now listening for interrupts
172.20.0.3: 2021-03-09T13:59:34.761113762Z stdout F I0309 13:59:34.760982      10 serving.go:331] Generated self-signed cert in-memory
...
```

### Checking controller runtime logs

Talos runs a set of controllers which work on resources to build and support Kubernetes control plane.

Some debugging information can be queried from the controller logs with `talosctl logs controller-runtime`:

```bash
$ talosctl -n <IP> logs controller-runtime
172.20.0.2: 2021/03/09 13:57:11  secrets.EtcdController: controller starting
172.20.0.2: 2021/03/09 13:57:11  config.MachineTypeController: controller starting
172.20.0.2: 2021/03/09 13:57:11  k8s.ManifestApplyController: controller starting
172.20.0.2: 2021/03/09 13:57:11  v1alpha1.BootstrapStatusController: controller starting
172.20.0.2: 2021/03/09 13:57:11  v1alpha1.TimeStatusController: controller starting
...
```

Controllers run reconcile loop, so they might be starting, failing and restarting, that is expected behavior.
Things to look for:

`v1alpha1.BootstrapStatusController: bootkube initialized status not found`: control plane is not self-hosted, running with static pods.

`k8s.KubeletStaticPodController: writing static pod "/etc/kubernetes/manifests/talos-kube-apiserver.yaml"`: static pod definitions were rendered successfully.

`k8s.ManifestApplyController: controller failed: error creating mapping for object /v1/Secret/bootstrap-token-q9pyzr: an error on the server ("") has prevented the request from succeeding`: control plane endpoint is not up yet, bootstrap manifests can't be injected, controller is going to retry.

`k8s.KubeletStaticPodController: controller failed: error refreshing pod status: error fetching pod status: an error on the server ("Authorization error (user=apiserver-kubelet-client, verb=get, resource=nodes, subresource=proxy)") has prevented the request from succeeding`: kubelet hasn't been able to contact `kube-apiserver` yet to push pod status, controller
is going to retry.

`k8s.ManifestApplyController: created rbac.authorization.k8s.io/v1/ClusterRole/psp:privileged`: one of the bootstrap manifests got successfully applied.

`secrets.KubernetesController: controller failed: missing cluster.aggregatorCA secret`: Talos is running with 0.8 configuration, if the cluster was upgraded from 0.8, this is expected, and conversion process will fix machine config
automatically.
If this cluster was bootstrapped with version 0.9, machine configuration should be regenerated with 0.9 talosctl.

If there are no new messages in `controller-runtime` log, it means that controllers finished reconciling successfully.

### Checking static pod definitions

Talos generates static pod definitions for `kube-apiserver`, `kube-controller-manager`, and `kube-scheduler`
components based on machine configuration.
These definitions can be checked as resources with `talosctl get staticpods`:

```bash
$ talosctl -n <IP> get staticpods -o yaml
get staticpods -o yaml
node: 172.20.0.2
metadata:
    namespace: controlplane
    type: StaticPods.kubernetes.talos.dev
    id: kube-apiserver
    version: 2
    phase: running
    finalizers:
        - k8s.StaticPodStatus("kube-apiserver")
spec:
    apiVersion: v1
    kind: Pod
    metadata:
        annotations:
            talos.dev/config-version: "1"
            talos.dev/secrets-version: "1"
        creationTimestamp: null
        labels:
            k8s-app: kube-apiserver
            tier: control-plane
        name: kube-apiserver
        namespace: kube-system
...
```

Status of the static pods can queried with `talosctl get staticpodstatus`:

```bash
$ talosctl -n <IP> get staticpodstatus
NODE         NAMESPACE      TYPE              ID                                                           VERSION   READY
172.20.0.2   controlplane   StaticPodStatus   kube-system/kube-apiserver-talos-default-master-1            1         True
172.20.0.2   controlplane   StaticPodStatus   kube-system/kube-controller-manager-talos-default-master-1   1         True
172.20.0.2   controlplane   StaticPodStatus   kube-system/kube-scheduler-talos-default-master-1            1         True
```

Most important status is `Ready` printed as last column, complete status can be fetched by adding `-o yaml` flag.

### Checking bootstrap manifests

As part of bootstrap process, Talos injects bootstrap manifests into Kubernetes API server.
There are two kinds of manifests: system manifests built-in into Talos and extra manifests downloaded (custom CNI, extra manifests in the machine config):

```bash
$ talosctl -n <IP> get manifests
NODE         NAMESPACE      TYPE       ID                               VERSION
172.20.0.2   controlplane   Manifest   00-kubelet-bootstrapping-token   1
172.20.0.2   controlplane   Manifest   01-csr-approver-role-binding     1
172.20.0.2   controlplane   Manifest   01-csr-node-bootstrap            1
172.20.0.2   controlplane   Manifest   01-csr-renewal-role-binding      1
172.20.0.2   controlplane   Manifest   02-kube-system-sa-role-binding   1
172.20.0.2   controlplane   Manifest   03-default-pod-security-policy   1
172.20.0.2   controlplane   Manifest   05-https://docs.projectcalico.org/manifests/calico.yaml   1
172.20.0.2   controlplane   Manifest   10-kube-proxy                    1
172.20.0.2   controlplane   Manifest   11-core-dns                      1
172.20.0.2   controlplane   Manifest   11-core-dns-svc                  1
172.20.0.2   controlplane   Manifest   11-kube-config-in-cluster        1
```

Details of each manifests can be queried by adding `-o yaml`:

```bash
$ talosctl -n <IP> get manifests 01-csr-approver-role-binding --namespace=controlplane -o yaml
node: 172.20.0.2
metadata:
    namespace: controlplane
    type: Manifests.kubernetes.talos.dev
    id: 01-csr-approver-role-binding
    version: 1
    phase: running
spec:
    - apiVersion: rbac.authorization.k8s.io/v1
      kind: ClusterRoleBinding
      metadata:
        name: system-bootstrap-approve-node-client-csr
      roleRef:
        apiGroup: rbac.authorization.k8s.io
        kind: ClusterRole
        name: system:certificates.k8s.io:certificatesigningrequests:nodeclient
      subjects:
        - apiGroup: rbac.authorization.k8s.io
          kind: Group
          name: system:bootstrappers
```

### Worker node is stuck with `apid` health check failures

Control plane nodes have enough secret material to generate `apid` server certificates, but worker nodes
depend on control plane `trustd` services to generate certificates.
Worker nodes wait for `kubelet` to join the cluster, then `apid` queries Kubernetes endpoints via control plane
endpoint to find `trustd` endpoints, and use `trustd` to issue the certficiate.

So if `apid` health checks is failing on worker node:

* make sure control plane endpoint is healthy
* check that worker node `kubelet` joined the cluster
