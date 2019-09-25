# Proposal: Machine Configuration in Talos v0.2.0 and Beyond

Author(s): Spencer Smith (@rsmitty)

## Abstract

This proposal will outline how we'll handle the passing of machine configurations in Talos, as we're aiming to simplify the options and keep the user from having to worry about the minutiae of kubeadm and all of the options.

## Background

I think the easiest way to background this is to take a look at the init node machine config that we currently have a template for, since it is our most verbose template with the most options.
When looking it, it's somewhat self-explanatory on what is available to tweak, but it also gives a good starting point to view what is similar between the three types of Talos nodes: init (the first master), control plane (any other masters), and workers.
I've also appended some additional fields that we use for certain platforms like Packet.
Additionally, as some background around naming, we'll be referring to our configs only as "machine configs", since using terms like "userdata" interchangably led users to believe we supported cloud-init, which is not true.

### Init

```yaml
#!talos
version: ""
security:
  os:
    ca:
      crt: "{{ .Certs.OsCert }}"
      key: "{{ .Certs.OsKey }}"
  kubernetes:
    ca:
      crt: "{{ .Certs.K8sCert }}"
      key: "{{ .Certs.K8sKey }}"
    aescbcEncryptionSecret: "{{ .KubeadmTokens.AESCBCEncryptionSecret }}"
services:
  init:
    cni: flannel
  kubeadm:
    initToken: {{ .InitToken }}
    configuration: |
      apiVersion: kubeadm.k8s.io/v1beta1
      kind: InitConfiguration
      bootstrapTokens:
      - token: '{{ .KubeadmTokens.BootstrapToken }}'
        ttl: 0s
      nodeRegistration:
        taints: []
        kubeletExtraArgs:
          node-labels: ""
      ---
      apiVersion: kubeadm.k8s.io/v1beta1
      kind: ClusterConfiguration
      clusterName: {{ .ClusterName }}
      kubernetesVersion: {{ .KubernetesVersion }}
      controlPlaneEndpoint: {{ .IP }}:443
      apiServer:
        certSANs: [ {{ range $i,$ip := .MasterIPs }}{{if $i}},{{end}}"{{$ip}}"{{end}}, "127.0.0.1" ]
        extraArgs:
          runtime-config: settings.k8s.io/v1alpha1=true
          feature-gates: ExperimentalCriticalPodAnnotation=true
      controllerManager:
        extraArgs:
          terminated-pod-gc-threshold: '100'
          feature-gates: ExperimentalCriticalPodAnnotation=true
      scheduler:
        extraArgs:
          feature-gates: ExperimentalCriticalPodAnnotation=true
      networking:
        dnsDomain: {{ .ServiceDomain }}
        podSubnet: {{ index .PodNet 0 }}
        serviceSubnet: {{ index .ServiceNet 0 }}
      ---
      apiVersion: kubelet.config.k8s.io/v1beta1
      kind: KubeletConfiguration
      featureGates:
        ExperimentalCriticalPodAnnotation: true
      ---
      apiVersion: kubeproxy.config.k8s.io/v1alpha1
      kind: KubeProxyConfiguration
      mode: ipvs
      ipvs:
        scheduler: lc
  trustd:
    token: '{{ .TrustdInfo.Token }}'
    endpoints: [ {{ .Endpoints }} ]
    certSANs: [ "{{ .IP }}", "127.0.0.1" ]
networking:
  os:
    devices:
      - interface: lo
        cidr: x.x.x.x/32
      - interface: eth0
        dhcp: true
install:
  wipe: false
  force: true
  boot:
    device: /dev/sda
    size: 1024000000
    kernel: http://139.178.69.21:8080/assets/talos/release/v0.1.0-beta.0/vmlinuz
    initramfs: http://139.178.69.21:8080/assets/talos/release/v0.1.0-beta.0/initramfs.xz
  root:
    device: /dev/sda
    size: 2048000000
    rootfs: http://139.178.69.21:8080/assets/talos/release/v0.1.0-beta.0/rootfs.tar.gz
  data:
    device: /dev/sda
    size: 4096000000
```

## Proposal

There are quite a few fields that are similar among all node types, while the main difference between them is the kubeadm templates.
Given all the proper info, we should be able to generate the kubeadm configuration on the fly depending on node type and abstract the need to know all the different options away.
I'm proposing a machine configuration that looks like this:

```yaml
#!talos
## We'll implement a versioning strategy and
## respect it when parsing the config
version: "v0.1.0"

## All configs that are specific to this machine only
## go in machine key
machine:
  type: init, controlplane, worker
  kubelet:
    image: gcr.io/google-containers/hyperkube:v1.15.2
    extraArgs: {}
  network:
    hostname: "wtf.m8.cluster"
    interfaces:
      ##MAC AND DEVICE ARE MUTUALLY EXCLUSIVE
      - mac: aa:bb:cc:ee:ff:00
        addresses:
          - "10.254.10.11/24"
          - "192.168.251.11/24"
          - "2001:db8:1000::54/64"
        mtu: 1450
        type: static/dhcp
      - device: eth1
        addresses:
          - "55.44.11.23/29"
        ## An example of how we might provide routes for a given interface
        routes:
          - via: 192.168.24.1
            prefix: 0.0.0.0/0
            gateway: true
            metric: 100
          - via: 2001:db8::0:2bd9
            prefix: 2001:db8::10:0/64
            gateway: false
            metric: 10
      - device: bond0
        subdevices:
          - eth0
          - eth1
  ## This is currently just an idea of how another talos "service"
  ## might get configured for the machine
  ntp:
    servers:
      - server1
  ca:
    crt: "{{ .Certs.OsCert }}"
    key: "{{ .Certs.OsKey }}"
  token: abc.1234
  ## We've removed the size of the "data" partition and renamed it to "ephemeral". Also removed the root partition
  install:
    wipe: false
    force: true
    boot:
      device: /dev/sda
      size: 1024000000
      kernel: https://github.com/talos-systems/talos/releases/download/v0.2.0-alpha.3/vmlinuz
      initramfs: https://github.com/talos-systems/talos/releases/download/v0.2.0-alpha.3/initramfs.xz
    ephemeral:
      device: /dev/sda

## All cluster-wide configs go here. These are generally things that get
## injected to kubeadm, or affect the behavior of the kubernetes control
## plane as a whole.
cluster:
  clusterName: {{ .ClusterName }}
  controlPlane:
    ips:
      - x.x.x.x
      - y.y.y.y
      - z.z.z.z
    apiServer:
      image: gcr.io/google-containers/hyperkube:v1.15.2
      extraArgs: {}
    controllerManager:
      image:
      extraArgs: {}
    scheduler:
      image:
      extraArgs: {}
    etcd:
      image:
    network:
      dnsDomain: {{ .ServiceDomain }}
      podSubnet: {{ index .PodNet 0 }}
      serviceSubnet: {{ index .ServiceNet 0 }}
    ca:
      crt: "{{ .Certs.K8sCert }}"
      key: "{{ .Certs.K8sKey }}"
  token: abc.1234
```

I'm hoping that most kubernetes-specific knobs that need to be tweaked can be done with extra args.
The machine and cluster top-level keys are new and correspond to things that need to be configured per-machine, as well as things that need to be set at the cluster level.

## Rationale

The advantage of this new machine config will mostly center around end-user usability and an increased ability to reason about the configurations supplied to Talos nodes.
Some discussion among maintainers have generally been that there are too many options that need to be passed into Talos for each environment and that we would do better to minimize the amount of information needed to get a cluster up and running.
This also has the added benefit of ensuring that clusters are more supportable because we know exactly what is deployed and their configuration is a known constant.

As far as downsides, we may certainly lose some of the customization available in kubeadm.
There is no way to anticipate what options any and all users will need, so we will likely have to handle requests to add configs back in for different kubeadm features.
We may be able to mitigate some of this as well by allowing users to supply their own kubeadm configuration with the warning that they need to be pretty familiar with all of the kubeadm options and that it's not something we normally recommend.

## Compatibility

The move towards having less options available will likely cause some compatibility issues between versions of Talos.
We do, however, already have a `version` field in our machine configuration that we can use to support the current implementation and this future one for some amount of time.
Going forward, we'll support `n-1` versions of machine configuration.

## Implementation

Some bullet points that come to mind on implementation:

- Determine machine config versioning and update our current implementation to work only in the existence of an empty version string.
- Create go templates for new config above and ensure these get created when the version string matches.
- Refactor kubeadm service to generate its config using only the input provided with the new machine config.
- Refactor trustd service to generate its config using only the input provided with the new machine config.

## Open issues (if applicable)

N/A
