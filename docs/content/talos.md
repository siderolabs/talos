---
title: "Talos"
menu:
  main:
    name: Documentation
    weight: 10
  docs:
    weight: 1
---

<!-- markdownlint-disable no-inline-html -->
<a href="https://www.cncf.io/certification/software-conformance/">
  <img class="object-contain h-32 float-right"
       src="/images/certified-kubernetes-color.svg"
       alt="Certified Kubernetes" />
</a>
<!-- markdownlint-enable no-inline-html -->

**Talos** is a modern OS designed to be secure, immutable, and minimal.
Its purpose is to host Kubernetes clusters, so it is tightly integrated with Kubernetes.
Talos is based on the Linux kernel, and supports most cloud platforms, bare metal, and most virtualization platforms.
All system management is done via an API, and there is no shell or interactive console.

See the [FAQ]({{< ref "/docs/faqs" >}}) for more details and an architecture diagram.

Some of the capabilities and benefits provided by Talos include:

- **Security**: Talos reduces your attack surface by practicing the Principle of Least Privilege (PoLP) and by securing the API with mutual TLS (mTLS) authentication.
- **Predictability**: Talos eliminates unneeded variables and reduces unknown factors in your environment by employing immutable infrastructure ideology.
- **Evolvability**: Talos simplifies your architecture and increases your ability to easily accommodate future changes.

For details on the various components that make up Talos, please see the [components]({{< ref "/docs/components" >}}) section.

To get started with Talos, see the [Getting Started Guide]({{< ref "/docs/guides/getting_started" >}}).

If you need help, or if you have questions or comments, we would love to hear from you! Please join our community on [Slack](https://slack.dev.talos-systems.io), [GitHub](https://github.com/talos-systems), or the mailing list.

## Features

### Technologies

- **[musl-libc][musl]:** uses musl as the C standard library
- **[golang][golang]:** implements a pure golang `init`
- **[gRPC][grpc]:** exposes a secure gRPC API
- **[containerd][containerd]:** runs containerd for `system` services in tandem with the builtin [`CRI`][cri] runtime for Kubernetes pods
- **[kubeadm][kubeadm]:** uses `kubeadm` to create conformant Kubernetes clusters

### Secure

Talos takes a defense in depth approach to security.
Below, we touch on a few of the measures taken to increase the security posture of Talos.

#### Minimal

Talos is a minimalistic distribution that consists of only a handful of binaries and shared libraries.
Just enough to run [`containerd`][containerd] and a small set of `system` services.
This aligns with NIST's recommendation in the [Application Container Security Guide][nist]:

> Whenever possible, organizations should use these minimalistic OSs to reduce their attack surfaces and mitigate the typical risks and hardening activities associated with general-purpose OSs.

Talos differentiates itself and improves on this since it is built for one purpose — to run Kubernetes.

#### Hardened

There are a number of ways that Talos provides added hardening:

- employs the recommended configuration and runtime settings outlined in the [Kernel Self Protection Project][kspp]
- enables mutual TLS for the API
- enforces the settings and configurations described in the [CIS][cis] guidelines

#### Immutable

Talos improves its security posture further by mounting the root filesystem as read-only and removing any host-level access by traditional means such as a shell and SSH.

### Current

Stay current with our commitment to an `n-1` adoption rate of upstream Kubernetes.
Additionally, the latest LTS Linux kernel will always be used.

## Usage

Each Talos node exposes an API designed with cluster administrators in mind.
It provides just enough to debug and remediate issues.
Using the provided CLI (`osctl`), you can:

- restart a node (`osctl reboot`)
- get CPU and memory usage of a container (`osctl stats`)
- view kernel buffer logs (`osctl dmesg`)
- restart a container (`osctl restart`)
- tail container logs (`osctl logs`)

and more.

### Examples

Query `system` services:

```bash
$ osctl ps
NAMESPACE   ID       IMAGE          PID   STATUS
system      ntpd     talos/ntpd     101   RUNNING
system      osd      talos/osd      107   RUNNING
system      proxyd   talos/proxyd   393   RUNNING
system      trustd   talos/trustd   115   RUNNING
```

or query the containers in the `k8s.io` [`namespace`](https://github.com/containerd/containerd/blob/master/docs/namespaces.md):

```bash
$ osctl ps -k
NAMESPACE   ID                                                                     IMAGE                          PID   STATUS
k8s.io      kube-system/kube-scheduler-master-1:kube-scheduler                     k8s.gcr.io/hyperkube:v1.14.1   783   RUNNING
k8s.io      kube-system/kube-scheduler-master-1                                    k8s.gcr.io/pause:3.1           564   RUNNING
k8s.io      kube-system/kube-controller-manager-master-1:kube-controller-manager   k8s.gcr.io/hyperkube:v1.14.1   744   RUNNING
k8s.io      kube-system/kube-controller-manager-master-1                           k8s.gcr.io/pause:3.1           594   RUNNING
k8s.io      kube-system/kube-apiserver-master-1                                    k8s.gcr.io/pause:3.1           593   RUNNING
k8s.io      kube-system/kube-apiserver-master-1:kube-apiserver                     k8s.gcr.io/hyperkube:v1.14.1   796   RUNNING
k8s.io      kube-system/etcd-master-1                                              k8s.gcr.io/pause:3.1           592   RUNNING
k8s.io      kube-system/etcd-master-1:etcd                                         k8s.gcr.io/etcd:3.3.10         805   RUNNING
k8s.io      kubelet                                                                k8s.gcr.io/hyperkube:v1.14.1   446   RUNNING
```

[musl]: https://www.musl-libc.org/
[golang]: https://golang.org/
[grpc]: https://grpc.io/
[containerd]: https://containerd.io/
[kubeadm]: https://github.com/kubernetes/kubeadm
[cri]: https://github.com/containerd/cri
[cis]: https://www.cisecurity.org/benchmark/kubernetes/
[kspp]: https://kernsec.org/wiki/index.php/Kernel_Self_Protection_Project
[nist]: https://www.nist.gov/publications/application-container-security-guide
