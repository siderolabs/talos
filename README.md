<p align="center">
  <h1 align="center">Talos</h1>
  <p align="center">A modern operating system for Kubernetes.</p>
  <p align="center">
    <a href="https://github.com/talos-systems/talos/releases/latest">
      <img alt="Release" src="https://img.shields.io/github/release/talos-systems/talos.svg?logo=github&logoColor=white&style=flat-square">
    </a>
    <a href="https://github.com/talos-systems/talos/releases/latest">
      <img alt="Pre-release" src="https://img.shields.io/github/release-pre/talos-systems/talos.svg?label=pre-release&logo=GitHub&logoColor=white&style=flat-square">
    </a>
  </p>
</p>

---

**Talos** is a modern Linux distribution designed to be secure, immutable, and minimal. All system management is done via an API, and there is no shell or interactive console. Some of the capabilities and benefits provided by Talos include:

- **Security**: Talos reduces your attack surface by practicing the Principle of Least Privilege (PoLP) and by securing the API with mutual TLS (mTLS) authentication.
- **Predictability**: Talos eliminates unneeded variables and reduces unknown factors in your environment by employing immutable infrastructure ideology.
- **Evolvability**: Talos simplifies your architecture and increases your ability to easily accommodate future changes.

For details on the design and usage of Talos, see the [documentation](https://docs.talos-systems.com).

```bash
$ kubectl get nodes -o wide
NAME       STATUS   ROLES    AGE   VERSION   INTERNAL-IP   EXTERNAL-IP   OS-IMAGE                  KERNEL-VERSION   CONTAINER-RUNTIME
master-1   Ready    master   79s   v1.14.1   10.5.0.2      <none>        Talos (v0.1.0-alpha.24)   4.19.34-talos    containerd://1.2.6
master-2   Ready    master   42s   v1.14.1   10.5.0.3      <none>        Talos (v0.1.0-alpha.24)   4.19.34-talos    containerd://1.2.6
master-3   Ready    master   42s   v1.14.1   10.5.0.4      <none>        Talos (v0.1.0-alpha.24)   4.19.34-talos    containerd://1.2.6
worker-1   Ready    worker   44s   v1.14.1   10.5.0.5      <none>        Talos (v0.1.0-alpha.24)   4.19.34-talos    containerd://1.2.6
```

## Quick Start

The quickest way to get started with Talos is to create a local docker-based cluster:

```bash
osctl cluster create
```

> Note: You can download `osctl` from the latest [release](https://github.com/talos-systems/talos/releases/latest).

Once the cluster is up, download the kubeconfig:

```bash
osctl kubeconfig > kubeconfig
kubectl --kubeconfig kubeconfig config set-cluster talos_default --server https://127.0.0.1:6443
```

> Note: It can take up to a minute for the kubeconfig to be available.

To cleanup, run:

```bash
osctl cluster destroy
```

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

## Community

See [community](docs/content/community/index.md)

## Contributing

See [contributing](CONTRIBUTING.md)

## Changelog

See [CHANGELOG](CHANGELOG.md)

## FAQs

**Why "Talos"?**

> Talos was an automaton created by the Greek God of the forge to protect the island of Crete.
> He would patrol the coast and enforce laws throughout the land.
> We felt it was a fitting name for a security focused operating system designed to run Kubernetes.

**Why no shell or SSH?**

> We would like for Talos users to start thinking about what a "machine" is in the context of a Kubernetes cluster.
> That is that a Kubernetes _cluster_ can be thought of as one massive machine and the _nodes_ merely as additional resources.
> We don't want humans to focus on the _nodes_, but rather the _machine_ that is the Kubernetes cluster.
> Should an issue arise at the node level, osctl should provide the necessary tooling to assist in the identification, debugging, and remediation of the issue.
> However, the API is based on the Principle of Least Privilege, and exposes only a limited set of methods.
> We aren't quite there yet, but we envision Talos being a great place for the application of [control theory](https://en.wikipedia.org/wiki/Control_theory) in order to provide a self-healing platform.

**How is Talos different than CoreOS/RancherOS/Linuxkit?**

> Talos is similar in many ways, but there are some differences that make it unique.
> You can imagine Talos as a container image, in that it is immutable and built with a single purpose in mind.
> In this case, that purpose is Kubernetes.
> Talos tightly integrates with Kubernetes, and is not meant to be a general use operating system.
> This allows us to dramatically decrease the footprint of Talos, and in turn improve a number of other areas like security, predictability, and reliability.
> In addition to this, interaction with the host is done through a secure gRPC API.
> If you want to run Kubernetes with zero cruft, Talos is the perfect fit.

## License

[![license](https://img.shields.io/github/license/talos-systems/talos.svg?style=flat-square)](https://github.com/talos-systems/talos/blob/master/LICENSE)

[musl]: https://www.musl-libc.org/
[golang]: https://golang.org/
[grpc]: https://grpc.io/
[containerd]: https://containerd.io/
[kubeadm]: https://github.com/kubernetes/kubeadm
[cri]: https://github.com/containerd/cri
[cis]: https://www.cisecurity.org/benchmark/kubernetes/
[kspp]: https://kernsec.org/wiki/index.php/Kernel_Self_Protection_Project
[nist]: https://www.nist.gov/publications/application-container-security-guide
