---
title: v0 Reference
date: 2019-06-21T19:40:55-07:00
draft: false
weight: 20
menu:
  docs:
    parent: 'configuration'
---

Talos User Data is responsible for the host and Kubernetes configuration, and it is independent of other cloud config / cloud init data.

## Version

``Version`` represents the Talos userdata configuration version.

This denotes what the schema of the configuration file is.

```yaml
version: "0"
```

## Security

``Security`` contains all of the certificate information for Talos.

### OS

``OS`` handles the certificate configuration for Talos components (osd, trustd, etc.).

#### CA

``OS.CA`` contains the certificate/key pair.

```yaml
security:
  os:
    ca:
      crt: <base64 encoded x509 pem certificate>
      key: <base64 encoded x509 pem certificate key>
```

### Kubernetes

Kubernetes handles the certificate configuration for Kubernetes components (api server).

<!-- markdownlint-disable MD024 -->
#### CA

``Kubernetes.CA`` contains the certificate/key pair for the apiserver.

```yaml
security:
  kubernetes:
    ca:
      crt: <base64 encoded x509 pem certificate>
      key: <base64 encoded x509 pem certificate key>
```

#### SA

``Kubernetes.SA`` contains the certificate/key pair for the default service account.
This item is optional.
If it is not provided, a certificate/key pair will be generated.

```yaml
security:
  kubernetes:
    sa:
      crt: <base64 encoded x509 pem certificate>
      key: <base64 encoded x509 pem certificate key>
```

#### FrontProxy

``Kubernetes.FrontProxy`` contains the certificate/key pair for the [Front Proxy](https://kubernetes.io/docs/tasks/access-kubernetes-api/setup-extension-api-server/).
This item is optional.
If it is not provided, a certificate/key pair will be generated.

```yaml
security:
  kubernetes:
    frontproxy:
      crt: <base64 encoded x509 pem certificate>
      key: <base64 encoded x509 pem certificate key>
```

#### Etcd

``Kubernetes.Etcd`` contains the certificate/key pair for [etcd](https://kubernetes.io/docs/concepts/overview/components/#etcd).
This item is optional.
If it is not provided, a certificate/key pair will be generated.

```yaml
security:
  kubernetes:
    etcd:
      crt: <base64 encoded x509 pem certificate>
      key: <base64 encoded x509 pem certificate key>
```

## Networking

``Networking`` allows for the customization of the host networking.

**Note** Bonding is currently not supported.

<!-- markdownlint-disable MD024 -->
### OS

``OS`` contains a list of host networking devices and their respective configurations.

#### Devices

```yaml
networking:
os:
  devices:
  - interface: eth0
    cidr: <ip/mask>
    dhcp: bool
    ignore: bool
    routes:
      - network: <ip/mask>
        gateway: <ip>
```

##### Interface

This is the interface name that should be configured.

##### CIDR

``CIDR`` is used to specify a static IP address to the interface.

> Note: This option is mutually exclusive with DHCP.

##### DHCP

``DHCP`` is used to specify that this device should be configured via DHCP.

The following DHCP options are supported:

- `OptionHostName`
- `OptionClasslessStaticRouteOption`
- `OptionDNSDomainSearchList`
- `OptionNTPServers`

> Note: This option is mutually exclusive with CIDR.

##### Ignore

``Ignore`` is used to exclude a specific interface from configuration.
This parameter is optional.

##### Routes

``Routes`` is used to specify static routes that may be necessary.
This parameter is optional.

## Services

### Init

``Init`` allows for the customization of the CNI plugin.
This translates to additional host mounts.

```yaml
services:
  init:
    cni: [flannel|calico]
```

> Note: This option will eventually be deprecated.

### Kubelet

#### ExtraMounts

``Kubelet.ExtraMounts`` allows you to specify additional host mounts that should be presented to kubelet.

```yaml
services:
  kubelet:
    extraMounts:
      - < opencontainers/runtime-spec/mounts >
```

### Kubeadm

#### Configuration

``Kubeadm.Configuration`` contains the various kubeadm configs as a yaml block of yaml configs.

```yaml
services:
  kubeadm:
    configuration: |
      apiVersion: kubeadm.k8s.io/v1beta1
      kind: InitConfiguration
      ...
      ---
      apiVersion: kubeadm.k8s.io/v1beta1
      kind: ClusterConfiguration
      ...
      ---
      apiVersion: kubelet.config.k8s.io/v1beta1
      kind: KubeletConfiguration
      ...
      ---
      apiVersion: kubeproxy.config.k8s.io/v1alpha1
      kind: KubeProxyConfiguration
      ...
```

#### ExtraArgs

``Kubeadm.extraArgs`` contains an additional list of arguments that can be passed into kubeadm.

```yaml
services:
  kubeadm:
    extraArgs:
      - some arg
      - some arg
      ...
```

#### IgnorePreflightErrors

Kubeadm.Ignorepreflighterrors is a list of Kubeadm preflight errors to ignore.

```yaml
services:
  kubeadm:
    ignorePreflightErrors:
      - Swap
      - SystemVerification
      ...
```

### Trustd

#### Token

Trustd.Token can be used for auth for trustd.

```yaml
services:
  trustd:
    token: a9u3hjikoof.ADa
```

**Note** Token is mutually exclusive from Username and Password.

#### Username

Trustd.Username is part of the username/password combination used for auth for trustd.
The values defined here will be the credentials trustd will use.

```yaml
services:
  trustd:
    username: trusty
```

**Note** Username/Password mutually exclusive from Token.

#### Password

Trustd.Password is part of the username/password combination used for auth for trustd.
The values defined here will be the credentials trustd will use.

```yaml
services:
  trustd:
    password: mypass
```

**Note** Username/Password mutually exclusive from Token.

#### Endpoints

The endpoints denote the other trustd instances.
All trustd instances should be listed here.
These are typically your master nodes.

```yaml
services:
  trustd:
    endpoints:
      - endpoint
```

#### CertSANs

```yaml
services:
  trustd:
    certSANs:
      - san
```

### NTP

#### Server

NTP.Server allows you to customize which NTP server to use.
By default it consumes from pool.ntp.org.

```yaml
services:
  ntp:
    server: <ntp server>
```

## Install

Install is primarily used in bare metal situations.
It defines the disk layout and installation properties.

### Boot

#### Device

The device name to use for the `/boot` partition.
This should be specified as the unpartitioned block device.
If this parameter is omitted, the value of `install.root.device` is used.

```yaml
install:
  boot:
    device: <name of device to use>
```

#### Size

The size of the `/boot` partition in bytes.
If this parameter is omitted, a default value of 512MB will be used.

```yaml
install:
  boot:
    size: <size in bytes>
```

#### Kernel

This parameter can be used to specify a custom kernel to use.
If this parameter is omitted, the most recent Talos release will be used ( fetched from github releases ).

```yaml
install:
  boot:
    kernel: <path or url to vmlinuz>
```

**Note** The asset name **must** be named `vmlinuz`.

#### Initramfs

This parameter can be used to specify a custom initramfs to use.
If this parameter is omitted, the most recent Talos release will be used ( fetched from github releases ).

```yaml
install:
  boot:
    initramfs: <path or url to initramfs.xz>
```

**Note** The asset name **must** be named `initramfs.xz`.

### Ephemeral

<!-- markdownlint-disable MD024 -->
#### Device

``Device`` specifies the device name to use for the `/var` partition.
This should be specified as the unpartitioned block device.
If this parameter is omitted, the value of `install.root.device` is used.

```yaml
install:
  ephemeral:
    device: <name of device to use>
```

<!-- markdownlint-disable MD024 -->
#### Size

``Size`` defines the size of the `/var` partition in bytes.
If this parameter is omitted, a default value of 1GB will be used.
This partition will auto extend to consume the remainder of the unpartitioned space on the disk.

```yaml
install:
  ephemeral:
    size: <size in bytes>
```

### Wipe

``Wipe`` denotes if the disk should be wiped ( zero's written ) before it is partitioned.

```yaml
install:
  wipe: <bool>
```

### Force

``Force`` allows the partitioning to proceed if there is already a filesystem detected.

```yaml
install:
  force: <bool>
```

### ExtraDevices

``ExtraDevices`` allows for the extension of the partitioning scheme on the specified device.
These new partitions will be formatted as `xfs` filesystems.

```yaml
install:
  extraDevices:
    - device: sdb
      partitions:
        - size: 2048000000
          mountpoint: /var/lib/etcd
```

<!-- markdownlint-disable MD024 -->
#### Device

``ExtraDevices.Device`` specifies a device to use for additional host mountpoints.

#### Partitions

<!-- markdownlint-disable MD024 -->
##### Size

``Size`` specifies the size in bytes of the new partition.

##### MountPoint

``Mountpoint`` specifies where the device should be mounted.
