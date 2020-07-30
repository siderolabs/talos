```yaml
# Config v1alpha2
#
# The goal in this proposal is to outline the v1alpha2 config manifests and to
# detail how multi-doc YAML enables API and config driven management of Talos
#  without introducing accidental complexity.
#
# One benefit of the multi-doc approach is that we can iterate on finely scoped
# configuration changes. If there is a fundamental design flaw in a manifest, we
# can safely release a new version, and do so gradually.
#
# Over time we can start to align manifest kinds with APIs, thereby allowing us
# users to manage Talos via the API, or by configuration. This also opens up
# opportunities for Talos to become more dynamic. We can make decisions based
# on the presence/absence of a manifest kind, and, in some cases, either
# generate or retrieve from the Talos control plane.
#
# In this version, all manifests will be persisted locally, and never updated
# from the `talos.config` endpoint. An API for config management will be
# introduced that allows for updating the config. We will introduce a new
# service, configd, that will implement the API. The API will be implemented
# such that internal services will get their configuration using the API.
#
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
    endpoint: https://1.2.3.4
    token: 5dt69c.npg6duv71zwqhzbg
    ca:
      crt: ""
  talos:
    token: 5dt69c.npg6duv71zwqhzbg
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
spec: {}
---
# Kubelet contains configuration options for the kubelet service. In the absence
# of this manifest, a node will pull this manifest from the Talos control plane.
#
# Required: True
# Dynamic: true
kind: kubelet
version: v1alpha1
spec:
  image: docker.io/autonomy/kubelet:latest
  args: {}
---
# Machine contains machine configuration options.
#
# Dynamic: false
# Required: true
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
      cidrs:
        1.2.3.4/8:
          routes: []
  bonds:
    - name: ""
      interfaces: []
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
  disk: /dev/sda
  image: docker.io/autonomy/installer:latest
  zero: false
  args: [] # Overrides the defaults determined by Talos.
---
# Kubernetes contains options for the cluster creation.
#
# Requred: true
# Dynamic: true (Can be submitted with the bootstrap API)
kind: kubernetes
version: v1alpha1
spec:
  name: "test"
  auth:
    token: wlzjyw.bei2zfylhs2by0wd
    pki:
      ca:
        crt: ""
        key: ""
  encryption:
    aescbc: z01mye6j16bspJYtTB/5SFX8j7Ph4JXxM2Xuu4vsBPM=
  network:
    cni: ""
    dns:
      domain: cluster.local
    subnets:
      pods:
        - 10.244.0.0/16
      services:
        - 10.96.0.0/12
  components:
    controlPlane:
      apiServer: {}
      controllerManager: {}
      scheduler: {}
    addOns:
      coreDNS: {}
      kubeProxy: {}
      podCheckpointer: {}
    extraManifests:
      - source: {}
```
