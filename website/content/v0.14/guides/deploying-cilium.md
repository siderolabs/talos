---
title: "Deploying Cilium CNI"
description: "In this guide you will learn how to set up Cilium CNI on Talos."
---

From v1.9 onwards Cilium does no longer provide a one-liner install manifest that can be used to install Cilium on a node via `kubectl apply -f` or passing it in as an extra url in the `urls` part in the Talos machine configuration.

> Installing Cilium the new way via the `cilium` cli is broken, so we'll be using `helm` to install Cilium.
For more information: [Install with CLI fails, works with Helm](https://github.com/cilium/cilium-cli/issues/505)

Refer to [Installing with Helm](https://docs.cilium.io/en/v1.11/gettingstarted/k8s-install-helm/) for more information.

First we'll need to add the helm repo for Cilium.

```bash
helm repo add cilium https://helm.cilium.io/
helm repo update
```

This documentation will outline installing Cilium CNI v1.11.2 on Talos in four different ways.
Adhering to Talos principles we'll deploy Cilium with IPAM mode set to Kubernetes.
Each method can either install Cilium using kube proxy (default) or without: [Kubernetes Without kube-proxy](https://docs.cilium.io/en/v1.11/gettingstarted/kubeproxy-free/)

## Machine config preparation

When generating the machine config for a node set the CNI to none.
For example using a config patch:

```bash
talosctl gen config \
    my-cluster https://mycluster.local:6443 \
    --config-patch '[{"op":"add", "path": "/cluster/network/cni", "value": {"name": "none"}}]'
```

Or if you want to deploy Cilium in strict mode without kube-proxy, you also need to disable kube proxy:

```bash
talosctl gen config \
    my-cluster https://mycluster.local:6443 \
    --config-patch '[{"op": "add", "path": "/cluster/proxy", "value": {"disabled": true}}, {"op":"add", "path": "/cluster/network/cni", "value": {"name": "none"}}]'
```

## Method 1: Helm install

After applying the machine config and bootstrapping Talos will appear to hang on phase 18/19 with the message: retrying error: node not ready.
This happens because nodes in Kubernetes are only marked as ready once the CNI is up.
As there is no CNI defined, the boot process is pending and will reboot the node to retry after 10 minutes, this is expected behavior.

During this window you can install Cilium manually by running the following:

```bash
helm install cilium cilium/cilium \
    --version 1.11.2 \
    --namespace kube-system \
    --set ipam.mode=kubernetes
```

Or if you want to deploy Cilium in strict mode without kube-proxy, also set some extra paramaters:

```bash
export KUBERNETES_API_SERVER_ADDRESS=<>
export KUBERNETES_API_SERVER_PORT=6443

helm install cilium cilium/cilium \
    --version 1.11.2 \
    --namespace kube-system \
    --set ipam.mode=kubernetes \
    --set kubeProxyReplacement=strict \
    --set k8sServiceHost="${KUBERNETES_API_SERVER_ADDRESS}" \
    --set k8sServicePort="${KUBERNETES_API_SERVER_PORT}"
```

After Cilium is installed the boot process should continue and complete successfully.

## Method 2: Helm manifests install

Instead of directly installing Cilium you can instead first generate the manifest and then apply it:

```bash
helm template cilium cilium/cilium \
    --version 1.11.2 \
    --namespace kube-system
    --set ipam.mode=kubernetes > cilium.yaml

kubectl apply -f cilium.yaml
```

Without kube-proxy:

```bash
export KUBERNETES_API_SERVER_ADDRESS=<>
export KUBERNETES_API_SERVER_PORT=6443

helm template cilium cilium/cilium \
    --version 1.11.2 \
    --namespace kube-system \
    --set ipam.mode=kubernetes \
    --set kubeProxyReplacement=strict \
    --set k8sServiceHost="${KUBERNETES_API_SERVER_ADDRESS}" \
    --set k8sServicePort="${KUBERNETES_API_SERVER_PORT}" > cilium.yaml

kubectl apply -f cilium.yaml
```

## Method 3: Helm manifests hosted install

After generating `cilium.yaml` using `helm template`, instead of applying this manifest directly during the Talos boot window (before the reboot timeout).
You can also host this file somewhere and patch the machine config to apply this manifest automatically during bootstrap.
To do this patch your machine configuration to include this config instead of the above:

```bash
talosctl gen config \
    my-cluster https://mycluster.local:6443 \
    --config-patch '[{"op":"add", "path": "/cluster/network/cni", "value": {"name": "custom", "urls": ["https://server.yourdomain.tld/some/path/cilium.yaml"]}}]'
```

Resulting in a config that look like this:

``` yaml
name: custom # Name of CNI to use.
# URLs containing manifests to apply for the CNI.
urls:
    - https://server.yourdomain.tld/some/path/cilium.yaml
```

However, beware of the fact that the helm generated Cilium manifest contains sensitive key material.
As such you should definitely not host this somewhere publicly accessible.

## Method 4: Helm manifests inline install

A more secure option would be to include the `helm template` output manifest inside the machine configuration.
The machine config should be generated with CNI set to `none`

```bash
talosctl gen config \
    my-cluster https://mycluster.local:6443 \
    --config-patch '[{"op":"add", "path": "/cluster/network/cni", "value": {"name": "none"}}]'
```

if deploying Cilium with `kube-proxy` disabled, you can also include the following:

```bash
talosctl gen config \
    my-cluster https://mycluster.local:6443 \
    --config-patch '[{"op": "add", "path": "/cluster/proxy", "value": {"disabled": true}}, {"op":"add", "path": "/cluster/network/cni", "value": {"name": "none"}}]'
```

To do so patch this into your machine configuration:

``` yaml
inlineManifests:
    - name: cilium
      contents: |
        --
        # Source: cilium/templates/cilium-agent/serviceaccount.yaml
        apiVersion: v1
        kind: ServiceAccount
        metadata:
          name: "cilium"
          namespace: kube-system
        ---
        # Source: cilium/templates/cilium-operator/serviceaccount.yaml
        apiVersion: v1
        kind: ServiceAccount
        -> Your cilium.yaml file will be pretty long....
```

This will install the Cilium manifests at just the right time during bootstrap.

Beware though:

- Changing the namespace when templating with Helm does not generate a manifest containing the yaml to create that namespace.
As the inline manifest is processed from top to bottom make sure to manually put the namespace yaml at the start of the inline manifest.
- Only add the Cilium inline manifest to the control plane nodes machine configuration.
- Make sure all control plane nodes have an identical configuration.
- If you delete any of the generated resources they will be restored whenever a control plane node reboots.
- As a safety measure Talos only creates missing resources from inline manifests, it never deletes or updates anything.
- If you need to update a manifest make sure to first edit all control plane machine configurations and then run `talosctl upgrade-k8s` as it will take care of updating inline manifests.

## Known issues

- Currently there is an interaction between a Kubespan enabled Talos cluster and Cilium that results in the cluster going down during bootstrap after applying the Cilium manifests.
For more details: [Kubespan and Cilium compatiblity: etcd is failing](https://github.com/talos-systems/talos/issues/4836)

- When running Cilium with a kube-proxy eBPF replacement (strict mode) there is a conflicting kernel module that results in locked tx queues.
This can be fixed by blacklisting `aoe_init` with extraKernelArgs.
For more details: [Cilium on talos "aoe: packet could not be sent on \*. consider increasing tx_queue_len"](https://github.com/talos-systems/talos/issues/4863)

- There are some gotchas when using Talos and Cilium on the Google cloud platform when using internal load balancers.
For more details: [GCP ILB support / support scope local routes to be configured](https://github.com/talos-systems/talos/issues/4109)

- Some kernel values changed by kube-proxy are not set to good defaults when running the cilium kernel-proxy alternative.
For more details: [Kernel default values (sysctl)](https://github.com/talos-systems/talos/issues/4654)

## Other things to know

- Talos has full kernel module support for eBPF, See:
  - [Cilium System Requirements](https://docs.cilium.io/en/v1.11/operations/system_requirements/)
  - [Talos Kernel Config AMD64](https://github.com/talos-systems/pkgs/blob/master/kernel/build/config-amd64)
  - [Talos Kernel Config ARM64](https://github.com/talos-systems/pkgs/blob/master/kernel/build/config-arm64)

- Talos also includes the modules:

  - `CONFIG_NETFILTER_XT_TARGET_TPROXY=m`
  - `CONFIG_NETFILTER_XT_TARGET_CT=m`
  - `CONFIG_NETFILTER_XT_MATCH_MARK=m`
  - `CONFIG_NETFILTER_XT_MATCH_SOCKET=m`

  This allows you to set `--set enableXTSocketFallback=false` on the helm install/template command preventing Cilium from disabling the `ip_early_demux` kernel feature.
This will win back some performance.
