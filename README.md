<p align="center">
  <h1 align="center">Talos</h1>
  <p align="center">A modern Linux distribution for Kubernetes.</p>
  <p align="center">
    <a href="https://travis-ci.org/talos-systems/talos">
      <img alt="Build Status" src="https://img.shields.io/travis/talos-systems/talos.svg?logo=travis&style=flat-square">
    </a>
    <a href="https://codecov.io/gh/talos-systems/talos">
      <img alt="Codecov" src="https://img.shields.io/codecov/c/github/talos-systems/talos.svg?style=flat-square">
    </a>
    <a href="https://github.com/talos-systems/talos/releases/latest">
      <img alt="Release" src="https://img.shields.io/github/release/talos-systems/talos.svg?logo=github&logoColor=white&style=flat-square">
    </a>
    <a href="https://github.com/talos-systems/talos/releases/latest">
      <img alt="Pre-release" src="https://img.shields.io/github/release-pre/talos-systems/talos.svg?label=pre-release&logo=GitHub&logoColor=white&style=flat-square">
    </a>
  </p>
</p>

---

**Talos** is a modern Linux distribution for Kubernetes that provides a number of capabilities. A few are:

- **Security**: reduce your attack surface by practicing the Principle of Least Privilege (PoLP) and enforcing mutual TLS (mTLS).
- **Predictability**: remove needless variables and reduce unknown factors from your environment using immutable infrastructure.
- **Evolvability**: simplify and increase your ability to easily accommodate future changes to your architecture.

For details on the design and usage of Talos, see the [documentation](https://docs.talos-systems.com).

```bash
$ kubectl get nodes -o wide
NAME              STATUS   ROLES    AGE   VERSION   INTERNAL-IP       EXTERNAL-IP   OS-IMAGE                              KERNEL-VERSION   CONTAINER-RUNTIME
192.168.124.200   Ready    master   50s   v1.13.2   192.168.124.200   <none>        Talos (v0.1.0-alpha.16) by Autonomy   4.19.10-talos    containerd://1.2.2
192.168.124.201   Ready    worker   26s   v1.13.2   192.168.124.201   <none>        Talos (v0.1.0-alpha.16) by Autonomy   4.19.10-talos    containerd://1.2.2
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
NAMESPACE   ID       IMAGE          PID    STATUS
system      blockd   talos/blockd   1461   RUNNING
system      osd      talos/osd      1449   RUNNING
system      proxyd   talos/proxyd   2754   RUNNING
system      trustd   talos/trustd   1451   RUNNING
```

or query the containers in the `k8s.io` [`namespace`](https://github.com/containerd/containerd/blob/master/docs/namespaces.md):

```bash
$ osctl ps -k
NAMESPACE   ID                                                                 IMAGE                                                                     PID    STATUS
k8s.io      0ca1fc5944d6ed075a33197921e0ca4dd4937ae243e428b570fea87ff34f1811   sha256:da86e6ba6ca197bf6bc5e9d900febd906b133eaa4750e6bed647b0fbe50ed43e   2341   RUNNING
k8s.io      356fc70fa1ba691deadf544b9ab4ade2256084a090a711eec3e70fc810709374   sha256:da86e6ba6ca197bf6bc5e9d900febd906b133eaa4750e6bed647b0fbe50ed43e   2342   RUNNING
...
k8s.io      e42ec788edc1e3af71cb6fa151dd8cc1076906dbe09d7099697f36069e38b5a8   sha256:4ff8d484069d463252df6a461ba13f073b247a4f19e421b3117c584d39b4a67f   2508   RUNNING
k8s.io      kubelet                                                            k8s.gcr.io/hyperkube:v1.13.2                                              2068   RUNNING
```

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md)

## Contact

### Slack

If you would like to participate in discussions about Talos, please send an email to maintainers@talos-systems.io with the subject line "Slack Invite", and we would be happy to send an invite to our workspace.

> It is important that the subject line is _exactly_ "Slack Invite" (exclude the double quotes).

### Twitter

![Twitter Follow](https://img.shields.io/twitter/follow/talossystems.svg?style=social)

## Changelog

See [CHANGELOG.md](CHANGELOG.md)

## FAQs

**Why "Talos"?**

> Talos was an automaton created by the Greek God of the forge to protect the island of Crete.
> He would patrol the coast and enforce laws throughout the land.
> We felt it was a fitting name for a security focused Linux distribution designed to run Kubernetes.

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
> Talos tightly integrates with Kubernetes, and is not meant to be a general use Linux distribution.
> This allows us to dramatically decrease the footprint of Talos, and in turn improve a number of other areas like security, predictability, and reliability.
> In addition to this, interaction with the host is done through a secure gRPC API.
> If you want to run Kubernetes with zero cruft, Talos is the perect fit.

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
