---
description: KubeControllerManagerConfig configures kube-controller-manager controlplane
    static pod.
title: KubeControllerManagerConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: KubeControllerManagerConfig
image: registry.k8s.io/kube-controller-manager:v1.36.2 # The container image used to run the kube-controller-manager component.
# Extra command line arguments to supply to the kube-controller-manager.
extraArgs:
    feature-gates: AllBeta=true
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`enabled` |bool |By default, kube-controller-manager static pod is enabled.<br>Set to false to disable the kube-controller-manager (assuming it runs on other controlplane node).  | |
|`image` |string |The container image used to run the kube-controller-manager component.<br><br>The image reference should contain the tag, even if it is pinned by digest.  | |
|`extraArgs` |Args |Extra command line arguments to supply to the kube-controller-manager.  | |
|`env` |map[string]string |The `env` field allows for the addition of environment variables for the kube-controller-manager.  | |
|`resources` |<a href="#KubeControllerManagerConfig.resources">ResourcesConfig</a> |Configure the kube-controller-manager resources.  | |




## resources {#KubeControllerManagerConfig.resources}

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








