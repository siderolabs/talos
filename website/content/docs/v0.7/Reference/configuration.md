---
title: Configuration
---

<!-- markdownlint-disable -->

Package v1alpha1 configuration file contains all the options available for configuring a machine.

To generate a set of basic configuration files, run:
```bash
talosctl gen config --version v1alpha1 <cluster name> <cluster endpoint>
````

This will generate a machine config for each node type, and a talosconfig for the CLI.

## Config

### version

Type: <code>string</code>

Indicates the schema used to decode the contents.

Valid Values:

- ``v1alpha1``

### debug

Type: <code>bool</code>

Enable verbose logging.

Valid Values:

- `true`
- `yes`
- `false`
- `no`

### persist

Type: <code>bool</code>

Indicates whether to pull the machine config upon every boot.

Valid Values:

- `true`
- `yes`
- `false`
- `no`

### machine

Type: <code>[MachineConfig](#machineconfig)</code>

Provides machine specific configuration options.

### cluster

Type: <code>[ClusterConfig](#clusterconfig)</code>

Provides cluster specific configuration options.

---

## MachineConfig

### type

Type: <code>string</code>

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

Valid Values:

- ``init``
- ``controlplane``
- ``join``

### token

Type: <code>string</code>

The `token` is used by a machine to join the PKI of the cluster.
Using this token, a machine will create a certificate signing request (CSR), and request a certificate that will be used as its' identity.

Examples:

```yaml
token: 328hom.uqjzh6jnn2eie9oi
```

> Warning: It is important to ensure that this token is correct since a machine's certificate has a short TTL by default

### ca

Type: <code>PEMEncodedCertificateAndKey</code>

The root certificate authority of the PKI.
It is composed of a base64 encoded `crt` and `key`.

Examples:

```yaml
ca:
    crt: TFMwdExTMUNSVWRKVGlCRFJWSlVTVVpKUTBGVVJTMHRMUzB0Q2sxSlNVSklla05DTUhGLi4u
    key: TFMwdExTMUNSVWRKVGlCRlJESTFOVEU1SUZCU1NWWkJWRVVnUzBWWkxTMHRMUzBLVFVNLi4u
```

### certSANs

Type: <code>[]string</code>

Extra certificate subject alternative names for the machine's certificate.
By default, all non-loopback interface IPs are automatically added to the certificate's SANs.

Examples:

```yaml
certSANs:
    - 10.0.0.10
    - 172.16.0.10
    - 192.168.0.10
```

### kubelet

Type: <code>[KubeletConfig](#kubeletconfig)</code>

Used to provide additional options to the kubelet.

Examples:

```yaml
kubelet:
    image: docker.io/autonomy/kubelet:v1.19.3 # The `image` field is an optional reference to an alternative kubelet image.
    # The `extraArgs` field is used to provide additional flags to the kubelet.
    extraArgs:
        key: value

    # # The `extraMounts` field is used to add additional mounts to the kubelet container.
    # extraMounts:
    #     - destination: /var/lib/example
    #       type: bind
    #       source: /var/lib/example
    #       options:
    #         - rshared
    #         - ro
```

### network

Type: <code>[NetworkConfig](#networkconfig)</code>

Used to configure the machine's network.

Examples:

```yaml
network:
    hostname: worker-1 # Used to statically set the hostname for the host.
    # `interfaces` is used to define the network interface configuration.
    interfaces:
        - interface: "" # The interface name.
          cidr: "" # The CIDR to use.
          # A list of routes associated with the interface.
          routes: []
          bond: null # Bond specific options.
          # VLAN specific options.
          vlans: []
          mtu: 0 # The interface's MTU.
          dhcp: false # Indicates if DHCP should be used.
          ignore: false # Indicates if the interface should be ignored.
          dummy: false # Indicates if the interface is a dummy interface.
          dhcpOptions: null # DHCP specific options.
    # Used to statically set the nameservers for the host.
    nameservers:
        - 9.8.7.6
        - 8.7.6.5

    # # Allows for extra entries to be added to /etc/hosts file
    # extraHostEntries:
    #     - ip: 192.168.1.100 # The IP of the host.
    #       # The host alias.
    #       aliases:
    #         - test
    #         - test.domain.tld
```

### disks

Type: <code>[][MachineDisk](#machinedisk)</code>

Used to partition, format and mount additional disks.
Since the rootfs is read only with the exception of `/var`, mounts are only valid if they are under `/var`.
Note that the partitioning and formating is done only once, if and only if no existing  partitions are found.
If `size:` is omitted, the partition is sized to occupy full disk.

Examples:

```yaml
disks:
    - device: /dev/sdb # The name of the disk to use.
      # A list of partitions to create on the disk.
      partitions:
        - size: 100000000 # This size of the partition in bytes.
          mountpoint: //lib/extra # Where to mount the partition.
```

> Note: `size` is in units of bytes.

### install

Type: <code>[InstallConfig](#installconfig)</code>

Used to provide instructions for bare-metal installations.

Examples:

```yaml
install:
    disk: /dev/sda # The disk used to install the bootloader, and ephemeral partitions.
    # Allows for supplying extra kernel args to the bootloader config.
    extraKernelArgs:
        - option=value
    image: ghcr.io/talos-systems/installer:latest # Allows for supplying the image used to perform the installation.
    bootloader: true # Indicates if a bootloader should be installed.
    wipe: false # Indicates if zeroes should be written to the `disk` before performing and installation.
```

### files

Type: <code>[][MachineFile](#machinefile)</code>

Allows the addition of user specified files.
The value of `op` can be `create`, `overwrite`, or `append`.
In the case of `create`, `path` must not exist.
In the case of `overwrite`, and `append`, `path` must be a valid file.
If an `op` value of `append` is used, the existing file will be appended.
Note that the file contents are not required to be base64 encoded.

Examples:

```yaml
files:
    - content: '...' # The contents of file.
      permissions: 438 # The file's permissions in octal.
      path: /tmp/file.txt # The path of the file.
      op: append # The operation to use
```

> Note: The specified `path` is relative to `/var`.

### env

Type: <code>Env</code>

The `env` field allows for the addition of environment variables to a machine.
All environment variables are set on the machine in addition to every service.

Valid Values:

- ``GRPC_GO_LOG_VERBOSITY_LEVEL``
- ``GRPC_GO_LOG_SEVERITY_LEVEL``
- ``http_proxy``
- ``https_proxy``
- ``no_proxy``

Examples:

```yaml
env:
    GRPC_GO_LOG_SEVERITY_LEVEL: info
    GRPC_GO_LOG_VERBOSITY_LEVEL: "99"
    https_proxy: http://SERVER:PORT/
```

```yaml
env:
    GRPC_GO_LOG_SEVERITY_LEVEL: error
    https_proxy: https://USERNAME:PASSWORD@SERVER:PORT/
```

```yaml
env:
    https_proxy: http://DOMAIN\USERNAME:PASSWORD@SERVER:PORT/
```

### time

Type: <code>[TimeConfig](#timeconfig)</code>

Used to configure the machine's time settings.

Examples:

```yaml
time:
    disabled: false # Indicates if time (ntp) is disabled for the machine
    # Specifies time (ntp) servers to use for setting system time.
    servers:
        - time.cloudflare.com
```

### sysctls

Type: <code>map[string]string</code>

Used to configure the machine's sysctls.

Examples:

```yaml
sysctls:
    kernel.domainname: talos.dev
    net.ipv4.ip_forward: "0"
```

### registries

Type: <code>[RegistriesConfig](#registriesconfig)</code>

Used to configure the machine's container image registry mirrors.

Automatically generates matching CRI configuration for registry mirrors.

Section `mirrors` allows to redirect requests for images to non-default registry,
which might be local registry or caching mirror.

Section `config` provides a way to authenticate to the registry with TLS client
identity, provide registry CA, or authentication information.
Authentication information has same meaning with the corresponding field in `.docker/config.json`.

See also matching configuration for [CRI containerd plugin](https://github.com/containerd/cri/blob/master/docs/registry.md).

Examples:

```yaml
registries:
    # Specifies mirror configuration for each registry.
    mirrors:
        docker.io:
            # List of endpoints (URLs) for registry mirrors to use.
            endpoints:
                - https://registry-1.docker.io
    # Specifies TLS & auth configuration for HTTPS image registries.
    config:
        some.host:123:
            # The TLS configuration for this registry.
            tls:
                # Enable mutual TLS authentication with the registry.
                clientIdentity:
                    crt: Li4u
                    key: Li4u
            # The auth configuration for this registry.
            auth:
                username: '...' # Optional registry authentication.
                password: '...' # Optional registry authentication.
                auth: '...' # Optional registry authentication.
                identityToken: '...' # Optional registry authentication.
```

---

## ClusterConfig

### controlPlane

Type: <code>[ControlPlaneConfig](#controlplaneconfig)</code>

Provides control plane specific configuration options.

Examples:

```yaml
controlPlane:
    endpoint: https://1.2.3.4 # Endpoint is the canonical controlplane endpoint, which can be an IP address or a DNS hostname.
    localAPIServerPort: 443 # The port that the API server listens on internally.
```

### clusterName

Type: <code>string</code>

Configures the cluster's name.

### network

Type: <code>[ClusterNetworkConfig](#clusternetworkconfig)</code>

Provides cluster network configuration.

Examples:

```yaml
network:
    # The CNI used.
    cni:
        name: flannel # Name of CNI to use.
    dnsDomain: cluster.local # The domain used by Kubernetes DNS.
    # The pod subnet CIDR.
    podSubnets:
        - 10.244.0.0/16
    # The service subnet CIDR.
    serviceSubnets:
        - 10.96.0.0/12
```

### token

Type: <code>string</code>

The [bootstrap token](https://kubernetes.io/docs/reference/access-authn-authz/bootstrap-tokens/).

Examples:

```yaml
token: wlzjyw.bei2zfylhs2by0wd
```

### aescbcEncryptionSecret

Type: <code>string</code>

The key used for the [encryption of secret data at rest](https://kubernetes.io/docs/tasks/administer-cluster/encrypt-data/).

Examples:

```yaml
aescbcEncryptionSecret: z01mye6j16bspJYtTB/5SFX8j7Ph4JXxM2Xuu4vsBPM=
```

### ca

Type: <code>PEMEncodedCertificateAndKey</code>

The base64 encoded root certificate authority used by Kubernetes.

Examples:

```yaml
ca:
    crt: TFMwdExTMUNSVWRKVGlCRFJWSlVTVVpKUTBGVVJTMHRMUzB0Q2sxSlNVSklla05DTUhGLi4u
    key: TFMwdExTMUNSVWRKVGlCRlJESTFOVEU1SUZCU1NWWkJWRVVnUzBWWkxTMHRMUzBLVFVNLi4u
```

### apiServer

Type: <code>[APIServerConfig](#apiserverconfig)</code>

API server specific configuration options.

Examples:

```yaml
apiServer:
    image: '...' # The container image used in the API server manifest.
    # Extra arguments to supply to the API server.
    extraArgs:
        key: value
    # Extra certificate subject alternative names for the API server's certificate.
    certSANs:
        - 1.2.3.4
        - 4.5.6.7
```

### controllerManager

Type: <code>[ControllerManagerConfig](#controllermanagerconfig)</code>

Controller manager server specific configuration options.

Examples:

```yaml
controllerManager:
    image: '...' # The container image used in the controller manager manifest.
    # Extra arguments to supply to the controller manager.
    extraArgs:
        key: value
```

### proxy

Type: <code>[ProxyConfig](#proxyconfig)</code>

Kube-proxy server-specific configuration options

Examples:

```yaml
proxy:
    image: '...' # The container image used in the kube-proxy manifest.
    mode: ipvs # proxy mode of kube-proxy.
    # Extra arguments to supply to kube-proxy.
    extraArgs:
        key: value
```

### scheduler

Type: <code>[SchedulerConfig](#schedulerconfig)</code>

Scheduler server specific configuration options.

Examples:

```yaml
scheduler:
    image: '...' # The container image used in the scheduler manifest.
    # Extra arguments to supply to the scheduler.
    extraArgs:
        key: value
```

### etcd

Type: <code>[EtcdConfig](#etcdconfig)</code>

Etcd specific configuration options.

Examples:

```yaml
etcd:
    image: '...' # The container image used to create the etcd service.
    # The `ca` is the root certificate authority of the PKI.
    ca:
        crt: TFMwdExTMUNSVWRKVGlCRFJWSlVTVVpKUTBGVVJTMHRMUzB0Q2sxSlNVSklla05DTUhGLi4u
        key: TFMwdExTMUNSVWRKVGlCRlJESTFOVEU1SUZCU1NWWkJWRVVnUzBWWkxTMHRMUzBLVFVNLi4u
    # Extra arguments to supply to etcd.
    extraArgs:
        key: value
```

### podCheckpointer

Type: <code>[PodCheckpointer](#podcheckpointer)</code>

Pod Checkpointer specific configuration options.

Examples:

```yaml
podCheckpointer:
    image: '...' # The `image` field is an override to the default pod-checkpointer image.
```

### coreDNS

Type: <code>[CoreDNS](#coredns)</code>

Core DNS specific configuration options.

Examples:

```yaml
coreDNS:
    image: '...' # The `image` field is an override to the default coredns image.
```

### extraManifests

Type: <code>[]string</code>

A list of urls that point to additional manifests.
These will get automatically deployed by bootkube.

Examples:

```yaml
extraManifests:
    - https://www.mysweethttpserver.com/manifest1.yaml
    - https://www.mysweethttpserver.com/manifest2.yaml
```

### extraManifestHeaders

Type: <code>map[string]string</code>

A map of key value pairs that will be added while fetching the ExtraManifests.

Examples:

```yaml
extraManifestHeaders:
    Token: "1234567"
    X-ExtraInfo: info
```

### adminKubeconfig

Type: <code>[AdminKubeconfigConfig](#adminkubeconfigconfig)</code>

Settings for admin kubeconfig generation.
Certificate lifetime can be configured.

Examples:

```yaml
adminKubeconfig:
    certLifetime: 1h0m0s # Admin kubeconfig certificate lifetime (default is 1 year).
```

### allowSchedulingOnMasters

Type: <code>bool</code>

Indicates if master nodes are schedulable.

Valid Values:

- `true`
- `yes`
- `false`
- `no`

---

## KubeletConfig

### image

Type: <code>string</code>

The `image` field is an optional reference to an alternative kubelet image.

Examples:

```yaml
image: docker.io/<org>/kubelet:latest
```

### extraArgs

Type: <code>map[string]string</code>

The `extraArgs` field is used to provide additional flags to the kubelet.

Examples:

```yaml
extraArgs:
    key: value
```

### extraMounts

Type: <code>[]Mount</code>

The `extraMounts` field is used to add additional mounts to the kubelet container.

Examples:

```yaml
extraMounts:
    - destination: /var/lib/example
      type: bind
      source: /var/lib/example
      options:
        - rshared
        - ro
```

---

## NetworkConfig

### hostname

Type: <code>string</code>

Used to statically set the hostname for the host.

### interfaces

Type: <code>[][Device](#device)</code>

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
>
> Note: To configure an interface with *only* IPv6 SLAAC addressing, CIDR should be set to "" and DHCP to false
> in order for Talos to skip configuration of addresses.
> All other options will still apply.

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

### nameservers

Type: <code>[]string</code>

Used to statically set the nameservers for the host.
Defaults to `1.1.1.1` and `8.8.8.8`

### extraHostEntries

Type: <code>[][ExtraHost](#extrahost)</code>

Allows for extra entries to be added to /etc/hosts file

Examples:

```yaml
extraHostEntries:
    - ip: 192.168.1.100 # The IP of the host.
      # The host alias.
      aliases:
        - test
        - test.domain.tld
```

---

## InstallConfig

### disk

Type: <code>string</code>

The disk used to install the bootloader, and ephemeral partitions.

Examples:

```yaml
disk: /dev/sda
```

```yaml
disk: /dev/nvme0
```

### extraKernelArgs

Type: <code>[]string</code>

Allows for supplying extra kernel args to the bootloader config.

Examples:

```yaml
extraKernelArgs:
    - a=b
```

### image

Type: <code>string</code>

Allows for supplying the image used to perform the installation.

Examples:

```yaml
image: docker.io/<org>/installer:latest
```

### bootloader

Type: <code>bool</code>

Indicates if a bootloader should be installed.

Valid Values:

- `true`
- `yes`
- `false`
- `no`

### wipe

Type: <code>bool</code>

Indicates if zeroes should be written to the `disk` before performing and installation.
Defaults to `true`.

Valid Values:

- `true`
- `yes`
- `false`
- `no`

---

## TimeConfig

### disabled

Type: <code>bool</code>

Indicates if time (ntp) is disabled for the machine
Defaults to `false`.

### servers

Type: <code>[]string</code>

Specifies time (ntp) servers to use for setting system time.
Defaults to `pool.ntp.org`

> Note: This parameter only supports a single time server

---

## RegistriesConfig

### mirrors

Type: <code>map[string][RegistryMirrorConfig](#registrymirrorconfig)</code>

Specifies mirror configuration for each registry.
This setting allows to use local pull-through caching registires,
air-gapped installations, etc.

Registry name is the first segment of image identifier, with 'docker.io'
being default one.
Name '*' catches any registry names not specified explicitly.

### config

Type: <code>map[string][RegistryConfig](#registryconfig)</code>

Specifies TLS & auth configuration for HTTPS image registries.
Mutual TLS can be enabled with 'clientIdentity' option.

TLS configuration can be skipped if registry has trusted
server certificate.

---

## PodCheckpointer

### image

Type: <code>string</code>

The `image` field is an override to the default pod-checkpointer image.

---

## CoreDNS

### image

Type: <code>string</code>

The `image` field is an override to the default coredns image.

---

## Endpoint

---

## ControlPlaneConfig

### endpoint

Type: <code>[Endpoint](#endpoint)</code>

Endpoint is the canonical controlplane endpoint, which can be an IP address or a DNS hostname.
It is single-valued, and may optionally include a port number.

Examples:

```yaml
endpoint: https://1.2.3.4:443
```

### localAPIServerPort

Type: <code>int</code>

The port that the API server listens on internally.
This may be different than the port portion listed in the endpoint field above.
The default is 6443.

---

## APIServerConfig

### image

Type: <code>string</code>

The container image used in the API server manifest.

### extraArgs

Type: <code>map[string]string</code>

Extra arguments to supply to the API server.

### certSANs

Type: <code>[]string</code>

Extra certificate subject alternative names for the API server's certificate.

---

## ControllerManagerConfig

### image

Type: <code>string</code>

The container image used in the controller manager manifest.

### extraArgs

Type: <code>map[string]string</code>

Extra arguments to supply to the controller manager.

---

## ProxyConfig

### image

Type: <code>string</code>

The container image used in the kube-proxy manifest.

### mode

Type: <code>string</code>

proxy mode of kube-proxy.
By default, this is 'iptables'.

### extraArgs

Type: <code>map[string]string</code>

Extra arguments to supply to kube-proxy.

---

## SchedulerConfig

### image

Type: <code>string</code>

The container image used in the scheduler manifest.

### extraArgs

Type: <code>map[string]string</code>

Extra arguments to supply to the scheduler.

---

## EtcdConfig

### image

Type: <code>string</code>

The container image used to create the etcd service.

### ca

Type: <code>PEMEncodedCertificateAndKey</code>

The `ca` is the root certificate authority of the PKI.
It is composed of a base64 encoded `crt` and `key`.

Examples:

```yaml
ca:
    crt: TFMwdExTMUNSVWRKVGlCRFJWSlVTVVpKUTBGVVJTMHRMUzB0Q2sxSlNVSklla05DTUhGLi4u
    key: TFMwdExTMUNSVWRKVGlCRlJESTFOVEU1SUZCU1NWWkJWRVVnUzBWWkxTMHRMUzBLVFVNLi4u
```

### extraArgs

Type: <code>map[string]string</code>

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

---

## ClusterNetworkConfig

### cni

Type: <code>[CNIConfig](#cniconfig)</code>

The CNI used.
Composed of "name" and "url".
The "name" key only supports upstream bootkube options of "flannel" or "custom".
URLs is only used if name is equal to "custom".
URLs should point to a single yaml file that will get deployed.
Empty struct or any other name will default to bootkube's flannel.

Examples:

```yaml
cni:
    name: custom # Name of CNI to use.
    # URLs containing manifests to apply for CNI.
    urls:
        - https://www.mysweethttpserver.com/supersecretcni.yaml
```

### dnsDomain

Type: <code>string</code>

The domain used by Kubernetes DNS.
The default is `cluster.local`

Examples:

```yaml
dnsDomain: cluser.local
```

### podSubnets

Type: <code>[]string</code>

The pod subnet CIDR.

Examples:

```yaml
podSubnets:
    - 10.244.0.0/16
```

### serviceSubnets

Type: <code>[]string</code>

The service subnet CIDR.

Examples:

```yaml
serviceSubnets:
    - 10.96.0.0/12
```

---

## CNIConfig

### name

Type: <code>string</code>

Name of CNI to use.

### urls

Type: <code>[]string</code>

URLs containing manifests to apply for CNI.

---

## AdminKubeconfigConfig

### certLifetime

Type: <code>Duration</code>

Admin kubeconfig certificate lifetime (default is 1 year).
Field format accepts any Go time.Duration format ('1h' for one hour, '10m' for ten minutes).

---

## MachineDisk

### device

Type: <code>string</code>

The name of the disk to use.

### partitions

Type: <code>[][DiskPartition](#diskpartition)</code>

A list of partitions to create on the disk.

---

## DiskPartition

### size

Type: <code>uint</code>

This size of the partition in bytes.

### mountpoint

Type: <code>string</code>

Where to mount the partition.

---

## MachineFile

### content

Type: <code>string</code>

The contents of file.

### permissions

Type: <code>FileMode</code>

The file's permissions in octal.

### path

Type: <code>string</code>

The path of the file.

### op

Type: <code>string</code>

The operation to use

Valid Values:

- `create`
- `append`

---

## ExtraHost

### ip

Type: <code>string</code>

The IP of the host.

### aliases

Type: <code>[]string</code>

The host alias.

---

## Device

### interface

Type: <code>string</code>

The interface name.

### cidr

Type: <code>string</code>

The CIDR to use.

### routes

Type: <code>[][Route](#route)</code>

A list of routes associated with the interface.
If used in combination with DHCP, these routes will be appended to routes returned by DHCP server.

### bond

Type: <code>[Bond](#bond)</code>

Bond specific options.

### vlans

Type: <code>[][Vlan](#vlan)</code>

VLAN specific options.

### mtu

Type: <code>int</code>

The interface's MTU.
If used in combination with DHCP, this will override any MTU settings returned from DHCP server.

### dhcp

Type: <code>bool</code>

Indicates if DHCP should be used.

### ignore

Type: <code>bool</code>

Indicates if the interface should be ignored.

### dummy

Type: <code>bool</code>

Indicates if the interface is a dummy interface.

### dhcpOptions

Type: <code>[DHCPOptions](#dhcpoptions)</code>

DHCP specific options.
DHCP *must* be set to true for these to take effect.

---

## DHCPOptions

### routeMetric

Type: <code>uint32</code>

The priority of all routes received via DHCP

---

## Bond

### interfaces

Type: <code>[]string</code>

The interfaces that make up the bond.

### arpIPTarget

Type: <code>[]string</code>

A bond option.
Please see the official kernel documentation.

### mode

Type: <code>string</code>

A bond option.
Please see the official kernel documentation.

### xmitHashPolicy

Type: <code>string</code>

A bond option.
Please see the official kernel documentation.

### lacpRate

Type: <code>string</code>

A bond option.
Please see the official kernel documentation.

### adActorSystem

Type: <code>string</code>

A bond option.
Please see the official kernel documentation.

### arpValidate

Type: <code>string</code>

A bond option.
Please see the official kernel documentation.

### arpAllTargets

Type: <code>string</code>

A bond option.
Please see the official kernel documentation.

### primary

Type: <code>string</code>

A bond option.
Please see the official kernel documentation.

### primaryReselect

Type: <code>string</code>

A bond option.
Please see the official kernel documentation.

### failOverMac

Type: <code>string</code>

A bond option.
Please see the official kernel documentation.

### adSelect

Type: <code>string</code>

A bond option.
Please see the official kernel documentation.

### miimon

Type: <code>uint32</code>

A bond option.
Please see the official kernel documentation.

### updelay

Type: <code>uint32</code>

A bond option.
Please see the official kernel documentation.

### downdelay

Type: <code>uint32</code>

A bond option.
Please see the official kernel documentation.

### arpInterval

Type: <code>uint32</code>

A bond option.
Please see the official kernel documentation.

### resendIgmp

Type: <code>uint32</code>

A bond option.
Please see the official kernel documentation.

### minLinks

Type: <code>uint32</code>

A bond option.
Please see the official kernel documentation.

### lpInterval

Type: <code>uint32</code>

A bond option.
Please see the official kernel documentation.

### packetsPerSlave

Type: <code>uint32</code>

A bond option.
Please see the official kernel documentation.

### numPeerNotif

Type: <code>uint8</code>

A bond option.
Please see the official kernel documentation.

### tlbDynamicLb

Type: <code>uint8</code>

A bond option.
Please see the official kernel documentation.

### allSlavesActive

Type: <code>uint8</code>

A bond option.
Please see the official kernel documentation.

### useCarrier

Type: <code>bool</code>

A bond option.
Please see the official kernel documentation.

### adActorSysPrio

Type: <code>uint16</code>

A bond option.
Please see the official kernel documentation.

### adUserPortKey

Type: <code>uint16</code>

A bond option.
Please see the official kernel documentation.

### peerNotifyDelay

Type: <code>uint32</code>

A bond option.
Please see the official kernel documentation.

---

## Vlan

### cidr

Type: <code>string</code>

The CIDR to use.

### routes

Type: <code>[][Route](#route)</code>

A list of routes associated with the VLAN.

### dhcp

Type: <code>bool</code>

Indicates if DHCP should be used.

### vlanId

Type: <code>uint16</code>

The VLAN's ID.

---

## Route

### network

Type: <code>string</code>

The route's network.

### gateway

Type: <code>string</code>

The route's gateway.

---

## RegistryMirrorConfig

### endpoints

Type: <code>[]string</code>

List of endpoints (URLs) for registry mirrors to use.
Endpoint configures HTTP/HTTPS access mode, host name,
port and path (if path is not set, it defaults to `/v2`).

---

## RegistryConfig

### tls

Type: <code>[RegistryTLSConfig](#registrytlsconfig)</code>

The TLS configuration for this registry.

### auth

Type: <code>[RegistryAuthConfig](#registryauthconfig)</code>

The auth configuration for this registry.

---

## RegistryAuthConfig

### username

Type: <code>string</code>

Optional registry authentication.
The meaning of each field is the same with the corresponding field in .docker/config.json.

### password

Type: <code>string</code>

Optional registry authentication.
The meaning of each field is the same with the corresponding field in .docker/config.json.

### auth

Type: <code>string</code>

Optional registry authentication.
The meaning of each field is the same with the corresponding field in .docker/config.json.

### identityToken

Type: <code>string</code>

Optional registry authentication.
The meaning of each field is the same with the corresponding field in .docker/config.json.

---

## RegistryTLSConfig

### clientIdentity

Type: <code>PEMEncodedCertificateAndKey</code>

Enable mutual TLS authentication with the registry.
Client certificate and key should be base64-encoded.

Examples:

```yaml
clientIdentity:
    crt: TFMwdExTMUNSVWRKVGlCRFJWSlVTVVpKUTBGVVJTMHRMUzB0Q2sxSlNVSklla05DTUhGLi4u
    key: TFMwdExTMUNSVWRKVGlCRlJESTFOVEU1SUZCU1NWWkJWRVVnUzBWWkxTMHRMUzBLVFVNLi4u
```

### ca

Type: <code>Base64Bytes</code>

CA registry certificate to add the list of trusted certificates.
Certificate should be base64-encoded.

### insecureSkipVerify

Type: <code>bool</code>

Skip TLS server certificate verification (not recommended).

---
