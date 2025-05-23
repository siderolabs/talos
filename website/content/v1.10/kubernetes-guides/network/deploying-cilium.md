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
Each method can either install Cilium using kube proxy (default) or without: [Kubernetes Without kube-proxy](https://docs.cilium.io/en/v1.16/network/kubernetes/kubeproxy-free/)

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

Install the [Cilium CLI](https://docs.cilium.io/en/v1.16/gettingstarted/k8s-install-default/#install-the-cilium-cli) following the steps here.

#### With kube-proxy

```bash
cilium install \
    --set ipam.mode=kubernetes \
    --set kubeProxyReplacement=false \
    --set securityContext.capabilities.ciliumAgent="{CHOWN,KILL,NET_ADMIN,NET_RAW,IPC_LOCK,SYS_ADMIN,SYS_RESOURCE,DAC_OVERRIDE,FOWNER,SETGID,SETUID}" \
    --set securityContext.capabilities.cleanCiliumState="{NET_ADMIN,SYS_ADMIN,SYS_RESOURCE}" \
    --set cgroup.autoMount.enabled=false \
    --set cgroup.hostRoot=/sys/fs/cgroup
```

#### Without kube-proxy

```bash
cilium install \
    --set ipam.mode=kubernetes \
    --set kubeProxyReplacement=true \
    --set securityContext.capabilities.ciliumAgent="{CHOWN,KILL,NET_ADMIN,NET_RAW,IPC_LOCK,SYS_ADMIN,SYS_RESOURCE,DAC_OVERRIDE,FOWNER,SETGID,SETUID}" \
    --set securityContext.capabilities.cleanCiliumState="{NET_ADMIN,SYS_ADMIN,SYS_RESOURCE}" \
    --set cgroup.autoMount.enabled=false \
    --set cgroup.hostRoot=/sys/fs/cgroup \
    --set k8sServiceHost=localhost \
    --set k8sServicePort=7445
```

Or if you want to deploy Cilium with support for Gateway API (requires installing cilium without kube-proxy), install [Gateway API CRDs](https://docs.cilium.io/en/v1.16/network/servicemesh/gateway-api/gateway-api/#prerequisites) and set some extra parameters:

```bash
cilium install \
    --set ipam.mode=kubernetes \
    --set kubeProxyReplacement=true \
    --set securityContext.capabilities.ciliumAgent="{CHOWN,KILL,NET_ADMIN,NET_RAW,IPC_LOCK,SYS_ADMIN,SYS_RESOURCE,DAC_OVERRIDE,FOWNER,SETGID,SETUID}" \
    --set securityContext.capabilities.cleanCiliumState="{NET_ADMIN,SYS_ADMIN,SYS_RESOURCE}" \
    --set cgroup.autoMount.enabled=false \
    --set cgroup.hostRoot=/sys/fs/cgroup \
    --set k8sServiceHost=localhost \
    --set k8sServicePort=7445 \
    --set gatewayAPI.enabled=true \
    --set gatewayAPI.enableAlpn=true \
    --set gatewayAPI.enableAppProtocol=true
```

> Note: If you plan to use gRPC and GRPCRoutes with TLS, you must enable ALPN by setting `gatewayAPI.enableAlpn=true`.
> Since gRPC relies on HTTP/2, ALPN is required to negotiate HTTP/2 support between the client and server.

### Installation using Helm

Refer to [Installing with Helm](https://docs.cilium.io/en/v1.16/installation/k8s-install-helm/) for more information.

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
    --version 1.15.6 \
    --namespace kube-system \
    --set ipam.mode=kubernetes \
    --set kubeProxyReplacement=false \
    --set securityContext.capabilities.ciliumAgent="{CHOWN,KILL,NET_ADMIN,NET_RAW,IPC_LOCK,SYS_ADMIN,SYS_RESOURCE,DAC_OVERRIDE,FOWNER,SETGID,SETUID}" \
    --set securityContext.capabilities.cleanCiliumState="{NET_ADMIN,SYS_ADMIN,SYS_RESOURCE}" \
    --set cgroup.autoMount.enabled=false \
    --set cgroup.hostRoot=/sys/fs/cgroup
```

Or if you want to deploy Cilium without kube-proxy, also set some extra parameters:

```bash
helm install \
    cilium \
    cilium/cilium \
    --version 1.15.6 \
    --namespace kube-system \
    --set ipam.mode=kubernetes \
    --set kubeProxyReplacement=true \
    --set securityContext.capabilities.ciliumAgent="{CHOWN,KILL,NET_ADMIN,NET_RAW,IPC_LOCK,SYS_ADMIN,SYS_RESOURCE,DAC_OVERRIDE,FOWNER,SETGID,SETUID}" \
    --set securityContext.capabilities.cleanCiliumState="{NET_ADMIN,SYS_ADMIN,SYS_RESOURCE}" \
    --set cgroup.autoMount.enabled=false \
    --set cgroup.hostRoot=/sys/fs/cgroup \
    --set k8sServiceHost=localhost \
    --set k8sServicePort=7445
```

And with GatewayAPI support:

```bash
...
    --set=gatewayAPI.enabled=true \
    --set=gatewayAPI.enableAlpn=true \
    --set=gatewayAPI.enableAppProtocol=true
```

After Cilium is installed the boot process should continue and complete successfully.

### Method 2: Helm manifests install

Instead of directly installing Cilium you can instead first generate the manifest and then apply it:

```bash
helm template \
    cilium \
    cilium/cilium \
    --version 1.15.6 \
    --namespace kube-system \
    --set ipam.mode=kubernetes \
    --set kubeProxyReplacement=false \
    --set securityContext.capabilities.ciliumAgent="{CHOWN,KILL,NET_ADMIN,NET_RAW,IPC_LOCK,SYS_ADMIN,SYS_RESOURCE,DAC_OVERRIDE,FOWNER,SETGID,SETUID}" \
    --set securityContext.capabilities.cleanCiliumState="{NET_ADMIN,SYS_ADMIN,SYS_RESOURCE}" \
    --set cgroup.autoMount.enabled=false \
    --set cgroup.hostRoot=/sys/fs/cgroup > cilium.yaml

kubectl apply -f cilium.yaml
```

Without kube-proxy:

```bash
helm template \
    cilium \
    cilium/cilium \
    --version 1.15.6 \
    --namespace kube-system \
    --set ipam.mode=kubernetes \
    --set kubeProxyReplacement=true \
    --set securityContext.capabilities.ciliumAgent="{CHOWN,KILL,NET_ADMIN,NET_RAW,IPC_LOCK,SYS_ADMIN,SYS_RESOURCE,DAC_OVERRIDE,FOWNER,SETGID,SETUID}" \
    --set securityContext.capabilities.cleanCiliumState="{NET_ADMIN,SYS_ADMIN,SYS_RESOURCE}" \
    --set cgroup.autoMount.enabled=false \
    --set cgroup.hostRoot=/sys/fs/cgroup \
    --set k8sServiceHost=localhost \
    --set k8sServicePort=7445 > cilium.yaml

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
```

```bash
talosctl gen config \
  my-cluster https://mycluster.local:6443 \
  --config-patch @patch.yaml
```

To do so patch this into your machine configuration:

``` yaml
cluster:
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
- As a safety measure, Talos only creates missing resources from inline manifests, it never deletes or updates anything.
- If you need to update a manifest make sure to first edit all control plane machine configurations and then run `talosctl upgrade-k8s` as it will take care of updating inline manifests.

### Method 5: Using a job

We can utilize a job pattern run arbitrary logic during bootstrap time.
We can leverage this to our advantage to install Cilium by using an inline manifest as shown in the example below:

``` yaml
cluster:
  inlineManifests:
    - name: cilium-install
      contents: |
        ---
        apiVersion: rbac.authorization.k8s.io/v1
        kind: ClusterRoleBinding
        metadata:
          name: cilium-install
        roleRef:
          apiGroup: rbac.authorization.k8s.io
          kind: ClusterRole
          name: cluster-admin
        subjects:
        - kind: ServiceAccount
          name: cilium-install
          namespace: kube-system
        ---
        apiVersion: v1
        kind: ServiceAccount
        metadata:
          name: cilium-install
          namespace: kube-system
        ---
        apiVersion: batch/v1
        kind: Job
        metadata:
          name: cilium-install
          namespace: kube-system
        spec:
          backoffLimit: 10
          template:
            metadata:
              labels:
                app: cilium-install
            spec:
              restartPolicy: OnFailure
              tolerations:
                - operator: Exists
                - effect: NoSchedule
                  operator: Exists
                - effect: NoExecute
                  operator: Exists
                - effect: PreferNoSchedule
                  operator: Exists
                - key: node-role.kubernetes.io/control-plane
                  operator: Exists
                  effect: NoSchedule
                - key: node-role.kubernetes.io/control-plane
                  operator: Exists
                  effect: NoExecute
                - key: node-role.kubernetes.io/control-plane
                  operator: Exists
                  effect: PreferNoSchedule
              affinity:
                nodeAffinity:
                  requiredDuringSchedulingIgnoredDuringExecution:
                    nodeSelectorTerms:
                      - matchExpressions:
                          - key: node-role.kubernetes.io/control-plane
                            operator: Exists
              serviceAccount: cilium-install
              serviceAccountName: cilium-install
              hostNetwork: true
              containers:
              - name: cilium-install
                image: quay.io/cilium/cilium-cli:latest
                env:
                - name: KUBERNETES_SERVICE_HOST
                  valueFrom:
                    fieldRef:
                      apiVersion: v1
                      fieldPath: status.podIP
                - name: KUBERNETES_SERVICE_PORT
                  value: "6443"
                command:
                  - cilium
                  - install
                  - --set
                  - ipam.mode=kubernetes
                  - --set
                  - kubeProxyReplacement=true
                  - --set
                  - securityContext.capabilities.ciliumAgent={CHOWN,KILL,NET_ADMIN,NET_RAW,IPC_LOCK,SYS_ADMIN,SYS_RESOURCE,DAC_OVERRIDE,FOWNER,SETGID,SETUID}
                  - --set
                  - securityContext.capabilities.cleanCiliumState={NET_ADMIN,SYS_ADMIN,SYS_RESOURCE}
                  - --set
                  - cgroup.autoMount.enabled=false
                  - --set
                  - cgroup.hostRoot=/sys/fs/cgroup
                  - --set
                  - k8sServiceHost=localhost
                  - --set
                  - k8sServicePort=7445
```

Because there is no CNI present at installation time the kubernetes.default.svc cannot be used to install Cilium, to overcome this limitation we'll utilize the host network connection to connect back to itself with 'hostNetwork: true' in tandem with the environment variables KUBERNETES_SERVICE_PORT and KUBERNETES_SERVICE_HOST.

The job runs a container to install cilium to your liking, after the job is finished Cilium can be managed/operated like usual.

The above can be combined exchanged with for example Method 3 to host arbitrary configurations externally but render/run them at bootstrap time.

## Known issues

- There are some gotchas when using Talos and Cilium on the Google cloud platform when using internal load balancers.
For more details: [GCP ILB support / support scope local routes to be configured](https://github.com/siderolabs/talos/issues/4109)

- When using Talos `forwardKubeDNSToHost=true` option (which is enabled by default) in combination with cilium `bpf.masquerade=true`.
There is a known issue that causes `CoreDNS` to not work correctly.
As a workaround, configuring `forwardKubeDNSToHost=false` resolves the issue.
For more details see [the discusssion here](https://github.com/siderolabs/talos/pull/9200)

## Other things to know

- After installing Cilium, `cilium connectivity test` might hang and/or fail with errors similar to

    ```Error creating: pods "client-69748f45d8-9b9jg" is forbidden: violates PodSecurity "baseline:latest": non-default capabilities (container "client" must not include "NET_RAW" in securityContext.capabilities.add)```

  This is expected, you can workaround it by adding the `pod-security.kubernetes.io/enforce=privileged` [label on the namespace level]({{< relref "../configuration/pod-security">}}).

- Talos has full kernel module support for eBPF, See:
  - [Cilium System Requirements](https://docs.cilium.io/en/v1.14/operations/system_requirements/)
  - [Talos Kernel Config AMD64](https://github.com/siderolabs/pkgs/blob/main/kernel/build/config-amd64)
  - [Talos Kernel Config ARM64](https://github.com/siderolabs/pkgs/blob/main/kernel/build/config-arm64)
