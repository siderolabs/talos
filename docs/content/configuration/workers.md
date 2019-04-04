---
title: "Workers"
date: 2018-10-29T19:40:55-07:00
draft: false
weight: 30
menu:
  main:
    parent: 'configuration'
    weight: 30
---

Configuring the worker nodes is much more simple in comparison to configuring the master nodes.
Using the `trustd` API, worker nodes submit a `CSR`, and, if authenticated, receive a valid `osd` certificate.
Similarly, using a `kubeadm` token, the node joins an existing cluster.

We need to specify:

- the `osd` public certificate
- `trustd` credentials and endpoints
- and a `kubeadm` `JoinConfiguration`

```yaml
version: ""
...
services:
  kubeadm:
    configuration: |
      apiVersion: kubeadm.k8s.io/v1alpha3
      kind: JoinConfiguration
      ...
  trustd:
    username: <username>
    password: <password>
    endpoints:
    - <master-1>
    ...
    - <master-n>
```

> See the official [documentation](https://kubernetes.io/docs/reference/setup-tools/kubeadm/kubeadm-join/) for the options available in `JoinConfiguration`.
