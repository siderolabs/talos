---
title: v1alpha1 Reference
---

Talos User Data is responsible for the host and Kubernetes configuration, and it is independent of other cloud init data.

## Differences from v0

The main driver in introducing a new configuration file format is to reduce the complexity and make it more approachable.
The redesign proposal can be found [here](https://github.com/talos-systems/talos/blob/master/docs/proposals/20190708-MachineConfig.md).
The major change between these two versions is the introduction of `machine` and `cluster` configuration objects.
Machine configuration data deals with the configuration of the host itself whereas cluster configuration data deals with the configuration of the cluster on top of Talos ( ex, Kubernetes configuration ).

## Version

`Version` represents the Talos configuration version.

This denotes what the schema of the configuration file is.

```yaml
version: v1alpha1
```

## Machine Configuration

```yaml
machine:
  type: string
  token: string
  env: (optional)
    key: value
  ca:
    crt: string ( base64 encoded certificate )
    key: string ( base64 encoded key )
  kubelet: (optional)
    image: string
    extraArgs: []string
  network: (optional)
    hostname: string
    nameservers: []string
    interfaces:
      - interface: string
        cidr: string
        dhcp: bool
        ignore: bool
  install: (optional)
    disk: string
    extraKernelArgs: []string
    image: string
    bootloader: bool
    wipe: bool
    force: bool
```

### machine.type

`type` defines the type/role of a node.

Acceptable values are: -`init` -`controlplane` -`worker`

#### Init

Init node type designates the first control plane node to come up.
You can think of it like a bootstrap node.
This node will perform the initial steps to bootstrap the cluster -- generation of TLS assets, starting of the control plane, etc.

#### Control Plane

Control Plane node type designates the node as a control plane member.
This means it will host etcd along with the Kubernetes master components such as API Server, Controller Manager, Scheduler.

#### Worker

Worker node type designates the node as a worker node.
This means it will be an available compute node for scheduling workloads.

### machine.env

`env` is used to set any environment variables for the node.
These variables get set at the node level and get passed in to each service as environment variables.
The only supported environment variables are:

- `GRPC_GO_LOG_VERBOSITY_LEVEL`
- `GRPC_GO_LOG_SEVERITY_LEVEL`
- `http_proxy`
- `https_proxy`
- `no_proxy`

### machine.token

`token` is used for authentication to `trustd` to confirm the node's identity.

### machine.kubelet

`kubelet` is used to provide some additional options to the kubelet.

#### machine.kubelet.image

`image` is used to supply a hyperkube image location.

#### machine.kubelet.extraArgs

`extraArgs` is used to supply kubelet with additional startup command line arguments.

### machine.ca

`ca` handles the certificate configuration for Talos components (osd, trustd, etc.).

#### machine.ca.crt

`crt` provides the CA Certificate for OSD.

#### machine.ca.key

`crt` provides the CA Certificate Key for OSD.

### machine.network

`network` defines the host network configuration.

#### machine.network.hostname

`hostname` can be used to statically set the hostname for the host.

#### machine.network.nameservers

`nameservers` can be used to statically set the nameservers for the host.

#### machine.network.interfaces

`interfaces` is used to define the network interface configuration.
By default all network interfaces will attempt a DHCP discovery.
This can be further tuned through this configuration parameter.

##### machine.network.interfaces.interface

This is the interface name that should be configured.

##### machine.network.interfaces.cidr

`cidr` is used to specify a static IP address to the interface.
This should be in proper CIDR notation ( `192.168.2.5/24` ).

> Note: This option is mutually exclusive with DHCP.

##### machine.network.interfaces.dhcp

`dhcp` is used to specify that this device should be configured via DHCP.

The following DHCP options are supported:

- `OptionClasslessStaticRoute`
- `OptionDomainNameServer`
- `OptionDNSDomainSearchList`
- `OptionHostName`

> Note: This option is mutually exclusive with CIDR.

##### machine.network.interfaces.ignore

`ignore` is used to exclude a specific interface from configuration.
This parameter is optional.

##### machine.network.interfaces.routes

`routes` is used to specify static routes that may be necessary.
This parameter is optional.

Routes can be repeated and includes a `Network` and `Gateway` field.

### machine.install

`install` provides the details necessary to install the Talos image to disk.
This is typically only used in bare metal setups.

#### machine.install.disk

`disk` is the device name to use for the `/boot` partition and `/var` partitions.
This should be specified as the unpartitioned block device.

#### machine.install.extraDevices

`extraDevices` contains additional devices that should be formatted and partitioned.

#### machine.install.extraKernelArgs

`extraKernelArgs` contain additional kernel arguments to be appended to the bootloader.

#### machine.install.image

`image` is a url to a Talos installer image.

#### machine.install.bootloader

`bootloader` denotes if the bootloader should be installed to teh device.

#### machine.install.wipe

`wipe` denotes if the disk should have zeros written to it before partitioning.

#### machine.install.force

`force` will ignore any existing partitions on the device.

## Cluster Configuration

```yaml
cluster:
  controlPlane:
    ips: []string
  clusterName: string
  network:
    dnsDomain: string
    podSubnets: []string
    serviceSubnets: []string
  token: string
  ca:
    crt: string
    key: string
  apiServer:
    image: (optional) string
    extraArgs: map[string]string
    certSANs: []string
  controllerManager: (optional)
    image: string
    extraArgs: map[string]string
  scheduler: (optional)
    image: string
    extraArgs: map[string]string
  etcd: (optional)
    image: string
```

### cluster.controlPlane

#### cluster.controlPlane.endpoint

`endpoint` defines the address for kubernetes ( load balancer or DNS name ).

#### cluster.controlPlane.ips

`ips` lists the trustd endpoints.
This should be a list of all the control plane addresses.

### cluster.clusterName

`clusterName` is the name of the cluster.

### cluster.network

### cluster.network.dnsDomain

`dnsDomain` is the dns domain of the cluster.

### cluster.network.podSubnets

`podSubnets` is a list of the subnets that Kubernetes should allocate from for CNI.

### cluster.network.serviceSubnets

`serviceSubnets` is a list of the subnets that Kubernetes should allocate service addresses from.

### cluster.token

`token` is the kubeadm bootstrap token used to authenticate additional kubernetes nodes to the cluster.

### cluster.ca

`ca` represents the ca certificate and key pair for Kubernetes use.

### cluster.ca.crt

### cluster.ca.key

### cluster.apiServer

### cluster.apiServer.image

`image` defines the container image the Kubernetes API server will use.

### cluster.apiServer.extraArgs

`extraArgs` provides additional arguments to the Kubernetes API server.

### cluster.apiServer.certSANs

`certSANs` are a list of IP addresses that should be added to the API server certificate.

### cluster.controllerManager

### cluster.controllerManager.image

`image` defines the container image the Kubernetes API server will use.

### cluster.controllerManager.extraArgs

`extraArgs` provides additional arguments to the Kubernetes API server.

### cluster.scheduler

### cluster.scheduler.image

`image` defines the container image the Kubernetes API server will use.

### cluster.scheduler.extraArgs

`extraArgs` provides additional arguments to the Kubernetes API server.

### cluster.etcd

### cluster.etcd.image

`image` defines the container image the Kubernetes API server will use.
