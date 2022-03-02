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

A control plane node is a node which:

- runs etcd, the Kubernetes database
- runs the Kubernetes control plane
  - kube-apiserver
  - kube-controller-manager
  - kube-scheduler
- serves as an administrative proxy to the worker nodes

These nodes are critical to the operation of your cluster.
Without control plane nodes, Kubernetes will not respond to changes in the
system, and certain central services may not be available.

Talos nodes which have `.machine.type` of `controlplane` are control plane nodes.

Control plane nodes are tainted by default to prevent workloads from being scheduled to control plane nodes.

### How many control plane nodes should be deployed?

Because control plane nodes are so important, it is important that they be
deployed with redundancy to ensure consistent, reliable operation of the cluster
during upgrades, reboots, hardware failures, and other such events.
This is also known as high-availability or just HA.
Non-HA clusters are sometimes used as test clusters, CI clusters, or in specific scenarios
which warrant the loss of redundancy, but they should almost never be used in production.

Maintaining the proper count of control plane nodes is also critical.
The etcd database operates on the principles of membership and quorum, so
membership should always be an odd number, and there is exponentially-increasing
overhead for each additional member.
Therefore, the number of control plane nodes should almost always be 3.
In some particularly large or distributed clusters, the count may be 5, but this
is very rare.

See [this document](../../learn-more/concepts/#control-planes-are-not-linear-replicas) on the topic for more information.

### What is the control plane endpoint?

The Kubernetes control plane endpoint is the single canonical URL by which the
Kubernetes API is accessed.
Especially with high-availability (HA) control planes, it is common that this endpoint may not point to the Kubernetes API server
directly, but may be instead point to a load balancer or a DNS name which may
have multiple `A` and `AAAA` records.

Like Talos' own API, the Kubernetes API is constructed with mutual TLS, client
certs, and a common Certificate Authority (CA).
Unlike general-purpose websites, there is no need for an upstream CA, so tools
such as cert-manager, services such as Let's Encrypt, or purchased products such
as validated TLS certificates are not required.
Encryption, however, _is_, and hence the URL scheme will always be `https://`.

By default, the Kubernetes API server in Talos runs on port 6443.
As such, the control plane endpoint URLs for Talos will almost always be of the form
`https://endpoint:6443`, noting that the port, since it is not the `https`
default of `443` is _required_.
The `endpoint` above may be a DNS name or IP address, but it should be
ultimately be directed to the _set_ of all controlplane nodes, as opposed to a
single one.

As mentioned above, this can be achieved by a number of strategies, including:

- an external load balancer
- DNS records
- Talos-builtin shared IP ([VIP](../vip/)
- BGP peering of a shared IP (such as with [kube-vip](https://kube-vip.io))

Using a DNS name here is usually a good idea, it being the most flexible
option, since it allows the combination with any _other_ option, while offering
a layer of abstraction.
It allows the underlying IP addresses to change over time without impacting the
canonical URL.

Unlike most services in Kubernetes, the API server runs with host networking,
meaning that it shares the network namespace with the host.
This means you can use the IP address(es) of the host to refer to the Kubernetes
API server.

For availability of the API, it is important that any load balancer be aware of
the health of the backend API servers.
This makes a load balancer-based system valuable to minimize disruptions during
common node lifecycle operations like reboots and upgrades.

It is critical that control plane endpoint works correctly during cluster bootstrap phase, as nodes discover
each other using control plane endpoint.

### kubelet is not running on control plane node

The `kubelet` service should be running on control plane nodes as soon as networking is configured:

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

If the `kubelet` is not running, it may be due to invalid configuration.
Check `kubelet` logs with the `talosctl logs` command:

```bash
$ talosctl -n <IP> logs kubelet
172.20.0.2: I0305 20:45:07.756948    2334 controller.go:101] kubelet config controller: starting controller
172.20.0.2: I0305 20:45:07.756995    2334 controller.go:267] kubelet config controller: ensuring filesystem is set up correctly
172.20.0.2: I0305 20:45:07.757000    2334 fsstore.go:59] kubelet config controller: initializing config checkpoints directory "/etc/kubernetes/kubelet/store"
```

### etcd is not running

By far the most likely cause of `etcd` not running is because the cluster has
not yet been bootstrapped or because bootstrapping is currently in progress.
The `talosctl bootstrap` command must be run manually and only _once_ per
cluster, and this step is commonly missed.
Once a node is bootstrapped, it will start `etcd` and, over the course of a
minute or two (depending on the download speed of the control plane nodes), the
other control plane nodes should discover it and join themselves to the cluster.

Also, `etcd` will only run on control plane nodes.
If a node is designated as a worker node, you should not expect `etcd` to be
running on it.

When node boots for the first time, the `etcd` data directory (`/var/lib/etcd`) is empty, and it will only be populated when `etcd` is launched.

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
Talos pulls the `etcd` image from the container registry.

If the `etcd` service is crashing and restarting, check its logs with `talosctl -n <IP> logs etcd`.
The most common reasons for crashes are:

- wrong arguments passed via `extraArgs` in the configuration;
- booting Talos on non-empty disk with previous Talos installation, `/var/lib/etcd` contains data from old cluster.

### etcd is not running on non-bootstrap control plane node

The `etcd` service on control plane nodes which were not the target of the cluster bootstrap will wait until the bootstrapped control plane node has completed.
The bootstrap and discovery processes may take a few minutes to complete.
As soon as the bootstrapped node starts its Kubernetes control plane components, `kubectl get endpoints` will return the IP of bootstrapped control plane node.
At this point, the other control plane nodes will start their `etcd` services, join the cluster, and then start their own Kubernetes control plane components.

### Kubernetes static pod definitions are not generated

Talos should write the static pod definitions for the Kubernetes control plane
in `/etc/kubernetes/manifests`:

```bash
$ talosctl -n <IP> ls /etc/kubernetes/manifests
NODE         NAME
172.20.0.2   .
172.20.0.2   talos-kube-apiserver.yaml
172.20.0.2   talos-kube-controller-manager.yaml
172.20.0.2   talos-kube-scheduler.yaml
```

If the static pod definitions are not rendered, check `etcd` and `kubelet` service health (see above)
and the controller runtime logs (`talosctl logs controller-runtime`).

### Talos prints error `an error on the server ("") has prevented the request from succeeding`

This is expected during initial cluster bootstrap and sometimes after a reboot:

```bash
[   70.093289] [talos] task labelNodeAsMaster (1/1): starting
[   80.094038] [talos] retrying error: an error on the server ("") has prevented the request from succeeding (get nodes talos-default-master-1)
```

Initially `kube-apiserver` component is not running yet, and it takes some time before it becomes fully up
during bootstrap (image should be pulled from the Internet, etc.)
Once the control plane endpoint is up, Talos should continue with its boot
process.

If Talos doesn't proceed, it may be due to a configuration issue.

In any case, the status of the control plane components on each control plane nodes can be checked with `talosctl containers -k`:

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

This error means that `kube-apiserver` is up and the control plane endpoint is healthy, but the `kubelet` hasn't received
its client certificate yet, and it wasn't able to register itself to Kubernetes.
The Kubernetes controller manager (`kube-controller-manager`)is responsible for monitoring the certificate
signing requests (CSRs) and issuing certificates for each of them.
The kubelet is responsible for generating and submitting the CSRs for its
associated node.

For the `kubelet` to get its client certificate, then, the Kubernetes control plane
must be healthy:

- the API server is running and available at the Kubernetes control plane
  endpoint URL
- the controller manager is running and a leader has been elected

The states of any CSRs can be checked with `kubectl get csr`:

```bash
$ kubectl get csr
NAME        AGE   SIGNERNAME                                    REQUESTOR                 CONDITION
csr-jcn9j   14m   kubernetes.io/kube-apiserver-client-kubelet   system:bootstrap:q9pyzr   Approved,Issued
csr-p6b9q   14m   kubernetes.io/kube-apiserver-client-kubelet   system:bootstrap:q9pyzr   Approved,Issued
csr-sw6rm   14m   kubernetes.io/kube-apiserver-client-kubelet   system:bootstrap:q9pyzr   Approved,Issued
csr-vlghg   14m   kubernetes.io/kube-apiserver-client-kubelet   system:bootstrap:q9pyzr   Approved,Issued
```

### Talos prints error `node not ready`

A Node in Kubernetes is marked as `Ready` only once its CNI is up.
It takes a minute or two for the CNI images to be pulled and for the CNI to start.
If the node is stuck in this state for too long, check CNI pods and logs with `kubectl`.
Usually, CNI-related resources are created in `kube-system` namespace.

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

The full error might look like:

```bash
x509: certificate signed by unknown authority (possiby because of crypto/rsa: verification error" while trying to verify candidate authority certificate "kubernetes"
```

Usually, this occurs because the control plane endpoint points to a different
cluster than the client certificate was generated for.
If a node was recycled between clusters, make sure it was properly wiped between
uses.
If a client has multiple client configurations, make sure you are matching the correct `talosconfig` with the
correct cluster.

### etcd is running on bootstrap node, but stuck in `pre` state on non-bootstrap nodes

Please see question `etcd is not running on non-bootstrap control plane node`.

### Checking `kube-controller-manager` and `kube-scheduler`

If the control plane endpoint is up, the status of the pods can be ascertained with `kubectl`:

```bash
$ kubectl get pods -n kube-system -l k8s-app=kube-controller-manager
NAME                                             READY   STATUS    RESTARTS   AGE
kube-controller-manager-talos-default-master-1   1/1     Running   0          28m
kube-controller-manager-talos-default-master-2   1/1     Running   0          28m
kube-controller-manager-talos-default-master-3   1/1     Running   0          28m
```

If the control plane endpoint is not yet up, the container status of the control plane components can be queried with
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
Otherwise the process might crashing.
The logs can be checked with `talosctl logs --kubernetes <containerID>`:

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

Talos runs a set of controllers which operate on resources to build and support the Kubernetes control plane.

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

Controllers continuously run a reconcile loop, so at any time, they may be starting, failing, or restarting.
This is expected behavior.

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

If there are no new messages in the `controller-runtime` log, it means that the controllers have successfully finished reconciling, and that the current system state is the desired system state.

### Checking static pod definitions

Talos generates static pod definitions for the `kube-apiserver`, `kube-controller-manager`, and `kube-scheduler`
components based on its machine configuration.
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

The status of the static pods can queried with `talosctl get staticpodstatus`:

```bash
$ talosctl -n <IP> get staticpodstatus
NODE         NAMESPACE      TYPE              ID                                                           VERSION   READY
172.20.0.2   controlplane   StaticPodStatus   kube-system/kube-apiserver-talos-default-master-1            1         True
172.20.0.2   controlplane   StaticPodStatus   kube-system/kube-controller-manager-talos-default-master-1   1         True
172.20.0.2   controlplane   StaticPodStatus   kube-system/kube-scheduler-talos-default-master-1            1         True
```

The most important status field is `READY`, which is the last column printed.
The complete status can be fetched by adding `-o yaml` flag.

### Checking bootstrap manifests

As part of the bootstrap process, Talos injects bootstrap manifests into Kubernetes API server.
There are two kinds of these manifests: system manifests built-in into Talos and extra manifests downloaded (custom CNI, extra manifests in the machine config):

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

Details of each manifest can be queried by adding `-o yaml`:

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
Worker nodes wait for their `kubelet` to join the cluster.
Then the Talos `apid` queries the Kubernetes endpoints via control plane
endpoint to find `trustd` endpoints.
They then use `trustd` to request and receive their certificate.

So if `apid` health checks are failing on worker node:

- make sure control plane endpoint is healthy
- check that worker node `kubelet` joined the cluster
