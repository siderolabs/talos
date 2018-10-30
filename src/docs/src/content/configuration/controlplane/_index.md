---
title: "Control Plane"
date: 2018-10-29T19:40:55-07:00
draft: false
---

```yaml
version: ""
security:
  os:
    ca:
      crt: ${BASE64_ENCODED_PEM_FORMATTED_PUBLIC_X509}
      key: ${BASE64_ENCODED_PEM_FORMATTED_PRIVATE_X509}
    identity:
      crt: ${BASE64_ENCODED_PEM_FORMATTED_PUBLIC_X509}
      key: ${BASE64_ENCODED_PEM_FORMATTED_PRIVATE_X509}
  kubernetes:
    ca:
      crt: ${BASE64_ENCODED_PEM_FORMATTED_PUBLIC_X509}
      key: ${BASE64_ENCODED_PEM_FORMATTED_PRIVATE_X509}
networking:
  os: {}
  kubernetes: {}
services:
  kubeadm:
    init:
      type: initial
      etcdMemberName: etcd-1
    containerRuntime: docker
    configuration: |
      apiVersion: kubeadm.k8s.io/v1alpha2
      kind: MasterConfiguration
      clusterName: example
      bootstrapTokens:
      - token: abcdef.0123456789abcdef
        ttl: 0s
      kubeProxy:
        config:
          ipvs:
            scheduler: lc
          mode: ipvs
      networking:
        dnsDomain: cluster.local
        podSubnet: 10.244.0.0/16
        serviceSubnet: 10.96.0.0/12
  trustd:
    username: example
    password: example
```

> You can generate the PKI resources and inject them into the configuration with [osctl]({{< relref "/components/osctl" >}}).
