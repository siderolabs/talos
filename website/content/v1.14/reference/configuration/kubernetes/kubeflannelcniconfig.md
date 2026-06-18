---
description: KubeFlannelCNIConfig deploys Flannel CNI to the cluster.
title: KubeFlannelCNIConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: KubeFlannelCNIConfig
backendType: vxlan # Type of the Flannel backend to use.
backendPort: 4789 # UDP port used by Flannel for encapsulating traffic (if the backend type requires encapsulation).
backendMTU: 1420 # Transport MTU to be used for the pod network.
# Extra arguments for 'flanneld'.
extraArgs:
    - --iface-can-reach=192.168.1.1
kubeNetworkPoliciesEnabled: true # Deploys kube-network-policies along with Flannel.
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`backendType` |string |Type of the Flannel backend to use.<br><br>See Flannel documentation for supported backend types.<br>The default value in generated machine configuration is "vxlan".  | |
|`backendPort` |uint16 |UDP port used by Flannel for encapsulating traffic (if the backend type requires encapsulation).<br><br>The default value in generated machine configuration is 4789.  | |
|`backendMTU` |uint32 |Transport MTU to be used for the pod network.<br><br>Flannel will subtract encapsulation overhead from this MTU to calculate<br>the MTU of the pod interface.<br>If not set, the default is auto-detection of MTU by Flannel.<br>If KubeSpan is enabled, and the value is not set, defaults to KubeSpan MTU.  | |
|`backendExtraConfig` |Unstructured |Extra configuration for Flannel backend.<br><br>The content of this field depends on the backend type used.<br>The value of this field will be patched into Flannel configuration 'Backend' section as-is.  | |
|`resources` |<a href="#KubeFlannelCNIConfig.resources">ResourcesConfig</a> |Resources configuration for Flannel main container.  | |
|`extraArgs` |[]string |Extra arguments for 'flanneld'. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
extraArgs:
    - --iface-can-reach=192.168.1.1
{{< /highlight >}}</details> | |
|`kubeNetworkPoliciesEnabled` |bool |Deploys kube-network-policies along with Flannel.<br><br>This enables Kubernetes Network Policies support in the cluster.  | |




## resources {#KubeFlannelCNIConfig.resources}

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








