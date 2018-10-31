---
title: "Workers"
date: 2018-10-29T19:40:55-07:00
draft: false
menu:
  main:
    parent: 'configuration'
    weight: 20
---

```yaml
version: ""
security:
  os:
    ca:
      crt: ${BASE64_ENCODED_PEM_FORMATTED_PUBLIC_X509}
networking:
  os: {}
  kubernetes: {}
services:
  kubeadm:
    containerRuntime: docker
    configuration: |
      apiVersion: kubeadm.k8s.io/v1alpha2
      kind: NodeConfiguration
      token: abcdef.0123456789abcdef
      discoveryTokenAPIServers:
      - ${MASTER_IP}:443
      discoveryTokenCACertHashes:
      - sha256:${CA_CERT_HASH}
  trustd:
    username: example
    password: example
    endpoints:
    - ${MASTER_IP}
```
