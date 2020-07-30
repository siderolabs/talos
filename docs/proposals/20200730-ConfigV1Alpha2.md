# Proposal: Config v1alpha2

Author(s): Andrew Rynhard

## Abstract

The goal in this proposal is to outline the v1alpha2 config manifests and to detail how multi-doc YAML enables API and config driven management of Talos without introducing accidental complexity.

## Background

The `v1alpha1` config has limitations, and layout flaws that we would like to fix.
In our experience with `v1alpha1` we found that a single configuration file made it difficult to evlove configuration options without breaking things.

## Proposal

```yaml
---
# Controlplane contains configuration options for the Talos and Kubernetes
# control planes. The absence of this manifest enables automatic retrieval from
# the Talos control plane.
#
# Dynamic: true
# Required: false
kind: controlplane
version: v1alpha1
spec:
  talos:
    ca:
      key: ""
---
# Join contains the parameters required to join the Talos and Kubernetes control
# planes.
#
# Required: true
# Dynamic: false
kind: join
version: v1alpha1
spec:
  kubernetes:
    endpoint: ""
    token: ""
    ca:
      crt: ""
  talos:
    token: ""
    ca: # Client needs this to validate trustd connection.
      crt: ""
---
# Etcd contains configuration options for the etcd service. The presence of
# this manifest enables the etcd service.
#
# Required: true
# Dynamic: true
kind: etcd
version: v1alpha1
spec:
  image: ""
  args: {}
---
# Kubelet contains configuration options for the kubelet service. In the absence
# of this manifest, a node will pull this manifest from the Talos control plane.
#
# Required: true
# Dynamic: true
kind: kubelet
version: v1alpha1
spec:
  image: ""
  args: {}
---
# Machine contains machine configuration options.
#
# Required: true
# Dynamic: false
kind: machine
version: v1alpha1
spec:
  identity:
    sans: []
  bootloader:
    cmdline: []
  kernel:
    parameters: {}
  storage: {}
  time:
    ntp:
      servers: {}
  env: {}
  registries: {}
---
# Network contains network configuration options.
#
# Required: false
# Dynamic: false
kind: network
version: v1alpha1
spec:
  interfaces:
    - name: ""
      dhcp: bool
      cidrs: []
  routes:
    - destination: ""
      gateway: ""
  bonds:
    - name: ""
      interfaces: []
      cidrs: []
      mode: ""
---
# Install contains installation options.
#
# Proposal: Use this for upgrades. The cached local version will be updated
#           upon an upgrade.
#
# Required: false
# Dynamic: false
kind: install
version: v1alpha1
spec:
  disk: ""
  image: ""
  zero: bool
  args: [] # Overrides the defaults determined by Talos.
---
# Kubernetes contains options for the cluster creation.
#
# Required: true
# Dynamic: true (Can be submitted with the bootstrap API)
kind: kubernetes
version: v1alpha1
spec:
  name: ""
  auth:
    token: ""
    pki:
      ca:
        crt: ""
        key: ""
  encryption:
    aescbc: ""
  network:
    cni: ""
    dns:
      domain: ""
    subnets:
      pods: []
      services: []
  components:
    controlPlane:
      apiServer:
        image: ""
        args: []
      controllerManager:
        image: ""
        args: []
      scheduler:
        image: ""
        args: []
    addOns:
      coreDNS:
        enabled: bool
        image: ""
        args: []
      kubeProxy:
        enabled: bool
        image: ""
        args: []
    extraManifests:
      - source: {}
```

## Rationale

One benefit of the multi-doc approach is that we can iterate on finely scoped configuration changes.
If there is a fundamental design flaw in a manifest, we can safely release a new version, and do so gradually.
Additionally, multi-doc allows for different services to have a manigest.
If this becomes the case we could create a service such that internal services will get their configuration using this new service's API.

Over time we can start to align manifest kinds with APIs, thereby allowing us users to manage Talos via the API, or by configuration.
This also opens up opportunities for Talos to become more dynamic. We can make decisions based on the presence/absence of a manifest kind, and, in some cases, either generate or retrieve from the Talos control plane.

In this version, all manifests will be persisted locally, and never updated from the `talos.config` endpoint.
An API for config management will be introduced that allows for updating the config.

## Compatibility

We can introduce this without any changes to the way that `v1alpha1` behaves.
However, it is an opportunity for us to omit options that are in `v1alpha1`.
In other words, there is the possibility that there will not be feature parity.

## Implementation

- preparation
  - change provider concrete types to interfaces
  - proposal
- implementation
- generate functionality
- documentation

## Open issues (if applicable)

[A discussion of issues relating to this proposal for which the author does not know the solution.][this section may be omitted if there are none.]
