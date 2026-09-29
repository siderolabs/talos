---
description: KubeNodeConfig configures Kubernetes node.
title: KubeNodeConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: KubeNodeConfig
registerWithFQDN: true # The `registerWithFQDN` field is used to force kubelet to use the node FQDN for registration.
# The `nodeIP` field is used to configure `--node-ip` flag for the kubelet.
nodeIP:
    # The `validSubnets` field configures the networks to pick kubelet node IP from.
    validSubnets:
        - 10.0.0.0/8
        - '!10.0.0.3/32'
        - fdc7::/16
# Configures the node labels for the machine.
labels:
    examplelabel: examplevalue
# Configures the node annotations for the machine.
annotations:
    customer.io/rack: r13a25
# Configures the node taints for the machine. Effect is optional.
taints:
    exampletaint: examplevalue:NoSchedule
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`skipNodeRegistration` |bool |The `skipNodeRegistration` is used to run the kubelet without registering with the apiserver.<br>This runs kubelet as standalone and only runs static pods.<br>When this is set to true, other fields in this document are ignored.  |`true`<br />`yes`<br />`false`<br />`no`<br /> |
|`registerWithFQDN` |bool |The `registerWithFQDN` field is used to force kubelet to use the node FQDN for registration.<br>This is required in clouds like AWS.  |`true`<br />`yes`<br />`false`<br />`no`<br /> |
|`nodeIP` |<a href="#KubeNodeConfig.nodeIP">NodeIPConfig</a> |The `nodeIP` field is used to configure `--node-ip` flag for the kubelet.<br>This field should be set when a node has multiple addresses to choose from.  | |
|`labels` |map[string]string |Configures the node labels for the machine.<br><br>Note: In the default Kubernetes configuration, worker nodes are restricted to set<br>labels with some prefixes (see [NodeRestriction](https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/#noderestriction) admission plugin).  | |
|`annotations` |map[string]string |Configures the node annotations for the machine.  | |
|`taints` |map[string]string |Configures the node taints for the machine. Effect is optional.<br><br>Note: In the default Kubernetes configuration, worker nodes are not allowed to<br>modify the taints (see [NodeRestriction](https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/#noderestriction) admission plugin).  | |




## nodeIP {#KubeNodeConfig.nodeIP}

NodeIPConfig represents the node IP configuration.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`validSubnets` |[]string |The `validSubnets` field configures the networks to pick kubelet node IP from.<br>For dual stack configuration, there should be two subnets: one for IPv4, another for IPv6.<br>IPs can be excluded from the list by using negative match with `!`, e.g `!10.0.0.0/8`.<br>Negative subnet matches should be specified last to filter out IPs picked by positive matches.<br>If not specified, node IP is picked based on cluster podCIDRs: IPv4/IPv6 address or both.  | |








