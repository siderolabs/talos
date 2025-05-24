---
title: "Troubleshooting"
description: "Troubleshoot control plane and other failures for Talos Linux clusters."
aliases:
  - ../guides/troubleshooting-control-plane
  - ../advanced/troubleshooting-control-plane
---

<!-- markdownlint-disable MD026 -->

In this guide we assume that Talos is configured with default features enabled, such as [Discovery Service]({{< relref "../talos-guides/discovery" >}}) and [KubePrism]({{< relref "../kubernetes-guides/configuration/kubeprism" >}}).
If these features are disabled, some of the troubleshooting steps may not apply or may need to be adjusted.

This guide is structured so that it can be followed step-by-step, skip sections which are not relevant to your issue.

## Network Configuration

As Talos Linux is an API-based operating system, it is important to have networking configured so that the API can be accessed.
Some information can be gathered from the [Interactive Dashboard]({{< relref "../talos-guides/interactive-dashboard" >}}) which is available on the machine console.

When running in the cloud the networking should be configured automatically.
Whereas when running on bare-metal it may need more specific configuration, see [networking `metal` configuration guide]({{< relref "../talos-guides/install/bare-metal-platforms/network-config" >}}).

## Talos API

The Talos API runs on [port 50000]({{< relref "../learn-more/talos-network-connectivity" >}}).
Control plane nodes should always serve the Talos API, while worker nodes require access to the control plane nodes to issue TLS certificates for the workers.

### Firewall Issues

Make sure that the firewall is not blocking port 50000, and [communication]({{< relref "../learn-more/talos-network-connectivity" >}}) on ports 50000/50001 inside the cluster.

### Client Configuration Issues

Make sure to use correct `talosconfig` client configuration file matching your cluster.
See [getting started]({{< relref "./getting-started" >}}) for more information.

The most common issue is that `talosctl gen config` writes `talosconfig` to the file in the current directory, while `talosctl` by default picks up the configuration from the default location (`~/.talos/config`).
The path to the configuration file can be specified with `--talosconfig` flag to `talosctl`.

### Conflict on Kubernetes and Host Subnets

If `talosctl` returns an error saying that certificate IPs are empty, it might be due to a conflict between Kubernetes and host subnets.
The Talos API runs on the host network, but it automatically excludes Kubernetes pod & network subnets from the useable set of addresses.

Talos default machine configuration specifies the following Kubernetes pod and service IPv4 CIDRs: `10.244.0.0/16` and `10.96.0.0/12`.
If the host network is configured with one of these subnets, change the machine configuration to use a different subnet.

### Wrong Endpoints

The `talosctl` CLI connects to the Talos API via the specified endpoints, which should be a list of control plane machine addresses.
The client will automatically retry on other endpoints if there are unavailable endpoints.

Worker nodes should not be used as the endpoint, as they are not able to forward request to other nodes.

The [VIP]({{< relref "../talos-guides/network/vip" >}}) should never be used as Talos API endpoint.

### TCP Loadbalancer

When using a TCP loadbalancer, make sure the loadbalancer endpoint is included in the `.machine.certSANs` list in the machine configuration.

## System Requirements

If minimum [system requirements]({{< relref "./system-requirements" >}}) are not met, this might manifest itself in various ways, such as random failures when starting services, or failures to pull images from the container registry.

## Running Health Checks

Talos Linux provides a set of basic health checks with `talosctl health` command which can be used to check the health of the cluster.

In the default mode, `talosctl health` uses information from the [discovery]({{< relref "../talos-guides/discovery" >}}) to get the information about cluster members.
This can be overridden with command line flags `--control-plane-nodes` and `--worker-nodes`.

## Gathering Logs

While the logs and state of the system can be queried via the Talos API, it is often useful to gather the logs from all nodes in the cluster, and analyze them offline.
The `talosctl support` command can be used to gather logs and other information from the nodes specified with `--nodes` flag (multiple nodes are supported).

## Discovery and Cluster Membership

Talos Linux uses [Discovery Service]({{< relref "../talos-guides/discovery" >}}) to discover other nodes in the cluster.

The list of members on each machine should be consistent: `talosctl -n <IP> get members`.

### Some Members are Missing

Ensure connectivity to the discovery service (default is `discovery.talos.dev:443`), and that the discovery registry is not disabled.

### Duplicate Members

Don't use same base secrets to generate machine configuration for multiple clusters, as some secrets are used to identify members of the same cluster.
So if the same machine configuration (or secrets) are used to repeatedly create and destroy clusters, the discovery service will see the same nodes as members of different clusters.

### Removed Members are Still Present

Talos Linux removes itself from the discovery service when it is [reset]({{< relref "../talos-guides/resetting-a-machine" >}}).
If the machine was not reset, it might show up as a member of the cluster for the maximum TTL of the discovery service (30 minutes), and after that it will be automatically removed.

## `etcd` Issues

`etcd` is the distributed key-value store used by Kubernetes to store its state.
Talos Linux provides automation to manage `etcd` members running on control plane nodes.
If `etcd` is not healthy, the Kubernetes API server will not be able to function correctly.

It is always recommended to run an odd number of `etcd` members, as with 3 or more members it provides fault tolerance for less than quorum member failures.

Common troubleshooting steps:

- check `etcd` service state with `talosctl -n IP service etcd` for each control plane node
- check `etcd` membership on each control plane node with `talosctl -n IP etcd members`
- check `etcd` logs with `talosctl -n IP logs etcd`
- check `etcd` alarms with `talosctl -n IP etcd alarm list`

### All `etcd` Services are Stuck in `Pre` State

Make sure that a single member was [bootstrapped]({{< relref "./getting-started#kubernetes-bootstrap" >}}).

Check that the machine is able to pull the `etcd` container image, check `talosctl dmesg` for messages starting with `retrying:` prefix.

### Some `etcd` Services are Stuck in `Pre` State

Make sure traffic is not blocked on port 2380 between controlplane nodes.

Check that `etcd` quorum is not lost.

Check that all control plane nodes are reported in `talosctl get members` output.

### `etcd` Reports and Alarm

See [etcd maintenance]({{< relref "../advanced/etcd-maintenance" >}}) guide.

### `etcd` Quorum is Lost

See [disaster recovery]({{< relref "../advanced/disaster-recovery" >}}) guide.

### Other Issues

`etcd` will only run on control plane nodes.
If a node is designated as a worker node, you should not expect `etcd` to be running on it.

When a node boots for the first time, the `etcd` data directory (`/var/lib/etcd`) is empty, and it will only be populated when `etcd` is launched.

If the `etcd` service is crashing and restarting, check its logs with `talosctl -n <IP> logs etcd`.
The most common reasons for crashes are:

- wrong arguments passed via `extraArgs` in the configuration;
- booting Talos on non-empty disk with an existing Talos installation, `/var/lib/etcd` contains data from the old cluster.

## `kubelet` and Kubernetes Node Issues

The `kubelet` service should be running on all Talos nodes, and it is responsible for running Kubernetes pods,
static pods (including control plane components), and registering the node with the Kubernetes API server.

If the `kubelet` doesn't run on a control plane node, it will block the control plane components from starting.

The node will not be registered in Kubernetes until the Kubernetes API server is up and initial Kubernetes manifests are applied.

### `kubelet` is not running

Check that `kubelet` image is available (`talosctl image ls --namespace system`).

Check `kubelet` logs with `talosctl -n IP logs kubelet` for startup errors:

- make sure Kubernetes version is [supported]({{< relref "./support-matrix" >}}) with this Talos release
- make sure `kubelet` extra arguments and extra configuration supplied with Talos machine configuration is valid

### Talos Complains about Node Not Found

`kubelet` hasn't yet registered the node with the Kubernetes API server, this is expected during initial cluster bootstrap, the error will go away.
If the message persists, check Kubernetes API health.

The Kubernetes controller manager (`kube-controller-manager`) is responsible for monitoring the certificate
signing requests (CSRs) and issuing certificates for each of them.
The `kubelet` is responsible for generating and submitting the CSRs for its
associated node.

The state of any CSRs can be checked with `kubectl get csr`:

```bash
$ kubectl get csr
NAME        AGE   SIGNERNAME                                    REQUESTOR                 CONDITION
csr-jcn9j   14m   kubernetes.io/kube-apiserver-client-kubelet   system:bootstrap:q9pyzr   Approved,Issued
csr-p6b9q   14m   kubernetes.io/kube-apiserver-client-kubelet   system:bootstrap:q9pyzr   Approved,Issued
csr-sw6rm   14m   kubernetes.io/kube-apiserver-client-kubelet   system:bootstrap:q9pyzr   Approved,Issued
csr-vlghg   14m   kubernetes.io/kube-apiserver-client-kubelet   system:bootstrap:q9pyzr   Approved,Issued
```

### `kubectl get nodes` Reports Wrong Internal IP

Configure the correct internal IP address with [`.machine.kubelet.nodeIP`]({{< relref "../reference/configuration/v1alpha1/config#Config.machine.kubelet.nodeIP" >}})

### `kubectl get nodes` Reports Wrong External IP

Talos Linux doesn't manage the external IP, it is managed with the Kubernetes Cloud Controller Manager.

### `kubectl get nodes` Reports Wrong Node Name

By default, the Kubernetes node name is derived from the hostname.
Update the hostname using the machine configuration, cloud configuration, or via DHCP server.

### Node Is Not Ready

A Node in Kubernetes is marked as `Ready` only once its CNI is up.
It takes a minute or two for the CNI images to be pulled and for the CNI to start.
If the node is stuck in this state for too long, check CNI pods and logs with `kubectl`.
Usually, CNI-related resources are created in `kube-system` namespace.

For example, for the default Talos Flannel CNI:

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

### Duplicate/Stale Nodes

Talos Linux doesn't remove Kubernetes nodes automatically, so if a node is removed from the cluster, it will still be present in Kubernetes.
Remove the node from Kubernetes with `kubectl delete node <node-name>`.

### Talos Complains about Certificate Errors on `kubelet` API

This error might appear during initial cluster bootstrap, and it will go away once the Kubernetes API server is up and the node is registered.

The example of Talos logs:

```bash
[talos] controller failed {"component": "controller-runtime", "controller": "k8s.KubeletStaticPodController", "error": "error refreshing pod status: error fetching pod status: Get \"https://127.0.0.1:10250/pods/?timeout=30s\": remote error: tls: internal error"}
```

By default configuration, `kubelet` issues a self-signed server certificate, but when `rotate-server-certificates` feature is enabled,
`kubelet` issues its certificate using `kube-apiserver`.
Make sure the `kubelet` CSR is approved by the Kubernetes API server.

In either case, this error is not critical, as it only affects reporting of the pod status to Talos Linux.

## Kubernetes Control Plane

The Kubernetes control plane consists of the following components:

- `kube-apiserver` - the Kubernetes API server
- `kube-controller-manager` - the Kubernetes controller manager
- `kube-scheduler` - the Kubernetes scheduler

Optionally, `kube-proxy` runs as a DaemonSet to provide pod-to-service communication.

`coredns` provides name resolution for the cluster.

CNI is not part of the control plane, but it is required for Kubernetes pods using pod networking.

Troubleshooting should always start with `kube-apiserver`, and then proceed to other components.

Talos Linux configures `kube-apiserver` to talk to the `etcd` running on the same node, so `etcd` must be healthy before `kube-apiserver` can start.
The `kube-controller-manager` and `kube-scheduler` are configured to talk to the `kube-apiserver` on the same node, so they will not start until `kube-apiserver` is healthy.

### Control Plane Static Pods

Talos should generate the static pod definitions for the Kubernetes control plane
as resources:

```bash
$ talosctl -n <IP> get staticpods
NODE         NAMESPACE   TYPE        ID                        VERSION
172.20.0.2   k8s         StaticPod   kube-apiserver            1
172.20.0.2   k8s         StaticPod   kube-controller-manager   1
172.20.0.2   k8s         StaticPod   kube-scheduler            1
```

Talos should report that the static pod definitions are rendered for the `kubelet`:

```bash
$ talosctl -n <IP> dmesg | grep 'rendered new'
172.20.0.2: user: warning: [2023-04-26T19:17:52.550527204Z]: [talos] rendered new static pod {"component": "controller-runtime", "controller": "k8s.StaticPodServerController", "id": "kube-apiserver"}
172.20.0.2: user: warning: [2023-04-26T19:17:52.552186204Z]: [talos] rendered new static pod {"component": "controller-runtime", "controller": "k8s.StaticPodServerController", "id": "kube-controller-manager"}
172.20.0.2: user: warning: [2023-04-26T19:17:52.554607204Z]: [talos] rendered new static pod {"component": "controller-runtime", "controller": "k8s.StaticPodServerController", "id": "kube-scheduler"}
```

If the static pod definitions are not rendered, check `etcd` and `kubelet` service health (see above)
and the controller runtime logs (`talosctl logs controller-runtime`).

### Control Plane Pod Status

Initially the `kube-apiserver` component will not be running, and it takes some time before it becomes fully up
during bootstrap (image should be pulled from the Internet, etc.)

The status of the control plane components on each of the control plane nodes can be checked with `talosctl containers -k`:

```bash
$ talosctl -n <IP> containers --kubernetes
NODE         NAMESPACE   ID                                                                                            IMAGE                                               PID    STATUS
172.20.0.2   k8s.io      kube-system/kube-apiserver-talos-default-controlplane-1                                       registry.k8s.io/pause:3.2                                2539   SANDBOX_READY
172.20.0.2   k8s.io      └─ kube-system/kube-apiserver-talos-default-controlplane-1:kube-apiserver:51c3aad7a271        registry.k8s.io/kube-apiserver:v{{< k8s_release >}} 2572   CONTAINER_RUNNING
```

The logs of the control plane components can be checked with `talosctl logs --kubernetes` (or with `-k` as a shorthand):

```bash
talosctl -n <IP> logs -k kube-system/kube-apiserver-talos-default-controlplane-1:kube-apiserver:51c3aad7a271
```

If the control plane component reports error on startup, check that:

- make sure Kubernetes version is [supported]({{< relref "./support-matrix" >}}) with this Talos release
- make sure extra arguments and extra configuration supplied with Talos machine configuration is valid

### Kubernetes Bootstrap Manifests

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

### Other Control Plane Components

Once the Kubernetes API server is up, other control plane components issues can be troubleshooted with `kubectl`:

```shell
kubectl get nodes -o wide
kubectl get pods -o wide --all-namespaces
kubectl describe pod -n NAMESPACE POD
kubectl logs -n NAMESPACE POD
```

## Kubernetes API

The Kubernetes API client configuration (`kubeconfig`) can be retrieved using Talos API with `talosctl -n <IP> kubeconfig` command.
Talos Linux mostly doesn't depend on the Kubernetes API endpoint for the cluster, but Kubernetes API endpoint should be configured
correctly for external access to the cluster.

### Kubernetes Control Plane Endpoint

The Kubernetes control plane endpoint is the single canonical URL by which the
Kubernetes API is accessed.
Especially with high-availability (HA) control planes, this endpoint may point to a load balancer or a DNS name which may
have multiple `A` and `AAAA` records.

Like Talos' own API, the Kubernetes API uses mutual TLS, client
certs, and a common Certificate Authority (CA).
Unlike general-purpose websites, there is no need for an upstream CA, so tools
such as cert-manager, Let's Encrypt, or products such
as validated TLS certificates are not required.
Encryption, however, _is_, and hence the URL scheme will always be `https://`.

By default, the Kubernetes API server in Talos runs on port 6443.
As such, the control plane endpoint URLs for Talos will almost always be of the form
`https://endpoint:6443`.
(The port, since it is not the `https` default of `443` is required.)
The `endpoint` above may be a DNS name or IP address, but it should be
directed to the _set_ of all controlplane nodes, as opposed to a
single one.

As mentioned above, this can be achieved by a number of strategies, including:

- an external load balancer
- DNS records
- Talos-builtin shared IP ([VIP]({{< relref "../talos-guides/network/vip" >}}))
- BGP peering of a shared IP (such as with [kube-vip](https://kube-vip.io))

Using a DNS name here is a good idea, since it allows any other option, while offering
a layer of abstraction.
It allows the underlying IP addresses to change without impacting the
canonical URL.

Unlike most services in Kubernetes, the API server runs with host networking,
meaning that it shares the network namespace with the host.
This means you can use the IP address(es) of the host to refer to the Kubernetes
API server.

For availability of the API, it is important that any load balancer be aware of
the health of the backend API servers, to minimize disruptions during
common node operations like reboots and upgrades.

## Miscellaneous

### Checking Controller Runtime Logs

Talos runs a set of [controllers]({{< relref "../learn-more/controllers-resources" >}}) which operate on resources to build and support machine operations.

Some debugging information can be queried from the controller logs with `talosctl logs controller-runtime`:

```bash
talosctl -n <IP> logs controller-runtime
```

Controllers continuously run a reconcile loop, so at any time, they may be starting, failing, or restarting.
This is expected behavior.

If there are no new messages in the `controller-runtime` log, it means that the controllers have successfully finished reconciling, and that the current system state is the desired system state.
