---
description: KubeProxyConfig deploys Flannel CNI to the cluster.
title: KubeProxyConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: KubeProxyConfig
image: registry.k8s.io/kube-proxy:v1.37.0-alpha.3 # The container image used in the kube-proxy manifest.
mode: nftables # description: |
# Provide configuration for the kube-proxy.
config:
    bindAddressHardFail: true
# Configure the kube-proxy resources.
resources:
    # Requests configures the reserved cpu/memory resources.
    requests:
        cpu: 100m
        memory: 50Mi

    # # Limits configures the maximum cpu/memory limits a pod can use.

    # # resources limits.
    # limits:
    #     cpu: 2
    #     memory: 2500Mi
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`enabled` |bool |Enable or disable kube-proxy deployment on cluster bootstrap.<br><br>Default is enabled.  | |
|`image` |string |The container image used in the kube-proxy manifest.  | |
|`mode` |string |description: |<br>    Proxy mode of kube-proxy.<br><br>   The default value is 'nftables'.<br>   It is not recommended to use any other value.<br> values:<br>   - iptables<br>   - ipvs<br>   - nftables<br>  | |
|`config` |Unstructured |Provide configuration for the kube-proxy.<br><br>There is no need  to specify kind and apiVersion fields (they will be set automatically),<br>but the rest of the configuration should be provided as is.<br><br>See https://kubernetes.io/docs/reference/config-api/kube-proxy-config.v1alpha1/ for the details of the configuration schema.  | |
|`extraArgs` |Args |Extra arguments to supply to kube-proxy.<br><br>Please note that kube-proxy is configured with a configuration file,<br>so most flags have no effect.  | |
|`resources` |<a href="#KubeProxyConfig.resources">ResourcesConfig</a> |Configure the kube-proxy resources.  | |




## resources {#KubeProxyConfig.resources}

ResourcesConfig represents the pod resources.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`requests` |Unstructured |Requests configures the reserved cpu/memory resources. <details><summary>Show example(s)</summary>resources requests.:{{< highlight yaml >}}
requests:
    cpu: 1
    memory: 1Gi
{{< /highlight >}}</details> | |
|`limits` |Unstructured |Limits configures the maximum cpu/memory limits a pod can use. <details><summary>Show example(s)</summary>resources limits.:{{< highlight yaml >}}
limits:
    cpu: 2
    memory: 2500Mi
{{< /highlight >}}</details> | |








