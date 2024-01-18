---
title: "Deploying Cilium CNI"
description: "In this guide you will learn how to set up Cilium CNI on Talos."
aliases:
  - ../../guides/deploying-cilium
---

> Cilium can be installed either via the `cilium` cli or using `helm`.

This documentation will outline installing Cilium CNI v1.14.0 on Talos in six different ways.
Adhering to Talos principles we'll deploy Cilium with IPAM mode set to Kubernetes, and using the `cgroupv2` and `bpffs` mount that talos already provides.
As Talos does not allow loading kernel modules by Kubernetes workloads, `SYS_MODULE` capability needs to be dropped from the Cilium default set of values, this override can be seen in the helm/cilium cli install commands.
Each method can either install Cilium using kube proxy (default) or without: [Kubernetes Without kube-proxy](https://docs.cilium.io/en/v1.14/network/kubernetes/kubeproxy-free/)

In this guide we assume that [KubePrism]({{< relref "../configuration/kubeprism" >}}) is enabled and configured to use the port 7445.

## Machine config preparation

When generating the machine config for a node set the CNI to none.
For example using a config patch:

Create a `patch.yaml` file with the following contents:

```yaml
cluster:
  network:
    cni:
      name: none
```

```bash
talosctl gen config \
    my-cluster https://mycluster.local:6443 \
    --config-patch @patch.yaml
```

Or if you want to deploy Cilium without kube-proxy, you also need to disable kube proxy:

Create a `patch.yaml` file with the following contents:

```yaml
cluster:
  network:
    cni:
      name: none
  proxy:
    disabled: true
```

```bash
talosctl gen config \
    my-cluster https://mycluster.local:6443 \
    --config-patch @patch.yaml
```

### Installation using Cilium CLI

> Note: It is recommended to template the cilium manifest using helm and use it as part of Talos machine config, but if you want to install Cilium using the Cilium CLI, you can follow the steps below.

Install the [Cilium CLI](https://docs.cilium.io/en/v1.13/gettingstarted/k8s-install-default/#install-the-cilium-cli) following the steps here.

#### With kube-proxy

```bash
cilium install \
    --helm-set=ipam.mode=kubernetes \
    --helm-set=kubeProxyReplacement=disabled \
    --helm-set=securityContext.capabilities.ciliumAgent="{CHOWN,KILL,NET_ADMIN,NET_RAW,IPC_LOCK,SYS_ADMIN,SYS_RESOURCE,DAC_OVERRIDE,FOWNER,SETGID,SETUID}" \
    --helm-set=securityContext.capabilities.cleanCiliumState="{NET_ADMIN,SYS_ADMIN,SYS_RESOURCE}" \
    --helm-set=cgroup.autoMount.enabled=false \
    --helm-set=cgroup.hostRoot=/sys/fs/cgroup
```

#### Without kube-proxy

```bash
cilium install \
    --helm-set=ipam.mode=kubernetes \
    --helm-set=kubeProxyReplacement=true \
    --helm-set=securityContext.capabilities.ciliumAgent="{CHOWN,KILL,NET_ADMIN,NET_RAW,IPC_LOCK,SYS_ADMIN,SYS_RESOURCE,DAC_OVERRIDE,FOWNER,SETGID,SETUID}" \
    --helm-set=securityContext.capabilities.cleanCiliumState="{NET_ADMIN,SYS_ADMIN,SYS_RESOURCE}" \
    --helm-set=cgroup.autoMount.enabled=false \
    --helm-set=cgroup.hostRoot=/sys/fs/cgroup \
    --helm-set=k8sServiceHost=localhost \
    --helm-set=k8sServicePort=7445
```

### Installation using Helm

Refer to [Installing with Helm](https://docs.cilium.io/en/v1.13/installation/k8s-install-helm/) for more information.

First we'll need to add the helm repo for Cilium.

```bash
helm repo add cilium https://helm.cilium.io/
helm repo update
```

### Method 1: Helm install

After applying the machine config and bootstrapping Talos will appear to hang on phase 18/19 with the message: retrying error: node not ready.
This happens because nodes in Kubernetes are only marked as ready once the CNI is up.
As there is no CNI defined, the boot process is pending and will reboot the node to retry after 10 minutes, this is expected behavior.

During this window you can install Cilium manually by running the following:

```bash
helm install \
    cilium \
    cilium/cilium \
    --version 1.14.0 \
    --namespace kube-system \
    --set ipam.mode=kubernetes \
    --set=kubeProxyReplacement=disabled \
    --set=securityContext.capabilities.ciliumAgent="{CHOWN,KILL,NET_ADMIN,NET_RAW,IPC_LOCK,SYS_ADMIN,SYS_RESOURCE,DAC_OVERRIDE,FOWNER,SETGID,SETUID}" \
    --set=securityContext.capabilities.cleanCiliumState="{NET_ADMIN,SYS_ADMIN,SYS_RESOURCE}" \
    --set=cgroup.autoMount.enabled=false \
    --set=cgroup.hostRoot=/sys/fs/cgroup
```

Or if you want to deploy Cilium without kube-proxy, also set some extra paramaters:

```bash
helm install \
    cilium \
    cilium/cilium \
    --version 1.14.0 \
    --namespace kube-system \
    --set ipam.mode=kubernetes \
    --set=kubeProxyReplacement=true \
    --set=securityContext.capabilities.ciliumAgent="{CHOWN,KILL,NET_ADMIN,NET_RAW,IPC_LOCK,SYS_ADMIN,SYS_RESOURCE,DAC_OVERRIDE,FOWNER,SETGID,SETUID}" \
    --set=securityContext.capabilities.cleanCiliumState="{NET_ADMIN,SYS_ADMIN,SYS_RESOURCE}" \
    --set=cgroup.autoMount.enabled=false \
    --set=cgroup.hostRoot=/sys/fs/cgroup \
    --set=k8sServiceHost=localhost \
    --set=k8sServicePort=7445
```

After Cilium is installed the boot process should continue and complete successfully.

### Method 2: Helm manifests install

Instead of directly installing Cilium you can instead first generate the manifest and then apply it:

```bash
helm template \
    cilium \
    cilium/cilium \
    --version 1.14.0 \
    --namespace kube-system \
    --set ipam.mode=kubernetes \
    --set=kubeProxyReplacement=disabled \
    --set=securityContext.capabilities.ciliumAgent="{CHOWN,KILL,NET_ADMIN,NET_RAW,IPC_LOCK,SYS_ADMIN,SYS_RESOURCE,DAC_OVERRIDE,FOWNER,SETGID,SETUID}" \
    --set=securityContext.capabilities.cleanCiliumState="{NET_ADMIN,SYS_ADMIN,SYS_RESOURCE}" \
    --set=cgroup.autoMount.enabled=false \
    --set=cgroup.hostRoot=/sys/fs/cgroup > cilium.yaml

kubectl apply -f cilium.yaml
```

Without kube-proxy:

```bash
helm template \
    cilium \
    cilium/cilium \
    --version 1.14.0 \
    --namespace kube-system \
    --set ipam.mode=kubernetes \
    --set=kubeProxyReplacement=true \
    --set=securityContext.capabilities.ciliumAgent="{CHOWN,KILL,NET_ADMIN,NET_RAW,IPC_LOCK,SYS_ADMIN,SYS_RESOURCE,DAC_OVERRIDE,FOWNER,SETGID,SETUID}" \
    --set=securityContext.capabilities.cleanCiliumState="{NET_ADMIN,SYS_ADMIN,SYS_RESOURCE}" \
    --set=cgroup.autoMount.enabled=false \
    --set=cgroup.hostRoot=/sys/fs/cgroup \
    --set=k8sServiceHost=localhost \
    --set=k8sServicePort=7445 > cilium.yaml

kubectl apply -f cilium.yaml
```

### Method 3: Helm manifests hosted install

After generating `cilium.yaml` using `helm template`, instead of applying this manifest directly during the Talos boot window (before the reboot timeout).
You can also host this file somewhere and patch the machine config to apply this manifest automatically during bootstrap.
To do this patch your machine configuration to include this config instead of the above:

Create a `patch.yaml` file with the following contents:

```yaml
cluster:
  network:
    cni:
      name: custom
      urls:
        - https://server.yourdomain.tld/some/path/cilium.yaml
```

```bash
talosctl gen config \
    my-cluster https://mycluster.local:6443 \
    --config-patch @patch.yaml
```

However, beware of the fact that the helm generated Cilium manifest contains sensitive key material.
As such you should definitely not host this somewhere publicly accessible.

### Method 4: Helm manifests inline install

A more secure option would be to include the `helm template` output manifest inside the machine configuration.
The machine config should be generated with CNI set to `none`

Create a `patch.yaml` file with the following contents:

```yaml
cluster:
  network:
    cni:
      name: none
```

```bash
talosctl gen config \
    my-cluster https://mycluster.local:6443 \
    --config-patch @patch.yaml
```

if deploying Cilium with `kube-proxy` disabled, you can also include the following:

Create a `patch.yaml` file with the following contents:

```yaml
cluster:
  network:
    cni:
      name: none
  proxy:
    disabled: true
machine:
  features:
    kubePrism:
      enabled: true
      port: 7445
```

```bash
talosctl gen config \
    my-cluster https://mycluster.local:6443 \
    --config-patch @patch.yaml
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
- As a safety messure Talos only creates missing resources from inline manifests, it never deletes or updates anything.
- If you need to update a manifest make sure to first edit all control plane machine configurations and then run `talosctl upgrade-k8s` as it will take care of updating inline manifests.

## Known issues

- There are some gotchas when using Talos and Cilium on the Google cloud platform when using internal load balancers.
For more details: [GCP ILB support / support scope local routes to be configured](https://github.com/siderolabs/talos/issues/4109)

## Other things to know

- Talos has full kernel module support for eBPF, See:
  - [Cilium System Requirements](https://docs.cilium.io/en/v1.14/operations/system_requirements/)
  - [Talos Kernel Config AMD64](https://github.com/siderolabs/pkgs/blob/main/kernel/build/config-amd64)
  - [Talos Kernel Config ARM64](https://github.com/siderolabs/pkgs/blob/main/kernel/build/config-arm64)
