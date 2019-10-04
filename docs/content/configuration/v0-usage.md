---
title: "v0 Usage"
date: 2019-10-04T17:14:49-07:00
draft: false
weight: 10
menu:
  docs:
    identifier: "v0-usage-configuration"
    parent: 'configuration'
---

Talos enforces a high level of security by using mutual TLS for authentication and authorization.

We recommend that the configuration of Talos be performed by a cluster owner.
A cluster owner should be a person of authority within an organization, perhaps a director, manager, or senior member of a team.
They are responsible for storing the root CA, and distributing the PKI for authorized cluster administrators.

## Generate base configuration

We can generate a basic configuration using `osctl`.
This configuration is enough to get started with, however it can be customized as needed.

```bash
osctl config generate <cluster name> <master ip>[,<master ip>...]
```

This command will generate a yaml config per master node, a worker config, and a talosconfig.

## Example of generated master-1.yaml

```bash
osctl config generate cluster.local 1.2.3.4,2.3.4.5,3.4.5.6
```

```yaml
#!talos
version: ""
security:
  os:
    ca:
      crt: "LS0tLS1CRUdJTiBDRVJUSUZJQ..."
      key: "LS0tLS1CRUdJTiBFQyBQUklWQV..."
  kubernetes:
    ca:
      crt: "LS0tLS1CRUdJTiBDRVJ..."
      key: "LS0tLS1CRUdJTiBSU0E..."
services:
  init:
    cni: flannel
  kubeadm:
    certificateKey: 'mrhjuj5wlhd9v7z9xls3gh88uo'
    configuration: |
      apiVersion: kubeadm.k8s.io/v1beta2
      kind: InitConfiguration
      bootstrapTokens:
      - token: 'itv1vj.c8iznlo3gvbimoea'
        ttl: 0s
      nodeRegistration:
        taints: []
        kubeletExtraArgs:
          node-labels: ""
      ---
      apiVersion: kubeadm.k8s.io/v1beta2
      kind: ClusterConfiguration
      clusterName: cluster.local
      kubernetesVersion: v1.16.0
      controlPlaneEndpoint: "1.2.3.4"
      apiServer:
        certSANs: [ "127.0.0.1","::1","1.2.3.4","2.3.4.5","3.4.5.6" ]
        extraArgs:
          runtime-config: settings.k8s.io/v1alpha1=true
          feature-gates: ""
      controllerManager:
        extraArgs:
          terminated-pod-gc-threshold: '100'
          feature-gates: ""
      scheduler:
        extraArgs:
          feature-gates: ""
      networking:
        dnsDomain: cluster.local
        podSubnet: "10.244.0.0/16"
        serviceSubnet: "10.96.0.0/12"
      ---
      apiVersion: kubelet.config.k8s.io/v1beta1
      kind: KubeletConfiguration
      featureGates: {}
      ---
      apiVersion: kubeproxy.config.k8s.io/v1alpha1
      kind: KubeProxyConfiguration
      mode: ipvs
      ipvs:
        scheduler: lc
  trustd:
    token: '3gs2ja.q6yno1x90m3hb3f5'
    endpoints: [ "1.2.3.4", "2.3.4.5", "3.4.5.6" ]
    certSANs: [ "1.2.3.4", "127.0.0.1", "::1" ]
```

The above configuration can be customized as needed by using the following [reference guide](/docs/configuration/v0-reference/).
