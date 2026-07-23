---
description: KubeletConfig configures kubelet component on the node.
title: KubeletConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: KubeletConfig
image: ghcr.io/siderolabs/kubelet:v1.37.0-beta.0 # The container image used to run the kubelet component.
# Provide extra configuration for the kubelet.
config:
    serverTLSBootstrap: true
# Extra command line arguments to supply to the kubelet.
extraArgs:
    feature-gates: AllBeta=true
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`image` |string |The container image used to run the kubelet component.<br><br>The image reference should contain the tag, even if it is pinned by digest.  | |
|`config` |Unstructured |Provide extra configuration for the kubelet.<br><br>There is no need to specify kind and apiVersion fields (they will be set automatically),<br>but the rest of the configuration should be provided as is.<br><br>See https://kubernetes.io/docs/reference/config-api/kubelet-config.v1beta1/ for the details of the configuration schema.  | |
|`extraArgs` |Args |Extra command line arguments to supply to the kubelet.<br><br>It is preferable to use `config` field to provide configuration overrides.  | |
|`clusterDNS` |[]string |The `ClusterDNS` field is an optional reference to an alternative kubelet clusterDNS ip list.  | |
|`defaultRuntimeSeccompProfileEnabled` |bool |Enable container runtime default Seccomp profile.  |`true`<br />`yes`<br />`false`<br />`no`<br /> |






