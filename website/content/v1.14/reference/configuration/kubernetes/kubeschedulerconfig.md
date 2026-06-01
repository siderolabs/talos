---
description: KubeSchedulerConfig configures kube-scheduler controlplane static pod.
title: KubeSchedulerConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: KubeSchedulerConfig
image: registry.k8s.io/kube-scheduler:v1.36.1 # The container image used to run the kube-scheduler component.
# Provide configuration for the kube-scheduler static pod.
config:
    profiles:
        - plugins:
            score:
                disabled:
                    - name: PodTopologySpread
# Extra command line arguments to supply to the kube-scheduler.
extraArgs:
    feature-gates: AllBeta=true
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`enabled` |bool |By default, kube-scheduler static pod is enabled.<br>Set to false to disable the kube-scheduler (assuming it runs on other controlplane node).  | |
|`image` |string |The container image used to run the kube-scheduler component.<br><br>The image reference should contain the tag, even if it is pinned by digest.  | |
|`config` |Unstructured |Provide configuration for the kube-scheduler static pod.<br><br>There is no need  to specify kind and apiVersion fields (they will be set automatically),<br>but the rest of the configuration should be provided as is.<br><br>See https://kubernetes.io/docs/reference/scheduling/config/ for the details of the configuration schema.  | |
|`extraArgs` |Args |Extra command line arguments to supply to the kube-scheduler.<br><br>It is preferable to use `config` field to provide configuration overrides.  | |
|`env` |map[string]string |The `env` field allows for the addition of environment variables for the kube-scheduler.  | |
|`resources` |<a href="#KubeSchedulerConfig.resources">ResourcesConfig</a> |Configure the kube-scheduler resources.  | |




## resources {#KubeSchedulerConfig.resources}

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








