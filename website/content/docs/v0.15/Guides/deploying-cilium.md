---
title: "Deploying Cilium CNI"
description: "In this guide you will learn how to set up Cilium CNI on Talos."
---

From v1.9 onwards Cilium doesn't provide a one-liner install manifest that can
be used to install Cilium on a node via `kubectl apply -f` or by passing in as
extra `urls` in Talos machine configuration.

> Installing Cilium via `cilium` CLI is [broken](https://github.com/cilium/cilium-cli/issues/505),
so we'll be using `helm` to install Cilium.

Refer [Installing with Helm](https://docs.cilium.io/en/v1.11/gettingstarted/k8s-install-helm/)
for more information.

First we'll need to add the helm repo for Cilium.

```bash
helm repo add cilium https://helm.cilium.io/
helm repo update
```

This documentation will outline installing Cilium CNI on Talos in two different
ways.

> **Note:** When building a new cluster, an initial CNI must be provided. If you
want to build your cluster with Cilium as the CNI provider out of the box, you
will first have to generate the Cilium manifests and host them somewhere the
API can reach. The Cilium manifests will have secret values, so keep that in
consideration for where you host them.

## With Kube Proxy enabled

The following steps will create a Talos cluster with Cilium installed.

Generate the Cilium manifests with:

```bash
helm template cilium cilium/cilium \
    --version 1.11.0 \
    --namespace kube-system > cilium.yaml
```

Then host that `cilium.yaml` file somewhere secure.

When generating the machine config for a node, add the following config patch
with your `cilium.yaml` URL:

```bash
talosctl gen config \
    my-cluster https://mycluster.local:6443 \
    --config-patch '[{"op":"add", "path": "/cluster/network/cni", "value": {"name": "custom", "urls": ["<your_hosting_url>/cilium.yaml"]}}]'
```

Next, update the generated `talosconf` file to point to your endpoint:

```bash
talosctl --talosconfig ./talosconfig config endpoint mycluster.local
```

Finally, create your cluster using your generated config files. For example
with a local Docker deployment:

```bash
talosctl cluster create --input-dir . --wait
```

## Without Kube Proxy

The following steps will create a Talos cluster with Cilium installed and
*kube-proxy* disabled.

Generate the Cilium manifests with the commands below. You need to pass in the
Kubernetes API server address to the `helm` commands.
Refer to [Kube Proxy free](https://docs.cilium.io/en/v1.11/gettingstarted/kubeproxy-free/#quick-start)
for more information.

```bash
export KUBERNETES_API_SERVER_ADDRESS=mycluster.local
export KUBERNETES_API_SERVER_PORT=6443

helm template cilium cilium/cilium \
    --version 1.11.0 \
    --namespace kube-system \
    --set kubeProxyReplacement=strict \
    --set k8sServiceHost="${KUBERNETES_API_SERVER_ADDRESS}" \
    --set k8sServicePort="${KUBERNETES_API_SERVER_PORT}" > cilium.yaml
```

Then host that `cilium.yaml` file somewhere secure.

When generating the machine config for a node, add the following config patch
with your `cilium.yaml` URL:

```bash
talosctl gen config \
    my-cluster https://mycluster.local:6443 \
    --config-patch '[{"op": "add", "path": "/cluster/proxy", "value": {"disabled": true}}, {"op":"add", "path": "/cluster/network/cni", "value": {"name": "custom", "urls": ["<your_hosting_url>/cilium.yaml"]}}]'
```

Next, update the generated `talosconf` file to point to your endpoint:

```bash
talosctl --talosconfig ./talosconfig config endpoint mycluster.local
```

Finally, create your cluster using your generated config files. For example
with a local Docker deployment:

```bash
talosctl cluster create --input-dir . --wait
```
