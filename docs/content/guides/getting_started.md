---
title: Getting Started
date: 2019-06-21T06:25:46-08:00
draft: false
menu:
  docs:
    parent: 'guides'
    weight: 1
---

The quickest way to get started with Talos is to run a small cluster in a local Docker-based environment on your laptop or workstation. The default test cluster will consist of 3 master nodes, and 1 worker node. This configuration will be sufficient for experimentation and development.

## Environment

To run the Talos test environment, you will need to make sure you have Docker installed and running, as well as the most recent `osctl` release, which can be found on the [Talos Releases](https://github.com/talos-systems/talos/releases) page. Download the appropriate binary for your system: ``osctl-darwin-amd64`` for MacOS, and ``osctl-linux-amd64`` for Linux.

## Bring up the Docker Environment

```bash
osctl cluster create
```

Startup times can vary, but it typically takes ~45s-1min for the environment to be available.

## Apply PSP and CNI

Once the environment is available, the pod security policies will need to be applied to allow the control plane to come up. Following that, the default CNI (flannel) configuration will be applied.

```bash
# Fix up kubeconfig to use localhost since we're connecting to a local docker instance
osctl kubeconfig | sed -e 's/10.5.0.2:/127.0.0.1:6/' > kubeconfig

# Apply PSP
kubectl --kubeconfig ./kubeconfig apply -f \
  https://raw.githubusercontent.com/talos-systems/talos/master/hack/dev/manifests/psp.yaml

# Apply CNI
kubectl --kubeconfig ./kubeconfig apply -f \
  https://raw.githubusercontent.com/talos-systems/talos/master/hack/dev/manifests/flannel.yaml

# Fix loop detection for docker dns
kubectl --kubeconfig ./kubeconfig apply -f \
  https://raw.githubusercontent.com/talos-systems/talos/master/hack/dev/manifests/coredns.yaml
```

## Interact with the environment

Once the environment is available, you should be able to make use of `osctl` and `kubectl` commands.
You can view the current running containers via `osctl ps` and `osctl ps -k`. You can view logs of running containers via `osctl logs <container>` or `osctl logs -k <container>`

**Note** We only set up port forwarding to master-1 so other nodes will not be directly accessible.
