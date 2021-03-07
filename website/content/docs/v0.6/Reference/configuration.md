---
title: Configuration
description: ""
---

<!-- markdownlint-disable MD024 -->

Package v1alpha1 configuration file contains all the options available for configuring a machine.

We can generate the files using `talosctl`.
This configuration is enough to get started in most cases, however it can be customized as needed.

```bash
talosctl config generate --version v1alpha1 <cluster name> <cluster endpoint>
```

This will generate a machine config for each node type, and a talosconfig.
The following is an example of an `init.yaml`:

```yaml
version: v1alpha1
machine:
  type: init
  token: 5dt69c.npg6duv71zwqhzbg
  ca:
    crt: <base64 encoded Ed25519 certificate>
    key: <base64 encoded Ed25519 key>
  certSANs: []
  kubelet: {}
  network: {}
  install:
    disk: /dev/sda
    image: docker.io/autonomy/installer:latest
    bootloader: true
    wipe: false
    force: false
cluster:
  controlPlane:
    endpoint: https://1.2.3.4
  clusterName: example
  network:
    cni: ""
    dnsDomain: cluster.local
    podSubnets:
      - 10.244.0.0/16
    serviceSubnets:
      - 10.96.0.0/12
  token: wlzjyw.bei2zfylhs2by0wd
  certificateKey: 20d9aafb46d6db4c0958db5b3fc481c8c14fc9b1abd8ac43194f4246b77131be
  aescbcEncryptionSecret: z01mye6j16bspJYtTB/5SFX8j7Ph4JXxM2Xuu4vsBPM=
  ca:
    crt: <base64 encoded RSA certificate>
    key: <base64 encoded RSA key>
  apiServer: {}
  controllerManager: {}
  scheduler: {}
  etcd:
    ca:
      crt: <base64 encoded RSA certificate>
      key: <base64 encoded RSA key>
```

### Config

#### version

Indicates the schema used to decode the contents.

Type: `string`

Valid Values:

- `v1alpha1`

#### debug

Enable verbose logging.

Type: `bool`

Valid Values:

- `true`
- `yes`
- `false`
- `no`

#### persist

Indicates whether to pull the machine config upon every boot.

Type: `bool`

Valid Values:

- `true`
- `yes`
- `false`
- `no`

#### machine

Provides machine specific configuration options.

Type: `MachineConfig`

#### cluster

Provides cluster specific configuration options.

Type: `ClusterConfig`

---

### MachineConfig

#### type

Defines the role of the machine within the cluster.

##### Init

Init node type designates the first control plane node to come up.
You can think of it like a bootstrap node.
This node will perform the initial steps to bootstrap the cluster -- generation of TLS assets, starting of the control plane, etc.

##### Control Plane

Control Plane node type designates the node as a control plane member.
This means it will host etcd along with the Kubernetes master components such as API Server, Controller Manager, Scheduler.

##### Worker

Worker node type designates the node as a worker node.
This means it will be an available compute node for scheduling workloads.

Type: `string`

Valid Values:

- `init`
- `controlplane`
- `join`

#### token

The `token` is used by a machine to join the PKI of the cluster.
Using this token, a machine will create a certificate signing request (CSR), and request a certificate that will be used as its' identity.

Type: `string`

Examples:

```yaml
token: 328hom.uqjzh6jnn2eie9oi
```

> Warning: It is important to ensure that this token is correct since a machine's certificate has a short TTL by default

#### ca

The root certificate authority of the PKI.
It is composed of a base64 encoded `crt` and `key`.

Type: `PEMEncodedCertificateAndKey`

Examples:

```yaml
ca:
  crt: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUJIekNCMHF...
  key: LS0tLS1CRUdJTiBFRDI1NTE5IFBSSVZBVEUgS0VZLS0tLS0KTUM...
```

#### certSANs

Extra certificate subject alternative names for the machine's certificate.
By default, all non-loopback interface IPs are automatically added to the certificate's SANs.

Type: `array`

Examples:

```yaml
certSANs:
  - 10.0.0.10
  - 172.16.0.10
  - 192.168.0.10
```

#### kubelet

Used to provide additional options to the kubelet.

Type: `KubeletConfig`

Examples:

```yaml
kubelet:
  image:
  extraArgs:
    key: value
```

#### network

Used to configure the machine's network.

Type: `NetworkConfig`

Examples:

```yaml
network:
  hostname: worker-1
  interfaces:
  nameservers:
    - 9.8.7.6
    - 8.7.6.5
```

#### disks

Used to partition, format and mount additional disks.
Since the rootfs is read only with the exception of `/var`, mounts are only valid if they are under `/var`.
Note that the partitioning and formating is done only once, if and only if no existing partitions are found.
If `size:` is omitted, the partition is sized to occupy full disk.

Type: `array`

Examples:

```yaml
disks:
  - device: /dev/sdb
    partitions:
      - mountpoint: /var/lib/extra
        size: 10000000000
```

> Note: `size` is in units of bytes.

#### install

Used to provide instructions for bare-metal installations.

Type: `InstallConfig`

Examples:

```yaml
install:
  disk: /dev/sda
  extraKernelArgs:
    - option=value
  image: docker.io/autonomy/installer:latest
  bootloader: true
  wipe: false
  force: false
```

#### files

Allows the addition of user specified files.
The value of `op` can be `create`, `overwrite`, or `append`.
In the case of `create`, `path` must not exist.
In the case of `overwrite`, and `append`, `path` must be a valid file.
If an `op` value of `append` is used, the existing file will be appended.
Note that the file contents are not required to be base64 encoded.

Type: `array`

Examples:

```yaml
files:
  - content: |
      ...
    permissions: 0666
    path: /tmp/file.txt
    op: append
```

> Note: The specified `path` is relative to `/var`.

#### env

The `env` field allows for the addition of environment variables to a machine.
All environment variables are set on the machine in addition to every service.

Type: `Env`

Valid Values:

- `GRPC_GO_LOG_VERBOSITY_LEVEL`
- `GRPC_GO_LOG_SEVERITY_LEVEL`
- `http_proxy`
- `https_proxy`
- `no_proxy`

Examples:

```yaml
env:
  GRPC_GO_LOG_VERBOSITY_LEVEL: "99"
  GRPC_GO_LOG_SEVERITY_LEVEL: info
  https_proxy: http://SERVER:PORT/
```

```yaml
env:
  GRPC_GO_LOG_SEVERITY_LEVEL: error
  https_proxy: https://USERNAME:PASSWORD@SERVER:PORT/
```

```yaml
env:
  https_proxy: http://DOMAIN\\USERNAME:PASSWORD@SERVER:PORT/
```

#### time

Used to configure the machine's time settings.

Type: `TimeConfig`

Examples:

```yaml
time:
  servers:
    - time.cloudflare.com
```

#### sysctls

Used to configure the machine's sysctls.

Type: `map`

Examples:

```yaml
sysctls:
  kernel.domainname: talos.dev
  net.ipv4.ip_forward: "0"
```

#### registries

Used to configure the machine's container image registry mirrors.

Automatically generates matching CRI configuration for registry mirrors.

Section `mirrors` allows to redirect requests for images to non-default registry,
which might be local registry or caching mirror.

Section `config` provides a way to authenticate to the registry with TLS client
identity, provide registry CA, or authentication information.
Authentication information has same meaning with the corresponding field in `.docker/config.json`.

See also matching configuration for [CRI containerd plugin](https://github.com/containerd/cri/blob/master/docs/registry.md).

Type: `RegistriesConfig`

Examples:

```yaml
registries:
  mirrors:
    docker.io:
      endpoints:
        - https://registry-1.docker.io
    '*':
      endpoints:
        - http://some.host:123/
 config:
  "some.host:123":
    tls:
      CA: ... # base64-encoded CA certificate in PEM format
      clientIdentity:
        cert: ...  # base64-encoded client certificate in PEM format
        key: ...  # base64-encoded client key in PEM format
    auth:
      username: ...
      password: ...
      auth: ...
      identityToken: ...

```

---

### ClusterConfig

#### controlPlane

Provides control plane specific configuration options.

Type: `ControlPlaneConfig`

Examples:

```yaml
controlPlane:
  endpoint: https://1.2.3.4
  localAPIServerPort: 443
```

#### clusterName

Configures the cluster's name.

Type: `string`

#### network

Provides cluster network configuration.

Type: `ClusterNetworkConfig`

Examples:

```yaml
network:
  cni:
    name: flannel
  dnsDomain: cluster.local
  podSubnets:
    - 10.244.0.0/16
  serviceSubnets:
    - 10.96.0.0/12
```

#### token

The [bootstrap token](https://kubernetes.io/docs/reference/access-authn-authz/bootstrap-tokens/).

Type: `string`

Examples:

```yaml
wlzjyw.bei2zfylhs2by0wd
```

#### aescbcEncryptionSecret

The key used for the [encryption of secret data at rest](https://kubernetes.io/docs/tasks/administer-cluster/encrypt-data/).

Type: `string`

Examples:

```yaml
z01mye6j16bspJYtTB/5SFX8j7Ph4JXxM2Xuu4vsBPM=
```

#### ca

The base64 encoded root certificate authority used by Kubernetes.

Type: `PEMEncodedCertificateAndKey`

Examples:

```yaml
ca:
  crt: LS0tLS1CRUdJTiBDRV...
  key: LS0tLS1CRUdJTiBSU0...
```

#### apiServer

API server specific configuration options.

Type: `APIServerConfig`

Examples:

```yaml
apiServer:
  image: ...
  extraArgs:
    key: value
  certSANs:
    - 1.2.3.4
    - 5.6.7.8
```

#### controllerManager

Controller manager server specific configuration options.

Type: `ControllerManagerConfig`

Examples:

```yaml
controllerManager:
  image: ...
  extraArgs:
    key: value
```

#### proxy

Kube-proxy server-specific configuration options

Type: `ProxyConfig`

Examples:

```yaml
proxy:
  mode: ipvs
  extraArgs:
    key: value
```

#### scheduler

Scheduler server specific configuration options.

Type: `SchedulerConfig`

Examples:

```yaml
scheduler:
  image: ...
  extraArgs:
    key: value
```

#### etcd

Etcd specific configuration options.

Type: `EtcdConfig`

Examples:

```yaml
etcd:
  ca:
    crt: LS0tLS1CRUdJTiBDRV...
    key: LS0tLS1CRUdJTiBSU0...
  image: ...
```

#### podCheckpointer

Pod Checkpointer specific configuration options.

Type: `PodCheckpointer`

Examples:

```yaml
podCheckpointer:
  image: ...
```

#### coreDNS

Core DNS specific configuration options.

Type: `CoreDNS`

Examples:

```yaml
coreDNS:
  image: ...
```

#### extraManifests

A list of urls that point to additional manifests.
These will get automatically deployed by bootkube.

Type: `array`

Examples:

```yaml
extraManifests:
  - "https://www.mysweethttpserver.com/manifest1.yaml"
  - "https://www.mysweethttpserver.com/manifest2.yaml"
```

#### extraManifestHeaders

A map of key value pairs that will be added while fetching the ExtraManifests.

Type: `map`

Examples:

```yaml
extraManifestHeaders:
  Token: "1234567"
  X-ExtraInfo: info
```

#### adminKubeconfig

Settings for admin kubeconfig generation.
Certificate lifetime can be configured.

Type: `AdminKubeconfigConfig`

Examples:

```yaml
adminKubeconfig:
  certLifetime: 1h
```

---

### KubeletConfig

#### image

The `image` field is an optional reference to an alternative kubelet image.

Type: `string`

Examples:

```yaml
image: docker.io/<org>/kubelet:latest
```

#### extraArgs

The `extraArgs` field is used to provide additional flags to the kubelet.

Type: `map`

Examples:

```yaml
extraArgs:
  key: value
```

#### extraMounts

The `extraMounts` field is used to add additional mounts to the kubelet container.

Type: `array`

Examples:

```yaml
extraMounts:
  - source: /var/lib/example
    destination: /var/lib/example
    type: bind
    options:
      - rshared
      - ro
```

---

### NetworkConfig

#### hostname

Used to statically set the hostname for the host.

Type: `string`

#### interfaces

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

##### machine.network.interfaces.dummy

`dummy` is used to specify that this interface should be a virtual-only, dummy interface.
This parameter is optional.

##### machine.network.interfaces.routes

`routes` is used to specify static routes that may be necessary.
This parameter is optional.

Routes can be repeated and includes a `Network` and `Gateway` field.

Type: `array`

#### nameservers

Used to statically set the nameservers for the host.
Defaults to `1.1.1.1` and `8.8.8.8`

Type: `array`

#### extraHostEntries

Allows for extra entries to be added to /etc/hosts file

Type: `array`

Examples:

```yaml
extraHostEntries:
  - ip: 192.168.1.100
    aliases:
      - test
      - test.domain.tld
```

---

### InstallConfig

#### disk

The disk used to install the bootloader, and ephemeral partitions.

Type: `string`

Examples:

```yaml
/dev/sda
```

```yaml
/dev/nvme0
```

#### extraKernelArgs

Allows for supplying extra kernel args to the bootloader config.

Type: `array`

Examples:

```yaml
extraKernelArgs:
  - a=b
```

#### image

Allows for supplying the image used to perform the installation.

Type: `string`

Examples:

```yaml
image: docker.io/<org>/installer:latest
```

#### bootloader

Indicates if a bootloader should be installed.

Type: `bool`

Valid Values:

- `true`
- `yes`
- `false`
- `no`

#### wipe

Indicates if zeroes should be written to the `disk` before performing and installation.
Defaults to `true`.

Type: `bool`

Valid Values:

- `true`
- `yes`
- `false`
- `no`

#### force

Indicates if filesystems should be forcefully created.

Type: `bool`

Valid Values:

- `true`
- `yes`
- `false`
- `no`

---

### TimeConfig

#### servers

Specifies time (ntp) servers to use for setting system time.
Defaults to `pool.ntp.org`

> Note: This parameter only supports a single time server

Type: `array`

---

### RegistriesConfig

#### mirrors

Specifies mirror configuration for each registry.
This setting allows to use local pull-through caching registires,
air-gapped installations, etc.

Registry name is the first segment of image identifier, with 'docker.io'
being default one.
Name '\*' catches any registry names not specified explicitly.

Type: `map`

#### config

Specifies TLS & auth configuration for HTTPS image registries.
Mutual TLS can be enabled with 'clientIdentity' option.

TLS configuration can be skipped if registry has trusted
server certificate.

Type: `map`

---

### PodCheckpointer

#### image

The `image` field is an override to the default pod-checkpointer image.

Type: `string`

---

### CoreDNS

#### image

The `image` field is an override to the default coredns image.

Type: `string`

---

### Endpoint

---

### ControlPlaneConfig

#### endpoint

Endpoint is the canonical controlplane endpoint, which can be an IP address or a DNS hostname.
It is single-valued, and may optionally include a port number.

Type: `Endpoint`

Examples:

```yaml
https://1.2.3.4:443
```

#### localAPIServerPort

The port that the API server listens on internally.
This may be different than the port portion listed in the endpoint field above.
The default is 6443.

Type: `int`

---

### APIServerConfig

#### image

The container image used in the API server manifest.

Type: `string`

#### extraArgs

Extra arguments to supply to the API server.

Type: `map`

#### certSANs

Extra certificate subject alternative names for the API server's certificate.

Type: `array`

---

### ControllerManagerConfig

#### image

The container image used in the controller manager manifest.

Type: `string`

#### extraArgs

Extra arguments to supply to the controller manager.

Type: `map`

---

### ProxyConfig

#### image

The container image used in the kube-proxy manifest.

Type: `string`

#### mode

proxy mode of kube-proxy.
By default, this is 'iptables'.

Type: `string`

#### extraArgs

Extra arguments to supply to kube-proxy.

Type: `map`

---

### SchedulerConfig

#### image

The container image used in the scheduler manifest.

Type: `string`

#### extraArgs

Extra arguments to supply to the scheduler.

Type: `map`

---

### EtcdConfig

#### image

The container image used to create the etcd service.

Type: `string`

#### ca

The `ca` is the root certificate authority of the PKI.
It is composed of a base64 encoded `crt` and `key`.

Type: `PEMEncodedCertificateAndKey`

Examples:

```yaml
ca:
  crt: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUJIekNCMHF...
  key: LS0tLS1CRUdJTiBFRDI1NTE5IFBSSVZBVEUgS0VZLS0tLS0KTUM...
```

#### extraArgs

Extra arguments to supply to etcd.
Note that the following args are not allowed:

- `name`
- `data-dir`
- `initial-cluster-state`
- `listen-peer-urls`
- `listen-client-urls`
- `cert-file`
- `key-file`
- `trusted-ca-file`
- `peer-client-cert-auth`
- `peer-cert-file`
- `peer-trusted-ca-file`
- `peer-key-file`

Type: `map`

Examples:

```yaml
extraArgs:
  initial-cluster: https://1.2.3.4:2380
  advertise-client-urls: https://1.2.3.4:2379
```

---

### ClusterNetworkConfig

#### cni

The CNI used.
Composed of "name" and "url".
The "name" key only supports upstream bootkube options of "flannel" or "custom".
URLs is only used if name is equal to "custom".
URLs should point to a single yaml file that will get deployed.
Empty struct or any other name will default to bootkube's flannel.

Type: `CNIConfig`

Examples:

```yaml
cni:
  name: "custom"
  urls:
    - "https://www.mysweethttpserver.com/supersecretcni.yaml"
```

#### dnsDomain

The domain used by Kubernetes DNS.
The default is `cluster.local`

Type: `string`

Examples:

```yaml
cluser.local
```

#### podSubnets

The pod subnet CIDR.

Type: `array`

Examples:

```yaml
podSubnets:
  - 10.244.0.0/16
```

#### serviceSubnets

The service subnet CIDR.

Type: `array`

Examples:

```yaml
serviceSubnets:
  - 10.96.0.0/12
```

---

### CNIConfig

#### name

Name of CNI to use.

Type: `string`

#### urls

URLs containing manifests to apply for CNI.

Type: `array`

---

### AdminKubeconfigConfig

#### certLifetime

Admin kubeconfig certificate lifetime (default is 1 year).
Field format accepts any Go time.Duration format ('1h' for one hour, '10m' for ten minutes).

Type: `Duration`

---

### MachineDisk

#### device

The name of the disk to use.
Type: `string`

#### partitions

A list of partitions to create on the disk.
Type: `array`

---

### DiskPartition

#### size

The size of the partition in bytes. If `size:` is omitted, the partition is sized to occupy the full disk.

Type: `uint`

#### mountpoint

Where to mount the partition.
Type: `string`

---

### MachineFile

#### content

The contents of file.
Type: `string`

#### permissions

The file's permissions in octal.
Type: `FileMode`

#### path

The path of the file.
Type: `string`

#### op

The operation to use
Type: `string`

Valid Values:

- `create`
- `append`

---

### ExtraHost

#### ip

The IP of the host.
Type: `string`

#### aliases

The host alias.
Type: `array`

---

### Device

#### interface

The interface name.
Type: `string`

#### cidr

The CIDR to use.
Type: `string`

#### routes

A list of routes associated with the interface.
Type: `array`

#### bond

Bond specific options.
Type: `Bond`

#### vlans

VLAN specific options.
Type: `array`

#### mtu

The interface's MTU.
Type: `int`

#### dhcp

Indicates if DHCP should be used.
Type: `bool`

#### ignore

Indicates if the interface should be ignored.
Type: `bool`

#### dummy

Indicates if the interface is a dummy interface.
Type: `bool`

---

### Bond

#### interfaces

The interfaces that make up the bond.
Type: `array`

#### arpIPTarget

A bond option.
Please see the official kernel documentation.

Type: `array`

#### mode

A bond option.
Please see the official kernel documentation.

Type: `string`

#### xmitHashPolicy

A bond option.
Please see the official kernel documentation.

Type: `string`

#### lacpRate

A bond option.
Please see the official kernel documentation.

Type: `string`

#### adActorSystem

A bond option.
Please see the official kernel documentation.

Type: `string`

#### arpValidate

A bond option.
Please see the official kernel documentation.

Type: `string`

#### arpAllTargets

A bond option.
Please see the official kernel documentation.

Type: `string`

#### primary

A bond option.
Please see the official kernel documentation.

Type: `string`

#### primaryReselect

A bond option.
Please see the official kernel documentation.

Type: `string`

#### failOverMac

A bond option.
Please see the official kernel documentation.

Type: `string`

#### adSelect

A bond option.
Please see the official kernel documentation.

Type: `string`

#### miimon

A bond option.
Please see the official kernel documentation.

Type: `uint32`

#### updelay

A bond option.
Please see the official kernel documentation.

Type: `uint32`

#### downdelay

A bond option.
Please see the official kernel documentation.

Type: `uint32`

#### arpInterval

A bond option.
Please see the official kernel documentation.

Type: `uint32`

#### resendIgmp

A bond option.
Please see the official kernel documentation.

Type: `uint32`

#### minLinks

A bond option.
Please see the official kernel documentation.

Type: `uint32`

#### lpInterval

A bond option.
Please see the official kernel documentation.

Type: `uint32`

#### packetsPerSlave

A bond option.
Please see the official kernel documentation.

Type: `uint32`

#### numPeerNotif

A bond option.
Please see the official kernel documentation.

Type: `uint8`

#### tlbDynamicLb

A bond option.
Please see the official kernel documentation.

Type: `uint8`

#### allSlavesActive

A bond option.
Please see the official kernel documentation.

Type: `uint8`

#### useCarrier

A bond option.
Please see the official kernel documentation.

Type: `bool`

#### adActorSysPrio

A bond option.
Please see the official kernel documentation.

Type: `uint16`

#### adUserPortKey

A bond option.
Please see the official kernel documentation.

Type: `uint16`

#### peerNotifyDelay

A bond option.
Please see the official kernel documentation.

Type: `uint32`

---

### Vlan

#### cidr

The CIDR to use.
Type: `string`

#### routes

A list of routes associated with the VLAN.
Type: `array`

#### dhcp

Indicates if DHCP should be used.
Type: `bool`

#### vlanId

The VLAN's ID.
Type: `uint16`

---

### Route

#### network

The route's network.
Type: `string`

#### gateway

The route's gateway.
Type: `string`

---

### RegistryMirrorConfig

#### endpoints

List of endpoints (URLs) for registry mirrors to use.
Endpoint configures HTTP/HTTPS access mode, host name,
port and path (if path is not set, it defaults to `/v2`).

Type: `array`

---

### RegistryConfig

#### tls

The TLS configuration for this registry.
Type: `RegistryTLSConfig`

#### auth

The auth configuration for this registry.
Type: `RegistryAuthConfig`

---

### RegistryAuthConfig

#### username

Optional registry authentication.
The meaning of each field is the same with the corresponding field in .docker/config.json.

Type: `string`

#### password

Optional registry authentication.
The meaning of each field is the same with the corresponding field in .docker/config.json.

Type: `string`

#### auth

Optional registry authentication.
The meaning of each field is the same with the corresponding field in .docker/config.json.

Type: `string`

#### identityToken

Optional registry authentication.
The meaning of each field is the same with the corresponding field in .docker/config.json.

Type: `string`

---

### RegistryTLSConfig

#### clientIdentity

Enable mutual TLS authentication with the registry.
Client certificate and key should be base64-encoded.

Type: `PEMEncodedCertificateAndKey`

Examples:

```yaml
clientIdentity:
  crt: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUJIekNCMHF...
  key: LS0tLS1CRUdJTiBFRDI1NTE5IFBSSVZBVEUgS0VZLS0tLS0KTUM...
```

#### ca

CA registry certificate to add the list of trusted certificates.
Certificate should be base64-encoded.

Type: `array`

#### insecureSkipVerify

Skip TLS server certificate verification (not recommended).

Type: `bool`

---
