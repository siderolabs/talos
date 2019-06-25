---
title: Getting Started
date: 2019-06-21T06:25:46-08:00
draft: false
menu:
  docs:
    parent: 'guides'
    weight: 1
---

In this guide we will create a Kubernetes cluster in Docker, using a containerized version of Talos.
The cluster will consist of 3 master nodes, and 1 worker node.

Running Talos in Docker is intended to be used in CI pipelines, and local testing when you need a quick and easy cluster.
Furthermore, if you are running Talos in production, it provides an excellent way for developers to develop against the same version of Talos.

## Requirements

The follow are requirements for running Talos in Docker:

- Docker 18.03 or greater
- a recent version of [`osctl`](https://github.com/talos-systems/talos/releases)

## Create the Cluster

Creating a local cluster is as simple as:

```bash
osctl cluster create
```

Once the above finishes successfully, your talosconfig(`~/.talos/config`) will be configured to point to the new cluster.

{{% note %}}Startup times can take up to a minute before the cluster is available.{{% /note %}}

## Configure the Cluster

Once the cluster is available, the pod security policies will need to be applied to allow the control plane to come up.
Following that, the default CNI (flannel) configuration will be applied.

### Retreive and Configure the `kubeconfig`

```bash
osctl kubeconfig | sed -e 's/10.5.0.2:/127.0.0.1:6/' > kubeconfig
```

### Apply a Pod Security Policy

The first thing we need to do is apply a PSP manifest:

```bash
kubectl --kubeconfig ./kubeconfig apply -f https://raw.githubusercontent.com/talos-systems/talos/master/hack/dev/manifests/psp.yaml
```

{{% note %}}Talos enforces the use of [Pod Security Policies](https://kubernetes.io/docs/concepts/policy/pod-security-policy/).{{% /note %}}

### Deploy the CNI Provider

In this example we will deploy flannel, but Calico, and Cillium are known to work.

```bash
kubectl --kubeconfig ./kubeconfig apply -f https://raw.githubusercontent.com/talos-systems/talos/master/hack/dev/manifests/flannel.yaml
```

### Configure CoreDNS

Finally we need to fix loop detection for Docker dns:

```bash
kubectl --kubeconfig ./kubeconfig apply -f https://raw.githubusercontent.com/talos-systems/talos/master/hack/dev/manifests/coredns.yaml
```

## Using the Cluster

Once the cluster is available, you can make use of `osctl` and `kubectl` to interact with the cluster.
For example, to view current running containers, run `osctl ps` for a list of containers in the `system` namespace, or `osctl ps -k` for the `k8s.io` namespace.
To view the logs of a container, use `osctl logs <container>` or `osctl logs -k <container>`.

{{% note %}}We only set up port forwarding to master-1 so other nodes will not be directly accessible.{{% /note %}}
