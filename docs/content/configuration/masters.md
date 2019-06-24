---
title: "Masters"
date: 2018-10-29T19:40:55-07:00
draft: false
weight: 20
menu:
  docs:
    parent: 'configuration'
    weight: 20
---

Configuring master nodes in a Talos Kubernetes cluster is a two part process:

- configuring the Talos specific options
- and configuring the Kubernetes specific options

To get started, create a YAML file we will use in the following steps:

```bash
touch <node-name>.yaml
```

## Configuring Talos

### Injecting the Talos PKI

Using `osctl`, and our output from the `osd` configuration [documentation]({{< ref "osd.md" >}}), inject the generated PKI into the configuration file:

```bash
osctl inject os --crt <organization>.crt --key <organization>.key <node-name>.yaml
```

You should see the following fields populated:

```yaml
security:
  os:
    ca:
      crt: <base 64 encoded root public certificate>
      key: <base 64 encoded root private key>
  ...
```

This process only needs to be performed on you initial node's configuration file.

### Configuring `trustd`

Each master node participates as a Root of Trust in the cluster.
The responsibilities of `trustd` include:

- certificate as a service
- and Kubernetes PKI distribution amongst master nodes

The auth done between `trustd` and a client is, for now, a simple username and password combination.
Having these credentials gives a client the power to request a certifcate that identifies itself.
In the `<node-name>.yaml`, add the follwing:

```yaml
security:
...
services:
  ...
  trustd:
    username: '<username>'
    password: '<password>'
  ...
```

## Configuring Kubernetes

### Generating the Root CA

To create the root CA for the Kubernetes cluster, run:

```bash
osctl gen ca --rsa --hours <hours> --organization <kubernetes-organization>
```

{{% note %}}The `--rsa` flag is required for the generation of the Kubernetes CA. {{% /note %}}

### Injecting the Kubernetes PKI

Using `osctl`, inject the generated PKI into the configuration file:

```bash
osctl inject kubernetes --crt <kubernetes-organization>.crt --key <kubernetes-organization>.key <node-name>.yaml
```

You should see the following fields populated:

```yaml
security:
  ...
  kubernetes:
    ca:
      crt: <base 64 encoded root public certificate>
      key: <base 64 encoded root private key>
  ...
```

### Configuring Kubeadm

The configuration of the `kubeadm` service is done in two parts:

- supplying the Talos specific options
- supplying the `kubeadm` `InitConfiguration`

#### Talos Specific Options

```yaml
services:
  ...
  kubeadm:
    init:
      cni: <flannel|calico>
  ...
```

#### Kubeadm Specific Options

```yaml
services:
  ...
  kubeadm:
    ...
    configuration: |
      apiVersion: kubeadm.k8s.io/v1alpha3
      kind: InitConfiguration
      ...
  ...
```

> See the official [documentation](https://kubernetes.io/docs/reference/setup-tools/kubeadm/kubeadm-init/) for the options available in `InitConfiguration`.

In the end you should have something that looks similar to the following:

```yaml
version: ""
security:
  os:
    ca:
      crt: <base 64 encoded root public certificate>
      key: <base 64 encoded root private key>
  kubernetes:
    ca:
      crt: <base 64 encoded root public certificate>
      key: <base 64 encoded root private key>
services:
  init:
    cni: <flannel|calico>
  kubeadm:
    configuration: |
      apiVersion: kubeadm.k8s.io/v1beta1
      kind: InitConfiguration
      apiEndpoint:
        advertiseAddress: <master ip>
        bindPort: 6443
      apiVersion: kubeadm.k8s.io/v1beta1
      bootstrapTokens:
      - token: '<kubeadm token>'
        ttl: 0s
      ---
      apiVersion: kubeadm.k8s.io/v1beta1
      kind: ClusterConfiguration
      controlPlaneEndpoint: <master ip>:443
      networking:
        dnsDomain: cluster.local
        podSubnet: <pod subnet>
        serviceSubnet: <service subnet>
  trustd:
    username: '<username>'
    password: '<password>'
```
