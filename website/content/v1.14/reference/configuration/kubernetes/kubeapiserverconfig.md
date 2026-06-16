---
description: KubeAPIServerConfig configures kube-apiserver controlplane static pod.
title: KubeAPIServerConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: KubeAPIServerConfig
image: registry.k8s.io/kube-apiserver:v1.36.2 # The container image used to run the kube-apiserver component.
# Extra command line arguments to supply to the kube-apiserver.
extraArgs:
    feature-gates: ServerSideApply=true
    http2-max-streams-per-connection: "32"
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`image` |string |The container image used to run the kube-apiserver component.<br><br>The image reference should contain the tag, even if it is pinned by digest.  | |
|`extraArgs` |Args |Extra command line arguments to supply to the kube-apiserver.  | |
|`env` |map[string]string |The `env` field allows for the addition of environment variables for the kube-apiserver.  | |
|`resources` |<a href="#KubeAPIServerConfig.resources">ResourcesConfig</a> |Configure the kube-apiserver resources.  | |
|`apiPort` |int |The port on which the kube-apiserver will listen for requests.<br><br>Default is 6443.  | |
|`certExtraSANs` |[]string |Provide extra certificate SANs (hostnames, IPs) to add to the kube-apiserver serving certificate.<br><br>Talos automatically adds machine's addresses and hostnames, Kubernetes names, and control plane endpoint<br>derived SANs to the kube-apiserver serving certificate.<br>This field allows for adding additional SANs to the serving certificate.  | |
|`startupProbes` |bool |Enable or disable startup probes for kube-apiserver.<br><br>Default is enabled.  | |




## resources {#KubeAPIServerConfig.resources}

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








