---
title: "Deploying Cilium CNI"
description: "In this guide you will learn how to set up Cilium CNI on Talos."
---

From v1.9 onwards cilium doesn't provide a one liner install manifest that can be used to install cilium on a node via `kubectl apply -f` or passing in as extra `urls` in Talos machine configuration.

> installing Cilium via `cilium` cli is [broken](https://github.com/cilium/cilium-cli/issues/505), so we'll be using `helm` to install cilium.

Refer [Installing with Helm](https://docs.cilium.io/en/v1.11/gettingstarted/k8s-install-helm/) for more information.

First we'll need to add the helm repo for cilium.

```bash
helm repo add cilium https://helm.cilium.io/
helm repo update
```

This documentation will outline installing Cilium CNI on Talos in two different ways.

## With Kube Proxy enabled

When generating the machine config for a node add the following config patch.
An example usage is shown below:

```bash
talosctl gen config \
    my-cluster https://mycluster.local:6443 \
    --config-patch '[{"op":"add", "path": "/cluster/network/cni", "value": {"name": "none"}}]'
```

Now we can move onto installing cilium.

If you want to install with helm run the following:

```bash
helm install cilium cilium/cilium \
    --version 1.11.0 \
    --namespace kube-system
```

If you want to generate a manifest and apply manually run the following:

```bash
helm template cilium cilium/cilium \
    --version 1.11.0 \
    --namespace kube-system > cilium.yaml

kubectl apply -f cilium.yaml
```

## Without Kube Proxy

If you want to deploy Cilium in strict mode without kube-proxy, you can use the following config patch when generating a machine config.
This will create the Talos cluster with no CNI and *kube-proxy* disabled.

An example usage is shown below:

```bash
talosctl gen config \
    my-cluster https://mycluster.local:6443 \
    --config-patch '[{"op": "add", "path": "/cluster/proxy", "value": {"disabled": true}}, {"op":"add", "path": "/cluster/network/cni", "value": {"name": "none"}}]'
```

You need to pass in the Kubernetes API server address to the `helm` commands.
Refer [Kube Proxy free](https://docs.cilium.io/en/v1.11/gettingstarted/kubeproxy-free/#quick-start) for more information.

```bash
export KUBERNETES_API_SERVER_ADDRESS=<>
export KUBERNETES_API_SERVER_PORT=6443
```

If you want to install with helm run the following:

```bash
helm install cilium cilium/cilium \
    --version 1.11.0 \
    --namespace kube-system \
    --set kubeProxyReplacement=strict \
    --set k8sServiceHost="${KUBERNETES_API_SERVER_ADDRESS}" \
    --set k8sServicePort="${KUBERNETES_API_SERVER_PORT}"
```

If you want to generate a manifest and apply manually run the following:

```bash
helm template cilium cilium/cilium \
    --version 1.11.0 \
    --namespace kube-system \
    --set kubeProxyReplacement=strict \
    --set k8sServiceHost="${KUBERNETES_API_SERVER_ADDRESS}" \
    --set k8sServicePort="${KUBERNETES_API_SERVER_PORT}" > cilium.yaml

kubectl apply -f cilium.yaml
```
