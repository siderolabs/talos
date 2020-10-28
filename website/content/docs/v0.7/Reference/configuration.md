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



<div class="dd">

<code>version</code>  <i>string</i>

</div>
<div class="dt">

Indicates the schema used to decode the contents.


Valid values:


  - <code>v1alpha1</code>
</div>

<div class="dd">

<code>debug</code>  <i>bool</i>

</div>
<div class="dt">

Enable verbose logging.


Valid values:


  - <code>true</code>

  - <code>yes</code>

  - <code>false</code>

  - <code>no</code>
</div>

<div class="dd">

<code>persist</code>  <i>bool</i>

</div>
<div class="dt">

Indicates whether to pull the machine config upon every boot.


Valid values:


  - <code>true</code>

  - <code>yes</code>

  - <code>false</code>

  - <code>no</code>
</div>

<div class="dd">

<code>machine</code>  <i><a href="#machineconfig">MachineConfig</a></i>

</div>
<div class="dt">

Provides machine specific configuration options.

</div>

<div class="dd">

<code>cluster</code>  <i><a href="#clusterconfig">ClusterConfig</a></i>

</div>
<div class="dt">

Provides cluster specific configuration options.

</div>





---

## MachineConfig
Appears in:


- <code><a href="#config">Config</a>.machine</code>



<div class="dd">

<code>type</code>  <i>string</i>

</div>
<div class="dt">

Defines the role of the machine within the cluster.

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


Valid values:


  - <code>init</code>

  - <code>controlplane</code>

  - <code>join</code>
</div>

<div class="dd">

<code>token</code>  <i>string</i>

</div>
<div class="dt">

The `token` is used by a machine to join the PKI of the cluster.
Using this token, a machine will create a certificate signing request (CSR), and request a certificate that will be used as its' identity.


> Warning: It is important to ensure that this token is correct since a machine's certificate has a short TTL by default



Examples:


``` yaml
token: 328hom.uqjzh6jnn2eie9oi
```


</div>

<div class="dd">

<code>ca</code>  <i>PEMEncodedCertificateAndKey</i>

</div>
<div class="dt">

The root certificate authority of the PKI.
It is composed of a base64 encoded `crt` and `key`.



Examples:


``` yaml
ca:
    crt: TFMwdExTMUNSVWRKVGlCRFJWSlVTVVpKUTBGVVJTMHRMUzB0Q2sxSlNVSklla05DTUhGLi4u
    key: TFMwdExTMUNSVWRKVGlCRlJESTFOVEU1SUZCU1NWWkJWRVVnUzBWWkxTMHRMUzBLVFVNLi4u
```


</div>

<div class="dd">

<code>certSANs</code>  <i>[]string</i>

</div>
<div class="dt">

Extra certificate subject alternative names for the machine's certificate.
By default, all non-loopback interface IPs are automatically added to the certificate's SANs.



Examples:


``` yaml
certSANs:
    - 10.0.0.10
    - 172.16.0.10
    - 192.168.0.10
```


</div>

<div class="dd">

<code>kubelet</code>  <i><a href="#kubeletconfig">KubeletConfig</a></i>

</div>
<div class="dt">

Used to provide additional options to the kubelet.



Examples:


``` yaml
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


</div>

<div class="dd">

<code>network</code>  <i><a href="#networkconfig">NetworkConfig</a></i>

</div>
<div class="dt">

Used to configure the machine's network.



Examples:


``` yaml
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


</div>

<div class="dd">

<code>disks</code>  <i>[]<a href="#machinedisk">MachineDisk</a></i>

</div>
<div class="dt">

Used to partition, format and mount additional disks.
Since the rootfs is read only with the exception of `/var`, mounts are only valid if they are under `/var`.
Note that the partitioning and formating is done only once, if and only if no existing  partitions are found.
If `size:` is omitted, the partition is sized to occupy full disk.


> Note: `size` is in units of bytes.



Examples:


``` yaml
disks:
    - device: /dev/sdb # The name of the disk to use.
      # A list of partitions to create on the disk.
      partitions:
        - size: 100000000 # This size of the partition in bytes.
          mountpoint: /var/mnt/extra # Where to mount the partition.
```


</div>

<div class="dd">

<code>install</code>  <i><a href="#installconfig">InstallConfig</a></i>

</div>
<div class="dt">

Used to provide instructions for bare-metal installations.



Examples:


``` yaml
install:
    disk: /dev/sda # The disk used to install the bootloader, and ephemeral partitions.
    # Allows for supplying extra kernel args to the bootloader config.
    extraKernelArgs:
        - option=value
    image: ghcr.io/talos-systems/installer:latest # Allows for supplying the image used to perform the installation.
    bootloader: true # Indicates if a bootloader should be installed.
    wipe: false # Indicates if zeroes should be written to the `disk` before performing and installation.
```


</div>

<div class="dd">

<code>files</code>  <i>[]<a href="#machinefile">MachineFile</a></i>

</div>
<div class="dt">

Allows the addition of user specified files.
The value of `op` can be `create`, `overwrite`, or `append`.
In the case of `create`, `path` must not exist.
In the case of `overwrite`, and `append`, `path` must be a valid file.
If an `op` value of `append` is used, the existing file will be appended.
Note that the file contents are not required to be base64 encoded.


> Note: The specified `path` is relative to `/var`.



Examples:


``` yaml
files:
    - content: '...' # The contents of file.
      permissions: 438 # The file's permissions in octal.
      path: /tmp/file.txt # The path of the file.
      op: append # The operation to use
```


</div>

<div class="dd">

<code>env</code>  <i>Env</i>

</div>
<div class="dt">

The `env` field allows for the addition of environment variables to a machine.
All environment variables are set on the machine in addition to every service.


Valid values:


  - <code>`GRPC_GO_LOG_VERBOSITY_LEVEL`</code>

  - <code>`GRPC_GO_LOG_SEVERITY_LEVEL`</code>

  - <code>`http_proxy`</code>

  - <code>`https_proxy`</code>

  - <code>`no_proxy`</code>


Examples:


``` yaml
env:
    GRPC_GO_LOG_SEVERITY_LEVEL: info
    GRPC_GO_LOG_VERBOSITY_LEVEL: "99"
    https_proxy: http://SERVER:PORT/
```

``` yaml
env:
    GRPC_GO_LOG_SEVERITY_LEVEL: error
    https_proxy: https://USERNAME:PASSWORD@SERVER:PORT/
```

``` yaml
env:
    https_proxy: http://DOMAIN\USERNAME:PASSWORD@SERVER:PORT/
```


</div>

<div class="dd">

<code>time</code>  <i><a href="#timeconfig">TimeConfig</a></i>

</div>
<div class="dt">

Used to configure the machine's time settings.



Examples:


``` yaml
time:
    disabled: false # Indicates if time (ntp) is disabled for the machine
    # Specifies time (ntp) servers to use for setting system time.
    servers:
        - time.cloudflare.com
```


</div>

<div class="dd">

<code>sysctls</code>  <i>map[string]string</i>

</div>
<div class="dt">

Used to configure the machine's sysctls.



Examples:


``` yaml
sysctls:
    kernel.domainname: talos.dev
    net.ipv4.ip_forward: "0"
```


</div>

<div class="dd">

<code>registries</code>  <i><a href="#registriesconfig">RegistriesConfig</a></i>

</div>
<div class="dt">

Used to configure the machine's container image registry mirrors.

Automatically generates matching CRI configuration for registry mirrors.

Section `mirrors` allows to redirect requests for images to non-default registry,
which might be local registry or caching mirror.

Section `config` provides a way to authenticate to the registry with TLS client
identity, provide registry CA, or authentication information.
Authentication information has same meaning with the corresponding field in `.docker/config.json`.

See also matching configuration for [CRI containerd plugin](https://github.com/containerd/cri/blob/master/docs/registry.md).



Examples:


``` yaml
registries:
    # Specifies mirror configuration for each registry.
    mirrors:
        docker.io:
            # List of endpoints (URLs) for registry mirrors to use.
            endpoints:
                - https://registry.local
        ghcr.io:
            # List of endpoints (URLs) for registry mirrors to use.
            endpoints:
                - https://registry.insecure
                - https://ghcr.io/v2/
    # Specifies TLS & auth configuration for HTTPS image registries.
    config:
        registry.insecure:
            # The TLS configuration for this registry.
            tls:
                insecureSkipVerify: true # Skip TLS server certificate verification (not recommended).

                # # Enable mutual TLS authentication with the registry.
                # clientIdentity:
                #     crt: TFMwdExTMUNSVWRKVGlCRFJWSlVTVVpKUTBGVVJTMHRMUzB0Q2sxSlNVSklla05DTUhGLi4u
                #     key: TFMwdExTMUNSVWRKVGlCRlJESTFOVEU1SUZCU1NWWkJWRVVnUzBWWkxTMHRMUzBLVFVNLi4u
        registry.local:
            # The TLS configuration for this registry.
            tls:
                # Enable mutual TLS authentication with the registry.
                clientIdentity:
                    crt: TFMwdExTMUNSVWRKVGlCRFJWSlVTVVpKUTBGVVJTMHRMUzB0Q2sxSlNVSklla05DTUhGLi4u
                    key: TFMwdExTMUNSVWRKVGlCRlJESTFOVEU1SUZCU1NWWkJWRVVnUzBWWkxTMHRMUzBLVFVNLi4u
            # The auth configuration for this registry.
            auth:
                username: username # Optional registry authentication.
                password: password # Optional registry authentication.
```


</div>





---

## ClusterConfig
Appears in:


- <code><a href="#config">Config</a>.cluster</code>



<div class="dd">

<code>controlPlane</code>  <i><a href="#controlplaneconfig">ControlPlaneConfig</a></i>

</div>
<div class="dt">

Provides control plane specific configuration options.



Examples:


``` yaml
controlPlane:
    endpoint: https://1.2.3.4 # Endpoint is the canonical controlplane endpoint, which can be an IP address or a DNS hostname.
    localAPIServerPort: 443 # The port that the API server listens on internally.
```


</div>

<div class="dd">

<code>clusterName</code>  <i>string</i>

</div>
<div class="dt">

Configures the cluster's name.

</div>

<div class="dd">

<code>network</code>  <i><a href="#clusternetworkconfig">ClusterNetworkConfig</a></i>

</div>
<div class="dt">

Provides cluster network configuration.



Examples:


``` yaml
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


</div>

<div class="dd">

<code>token</code>  <i>string</i>

</div>
<div class="dt">

The [bootstrap token](https://kubernetes.io/docs/reference/access-authn-authz/bootstrap-tokens/).



Examples:


``` yaml
token: wlzjyw.bei2zfylhs2by0wd
```


</div>

<div class="dd">

<code>aescbcEncryptionSecret</code>  <i>string</i>

</div>
<div class="dt">

The key used for the [encryption of secret data at rest](https://kubernetes.io/docs/tasks/administer-cluster/encrypt-data/).



Examples:


``` yaml
aescbcEncryptionSecret: z01mye6j16bspJYtTB/5SFX8j7Ph4JXxM2Xuu4vsBPM=
```


</div>

<div class="dd">

<code>ca</code>  <i>PEMEncodedCertificateAndKey</i>

</div>
<div class="dt">

The base64 encoded root certificate authority used by Kubernetes.



Examples:


``` yaml
ca:
    crt: TFMwdExTMUNSVWRKVGlCRFJWSlVTVVpKUTBGVVJTMHRMUzB0Q2sxSlNVSklla05DTUhGLi4u
    key: TFMwdExTMUNSVWRKVGlCRlJESTFOVEU1SUZCU1NWWkJWRVVnUzBWWkxTMHRMUzBLVFVNLi4u
```


</div>

<div class="dd">

<code>apiServer</code>  <i><a href="#apiserverconfig">APIServerConfig</a></i>

</div>
<div class="dt">

API server specific configuration options.



Examples:


``` yaml
apiServer:
    image: k8s.gcr.io/kube-apiserver-amd64:v1.19.3 # The container image used in the API server manifest.
    # Extra arguments to supply to the API server.
    extraArgs:
        key: value
    # Extra certificate subject alternative names for the API server's certificate.
    certSANs:
        - 1.2.3.4
        - 4.5.6.7
```


</div>

<div class="dd">

<code>controllerManager</code>  <i><a href="#controllermanagerconfig">ControllerManagerConfig</a></i>

</div>
<div class="dt">

Controller manager server specific configuration options.



Examples:


``` yaml
controllerManager:
    image: k8s.gcr.io/kube-controller-manager-amd64:v1.19.3 # The container image used in the controller manager manifest.
    # Extra arguments to supply to the controller manager.
    extraArgs:
        key: value
```


</div>

<div class="dd">

<code>proxy</code>  <i><a href="#proxyconfig">ProxyConfig</a></i>

</div>
<div class="dt">

Kube-proxy server-specific configuration options



Examples:


``` yaml
proxy:
    image: k8s.gcr.io/kube-proxy-amd64:v1.19.3 # The container image used in the kube-proxy manifest.
    mode: ipvs # proxy mode of kube-proxy.
    # Extra arguments to supply to kube-proxy.
    extraArgs:
        key: value
```


</div>

<div class="dd">

<code>scheduler</code>  <i><a href="#schedulerconfig">SchedulerConfig</a></i>

</div>
<div class="dt">

Scheduler server specific configuration options.



Examples:


``` yaml
scheduler:
    image: k8s.gcr.io/kube-scheduler-amd64:v1.19.3 # The container image used in the scheduler manifest.
    # Extra arguments to supply to the scheduler.
    extraArgs:
        key: value
```


</div>

<div class="dd">

<code>etcd</code>  <i><a href="#etcdconfig">EtcdConfig</a></i>

</div>
<div class="dt">

Etcd specific configuration options.



Examples:


``` yaml
etcd:
    image: gcr.io/etcd-development/etcd:v3.4.12 # The container image used to create the etcd service.
    # The `ca` is the root certificate authority of the PKI.
    ca:
        crt: TFMwdExTMUNSVWRKVGlCRFJWSlVTVVpKUTBGVVJTMHRMUzB0Q2sxSlNVSklla05DTUhGLi4u
        key: TFMwdExTMUNSVWRKVGlCRlJESTFOVEU1SUZCU1NWWkJWRVVnUzBWWkxTMHRMUzBLVFVNLi4u
    # Extra arguments to supply to etcd.
    extraArgs:
        key: value
```


</div>

<div class="dd">

<code>podCheckpointer</code>  <i><a href="#podcheckpointer">PodCheckpointer</a></i>

</div>
<div class="dt">

Pod Checkpointer specific configuration options.



Examples:


``` yaml
podCheckpointer:
    image: '...' # The `image` field is an override to the default pod-checkpointer image.
```


</div>

<div class="dd">

<code>coreDNS</code>  <i><a href="#coredns">CoreDNS</a></i>

</div>
<div class="dt">

Core DNS specific configuration options.



Examples:


``` yaml
coreDNS:
    image: k8s.gcr.io/coredns:1.7.0 # The `image` field is an override to the default coredns image.
```


</div>

<div class="dd">

<code>extraManifests</code>  <i>[]string</i>

</div>
<div class="dt">

A list of urls that point to additional manifests.
These will get automatically deployed by bootkube.



Examples:


``` yaml
extraManifests:
    - https://www.mysweethttpserver.com/manifest1.yaml
    - https://www.mysweethttpserver.com/manifest2.yaml
```


</div>

<div class="dd">

<code>extraManifestHeaders</code>  <i>map[string]string</i>

</div>
<div class="dt">

A map of key value pairs that will be added while fetching the ExtraManifests.



Examples:


``` yaml
extraManifestHeaders:
    Token: "1234567"
    X-ExtraInfo: info
```


</div>

<div class="dd">

<code>adminKubeconfig</code>  <i><a href="#adminkubeconfigconfig">AdminKubeconfigConfig</a></i>

</div>
<div class="dt">

Settings for admin kubeconfig generation.
Certificate lifetime can be configured.



Examples:


``` yaml
adminKubeconfig:
    certLifetime: 1h0m0s # Admin kubeconfig certificate lifetime (default is 1 year).
```


</div>

<div class="dd">

<code>allowSchedulingOnMasters</code>  <i>bool</i>

</div>
<div class="dt">

Indicates if master nodes are schedulable.


Valid values:


  - <code>true</code>

  - <code>yes</code>

  - <code>false</code>

  - <code>no</code>
</div>





---

## KubeletConfig
Appears in:


- <code><a href="#machineconfig">MachineConfig</a>.kubelet</code>


``` yaml
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

<div class="dd">

<code>image</code>  <i>string</i>

</div>
<div class="dt">

The `image` field is an optional reference to an alternative kubelet image.



Examples:


``` yaml
image: docker.io/autonomy/kubelet:v1.19.3
```


</div>

<div class="dd">

<code>extraArgs</code>  <i>map[string]string</i>

</div>
<div class="dt">

The `extraArgs` field is used to provide additional flags to the kubelet.



Examples:


``` yaml
extraArgs:
    key: value
```


</div>

<div class="dd">

<code>extraMounts</code>  <i>[]Mount</i>

</div>
<div class="dt">

The `extraMounts` field is used to add additional mounts to the kubelet container.



Examples:


``` yaml
extraMounts:
    - destination: /var/lib/example
      type: bind
      source: /var/lib/example
      options:
        - rshared
        - ro
```


</div>





---

## NetworkConfig
Appears in:


- <code><a href="#machineconfig">MachineConfig</a>.network</code>


``` yaml
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

<div class="dd">

<code>hostname</code>  <i>string</i>

</div>
<div class="dt">

Used to statically set the hostname for the host.

</div>

<div class="dd">

<code>interfaces</code>  <i>[]<a href="#device">Device</a></i>

</div>
<div class="dt">

`interfaces` is used to define the network interface configuration.
By default all network interfaces will attempt a DHCP discovery.
This can be further tuned through this configuration parameter.

#### machine.network.interfaces.interface

This is the interface name that should be configured.

#### machine.network.interfaces.cidr

`cidr` is used to specify a static IP address to the interface.
This should be in proper CIDR notation ( `192.168.2.5/24` ).

> Note: This option is mutually exclusive with DHCP.

#### machine.network.interfaces.dhcp

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

#### machine.network.interfaces.ignore

`ignore` is used to exclude a specific interface from configuration.
This parameter is optional.

#### machine.network.interfaces.dummy

`dummy` is used to specify that this interface should be a virtual-only, dummy interface.
This parameter is optional.

#### machine.network.interfaces.routes

`routes` is used to specify static routes that may be necessary.
This parameter is optional.

Routes can be repeated and includes a `Network` and `Gateway` field.

</div>

<div class="dd">

<code>nameservers</code>  <i>[]string</i>

</div>
<div class="dt">

Used to statically set the nameservers for the host.
Defaults to `1.1.1.1` and `8.8.8.8`

</div>

<div class="dd">

<code>extraHostEntries</code>  <i>[]<a href="#extrahost">ExtraHost</a></i>

</div>
<div class="dt">

Allows for extra entries to be added to /etc/hosts file



Examples:


``` yaml
extraHostEntries:
    - ip: 192.168.1.100 # The IP of the host.
      # The host alias.
      aliases:
        - test
        - test.domain.tld
```


</div>





---

## InstallConfig
Appears in:


- <code><a href="#machineconfig">MachineConfig</a>.install</code>


``` yaml
disk: /dev/sda # The disk used to install the bootloader, and ephemeral partitions.
# Allows for supplying extra kernel args to the bootloader config.
extraKernelArgs:
    - option=value
image: ghcr.io/talos-systems/installer:latest # Allows for supplying the image used to perform the installation.
bootloader: true # Indicates if a bootloader should be installed.
wipe: false # Indicates if zeroes should be written to the `disk` before performing and installation.
```

<div class="dd">

<code>disk</code>  <i>string</i>

</div>
<div class="dt">

The disk used to install the bootloader, and ephemeral partitions.



Examples:


``` yaml
disk: /dev/sda
```

``` yaml
disk: /dev/nvme0
```


</div>

<div class="dd">

<code>extraKernelArgs</code>  <i>[]string</i>

</div>
<div class="dt">

Allows for supplying extra kernel args to the bootloader config.



Examples:


``` yaml
extraKernelArgs:
    - a=b
```


</div>

<div class="dd">

<code>image</code>  <i>string</i>

</div>
<div class="dt">

Allows for supplying the image used to perform the installation.



Examples:


``` yaml
image: docker.io/<org>/installer:latest
```


</div>

<div class="dd">

<code>bootloader</code>  <i>bool</i>

</div>
<div class="dt">

Indicates if a bootloader should be installed.


Valid values:


  - <code>true</code>

  - <code>yes</code>

  - <code>false</code>

  - <code>no</code>
</div>

<div class="dd">

<code>wipe</code>  <i>bool</i>

</div>
<div class="dt">

Indicates if zeroes should be written to the `disk` before performing and installation.
Defaults to `true`.


Valid values:


  - <code>true</code>

  - <code>yes</code>

  - <code>false</code>

  - <code>no</code>
</div>





---

## TimeConfig
Appears in:


- <code><a href="#machineconfig">MachineConfig</a>.time</code>


``` yaml
disabled: false # Indicates if time (ntp) is disabled for the machine
# Specifies time (ntp) servers to use for setting system time.
servers:
    - time.cloudflare.com
```

<div class="dd">

<code>disabled</code>  <i>bool</i>

</div>
<div class="dt">

Indicates if time (ntp) is disabled for the machine
Defaults to `false`.

</div>

<div class="dd">

<code>servers</code>  <i>[]string</i>

</div>
<div class="dt">

Specifies time (ntp) servers to use for setting system time.
Defaults to `pool.ntp.org`

> Note: This parameter only supports a single time server

</div>





---

## RegistriesConfig
Appears in:


- <code><a href="#machineconfig">MachineConfig</a>.registries</code>


``` yaml
# Specifies mirror configuration for each registry.
mirrors:
    docker.io:
        # List of endpoints (URLs) for registry mirrors to use.
        endpoints:
            - https://registry.local
    ghcr.io:
        # List of endpoints (URLs) for registry mirrors to use.
        endpoints:
            - https://registry.insecure
            - https://ghcr.io/v2/
# Specifies TLS & auth configuration for HTTPS image registries.
config:
    registry.insecure:
        # The TLS configuration for this registry.
        tls:
            insecureSkipVerify: true # Skip TLS server certificate verification (not recommended).

            # # Enable mutual TLS authentication with the registry.
            # clientIdentity:
            #     crt: TFMwdExTMUNSVWRKVGlCRFJWSlVTVVpKUTBGVVJTMHRMUzB0Q2sxSlNVSklla05DTUhGLi4u
            #     key: TFMwdExTMUNSVWRKVGlCRlJESTFOVEU1SUZCU1NWWkJWRVVnUzBWWkxTMHRMUzBLVFVNLi4u
    registry.local:
        # The TLS configuration for this registry.
        tls:
            # Enable mutual TLS authentication with the registry.
            clientIdentity:
                crt: TFMwdExTMUNSVWRKVGlCRFJWSlVTVVpKUTBGVVJTMHRMUzB0Q2sxSlNVSklla05DTUhGLi4u
                key: TFMwdExTMUNSVWRKVGlCRlJESTFOVEU1SUZCU1NWWkJWRVVnUzBWWkxTMHRMUzBLVFVNLi4u
        # The auth configuration for this registry.
        auth:
            username: username # Optional registry authentication.
            password: password # Optional registry authentication.
```

<div class="dd">

<code>mirrors</code>  <i>map[string]<a href="#registrymirrorconfig">RegistryMirrorConfig</a></i>

</div>
<div class="dt">

Specifies mirror configuration for each registry.
This setting allows to use local pull-through caching registires,
air-gapped installations, etc.

Registry name is the first segment of image identifier, with 'docker.io'
being default one.
Name '*' catches any registry names not specified explicitly.

</div>

<div class="dd">

<code>config</code>  <i>map[string]<a href="#registryconfig">RegistryConfig</a></i>

</div>
<div class="dt">

Specifies TLS & auth configuration for HTTPS image registries.
Mutual TLS can be enabled with 'clientIdentity' option.

TLS configuration can be skipped if registry has trusted
server certificate.

</div>





---

## PodCheckpointer
Appears in:


- <code><a href="#clusterconfig">ClusterConfig</a>.podCheckpointer</code>


``` yaml
image: '...' # The `image` field is an override to the default pod-checkpointer image.
```

<div class="dd">

<code>image</code>  <i>string</i>

</div>
<div class="dt">

The `image` field is an override to the default pod-checkpointer image.

</div>





---

## CoreDNS
Appears in:


- <code><a href="#clusterconfig">ClusterConfig</a>.coreDNS</code>


``` yaml
image: k8s.gcr.io/coredns:1.7.0 # The `image` field is an override to the default coredns image.
```

<div class="dd">

<code>image</code>  <i>string</i>

</div>
<div class="dt">

The `image` field is an override to the default coredns image.

</div>





---

## Endpoint
Appears in:


- <code><a href="#controlplaneconfig">ControlPlaneConfig</a>.endpoint</code>


``` yaml
https://1.2.3.4:443
```



---

## ControlPlaneConfig
Appears in:


- <code><a href="#clusterconfig">ClusterConfig</a>.controlPlane</code>


``` yaml
endpoint: https://1.2.3.4 # Endpoint is the canonical controlplane endpoint, which can be an IP address or a DNS hostname.
localAPIServerPort: 443 # The port that the API server listens on internally.
```

<div class="dd">

<code>endpoint</code>  <i><a href="#endpoint">Endpoint</a></i>

</div>
<div class="dt">

Endpoint is the canonical controlplane endpoint, which can be an IP address or a DNS hostname.
It is single-valued, and may optionally include a port number.



Examples:


``` yaml
endpoint: https://1.2.3.4:443
```


</div>

<div class="dd">

<code>localAPIServerPort</code>  <i>int</i>

</div>
<div class="dt">

The port that the API server listens on internally.
This may be different than the port portion listed in the endpoint field above.
The default is 6443.

</div>





---

## APIServerConfig
Appears in:


- <code><a href="#clusterconfig">ClusterConfig</a>.apiServer</code>


``` yaml
image: k8s.gcr.io/kube-apiserver-amd64:v1.19.3 # The container image used in the API server manifest.
# Extra arguments to supply to the API server.
extraArgs:
    key: value
# Extra certificate subject alternative names for the API server's certificate.
certSANs:
    - 1.2.3.4
    - 4.5.6.7
```

<div class="dd">

<code>image</code>  <i>string</i>

</div>
<div class="dt">

The container image used in the API server manifest.

</div>

<div class="dd">

<code>extraArgs</code>  <i>map[string]string</i>

</div>
<div class="dt">

Extra arguments to supply to the API server.

</div>

<div class="dd">

<code>certSANs</code>  <i>[]string</i>

</div>
<div class="dt">

Extra certificate subject alternative names for the API server's certificate.

</div>





---

## ControllerManagerConfig
Appears in:


- <code><a href="#clusterconfig">ClusterConfig</a>.controllerManager</code>


``` yaml
image: k8s.gcr.io/kube-controller-manager-amd64:v1.19.3 # The container image used in the controller manager manifest.
# Extra arguments to supply to the controller manager.
extraArgs:
    key: value
```

<div class="dd">

<code>image</code>  <i>string</i>

</div>
<div class="dt">

The container image used in the controller manager manifest.

</div>

<div class="dd">

<code>extraArgs</code>  <i>map[string]string</i>

</div>
<div class="dt">

Extra arguments to supply to the controller manager.

</div>





---

## ProxyConfig
Appears in:


- <code><a href="#clusterconfig">ClusterConfig</a>.proxy</code>


``` yaml
image: k8s.gcr.io/kube-proxy-amd64:v1.19.3 # The container image used in the kube-proxy manifest.
mode: ipvs # proxy mode of kube-proxy.
# Extra arguments to supply to kube-proxy.
extraArgs:
    key: value
```

<div class="dd">

<code>image</code>  <i>string</i>

</div>
<div class="dt">

The container image used in the kube-proxy manifest.

</div>

<div class="dd">

<code>mode</code>  <i>string</i>

</div>
<div class="dt">

proxy mode of kube-proxy.
By default, this is 'iptables'.

</div>

<div class="dd">

<code>extraArgs</code>  <i>map[string]string</i>

</div>
<div class="dt">

Extra arguments to supply to kube-proxy.

</div>





---

## SchedulerConfig
Appears in:


- <code><a href="#clusterconfig">ClusterConfig</a>.scheduler</code>


``` yaml
image: k8s.gcr.io/kube-scheduler-amd64:v1.19.3 # The container image used in the scheduler manifest.
# Extra arguments to supply to the scheduler.
extraArgs:
    key: value
```

<div class="dd">

<code>image</code>  <i>string</i>

</div>
<div class="dt">

The container image used in the scheduler manifest.

</div>

<div class="dd">

<code>extraArgs</code>  <i>map[string]string</i>

</div>
<div class="dt">

Extra arguments to supply to the scheduler.

</div>





---

## EtcdConfig
Appears in:


- <code><a href="#clusterconfig">ClusterConfig</a>.etcd</code>


``` yaml
image: gcr.io/etcd-development/etcd:v3.4.12 # The container image used to create the etcd service.
# The `ca` is the root certificate authority of the PKI.
ca:
    crt: TFMwdExTMUNSVWRKVGlCRFJWSlVTVVpKUTBGVVJTMHRMUzB0Q2sxSlNVSklla05DTUhGLi4u
    key: TFMwdExTMUNSVWRKVGlCRlJESTFOVEU1SUZCU1NWWkJWRVVnUzBWWkxTMHRMUzBLVFVNLi4u
# Extra arguments to supply to etcd.
extraArgs:
    key: value
```

<div class="dd">

<code>image</code>  <i>string</i>

</div>
<div class="dt">

The container image used to create the etcd service.

</div>

<div class="dd">

<code>ca</code>  <i>PEMEncodedCertificateAndKey</i>

</div>
<div class="dt">

The `ca` is the root certificate authority of the PKI.
It is composed of a base64 encoded `crt` and `key`.



Examples:


``` yaml
ca:
    crt: TFMwdExTMUNSVWRKVGlCRFJWSlVTVVpKUTBGVVJTMHRMUzB0Q2sxSlNVSklla05DTUhGLi4u
    key: TFMwdExTMUNSVWRKVGlCRlJESTFOVEU1SUZCU1NWWkJWRVVnUzBWWkxTMHRMUzBLVFVNLi4u
```


</div>

<div class="dd">

<code>extraArgs</code>  <i>map[string]string</i>

</div>
<div class="dt">

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

</div>





---

## ClusterNetworkConfig
Appears in:


- <code><a href="#clusterconfig">ClusterConfig</a>.network</code>


``` yaml
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

<div class="dd">

<code>cni</code>  <i><a href="#cniconfig">CNIConfig</a></i>

</div>
<div class="dt">

The CNI used.
Composed of "name" and "url".
The "name" key only supports upstream bootkube options of "flannel" or "custom".
URLs is only used if name is equal to "custom".
URLs should point to a single yaml file that will get deployed.
Empty struct or any other name will default to bootkube's flannel.



Examples:


``` yaml
cni:
    name: custom # Name of CNI to use.
    # URLs containing manifests to apply for CNI.
    urls:
        - https://www.mysweethttpserver.com/supersecretcni.yaml
```


</div>

<div class="dd">

<code>dnsDomain</code>  <i>string</i>

</div>
<div class="dt">

The domain used by Kubernetes DNS.
The default is `cluster.local`



Examples:


``` yaml
dnsDomain: cluser.local
```


</div>

<div class="dd">

<code>podSubnets</code>  <i>[]string</i>

</div>
<div class="dt">

The pod subnet CIDR.



Examples:


``` yaml
podSubnets:
    - 10.244.0.0/16
```


</div>

<div class="dd">

<code>serviceSubnets</code>  <i>[]string</i>

</div>
<div class="dt">

The service subnet CIDR.



Examples:


``` yaml
serviceSubnets:
    - 10.96.0.0/12
```


</div>





---

## CNIConfig
Appears in:


- <code><a href="#clusternetworkconfig">ClusterNetworkConfig</a>.cni</code>


``` yaml
name: custom # Name of CNI to use.
# URLs containing manifests to apply for CNI.
urls:
    - https://www.mysweethttpserver.com/supersecretcni.yaml
```

<div class="dd">

<code>name</code>  <i>string</i>

</div>
<div class="dt">

Name of CNI to use.

</div>

<div class="dd">

<code>urls</code>  <i>[]string</i>

</div>
<div class="dt">

URLs containing manifests to apply for CNI.

</div>





---

## AdminKubeconfigConfig
Appears in:


- <code><a href="#clusterconfig">ClusterConfig</a>.adminKubeconfig</code>


``` yaml
certLifetime: 1h0m0s # Admin kubeconfig certificate lifetime (default is 1 year).
```

<div class="dd">

<code>certLifetime</code>  <i>Duration</i>

</div>
<div class="dt">

Admin kubeconfig certificate lifetime (default is 1 year).
Field format accepts any Go time.Duration format ('1h' for one hour, '10m' for ten minutes).

</div>





---

## MachineDisk
Appears in:


- <code><a href="#machineconfig">MachineConfig</a>.disks</code>


``` yaml
- device: /dev/sdb # The name of the disk to use.
  # A list of partitions to create on the disk.
  partitions:
    - size: 100000000 # This size of the partition in bytes.
      mountpoint: /var/mnt/extra # Where to mount the partition.
```

<div class="dd">

<code>device</code>  <i>string</i>

</div>
<div class="dt">

The name of the disk to use.

</div>

<div class="dd">

<code>partitions</code>  <i>[]<a href="#diskpartition">DiskPartition</a></i>

</div>
<div class="dt">

A list of partitions to create on the disk.

</div>





---

## DiskPartition
Appears in:


- <code><a href="#machinedisk">MachineDisk</a>.partitions</code>



<div class="dd">

<code>size</code>  <i>uint64</i>

</div>
<div class="dt">

This size of the partition in bytes.

</div>

<div class="dd">

<code>mountpoint</code>  <i>string</i>

</div>
<div class="dt">

Where to mount the partition.

</div>





---

## MachineFile
Appears in:


- <code><a href="#machineconfig">MachineConfig</a>.files</code>


``` yaml
- content: '...' # The contents of file.
  permissions: 438 # The file's permissions in octal.
  path: /tmp/file.txt # The path of the file.
  op: append # The operation to use
```

<div class="dd">

<code>content</code>  <i>string</i>

</div>
<div class="dt">

The contents of file.

</div>

<div class="dd">

<code>permissions</code>  <i>FileMode</i>

</div>
<div class="dt">

The file's permissions in octal.

</div>

<div class="dd">

<code>path</code>  <i>string</i>

</div>
<div class="dt">

The path of the file.

</div>

<div class="dd">

<code>op</code>  <i>string</i>

</div>
<div class="dt">

The operation to use


Valid values:


  - <code>create</code>

  - <code>append</code>
</div>





---

## ExtraHost
Appears in:


- <code><a href="#networkconfig">NetworkConfig</a>.extraHostEntries</code>


``` yaml
- ip: 192.168.1.100 # The IP of the host.
  # The host alias.
  aliases:
    - test
    - test.domain.tld
```

<div class="dd">

<code>ip</code>  <i>string</i>

</div>
<div class="dt">

The IP of the host.

</div>

<div class="dd">

<code>aliases</code>  <i>[]string</i>

</div>
<div class="dt">

The host alias.

</div>





---

## Device
Appears in:


- <code><a href="#networkconfig">NetworkConfig</a>.interfaces</code>



<div class="dd">

<code>interface</code>  <i>string</i>

</div>
<div class="dt">

The interface name.

</div>

<div class="dd">

<code>cidr</code>  <i>string</i>

</div>
<div class="dt">

The CIDR to use.

</div>

<div class="dd">

<code>routes</code>  <i>[]<a href="#route">Route</a></i>

</div>
<div class="dt">

A list of routes associated with the interface.
If used in combination with DHCP, these routes will be appended to routes returned by DHCP server.

</div>

<div class="dd">

<code>bond</code>  <i><a href="#bond">Bond</a></i>

</div>
<div class="dt">

Bond specific options.

</div>

<div class="dd">

<code>vlans</code>  <i>[]<a href="#vlan">Vlan</a></i>

</div>
<div class="dt">

VLAN specific options.

</div>

<div class="dd">

<code>mtu</code>  <i>int</i>

</div>
<div class="dt">

The interface's MTU.
If used in combination with DHCP, this will override any MTU settings returned from DHCP server.

</div>

<div class="dd">

<code>dhcp</code>  <i>bool</i>

</div>
<div class="dt">

Indicates if DHCP should be used.

</div>

<div class="dd">

<code>ignore</code>  <i>bool</i>

</div>
<div class="dt">

Indicates if the interface should be ignored.

</div>

<div class="dd">

<code>dummy</code>  <i>bool</i>

</div>
<div class="dt">

Indicates if the interface is a dummy interface.

</div>

<div class="dd">

<code>dhcpOptions</code>  <i><a href="#dhcpoptions">DHCPOptions</a></i>

</div>
<div class="dt">

DHCP specific options.
DHCP *must* be set to true for these to take effect.

</div>





---

## DHCPOptions
Appears in:


- <code><a href="#device">Device</a>.dhcpOptions</code>



<div class="dd">

<code>routeMetric</code>  <i>uint32</i>

</div>
<div class="dt">

The priority of all routes received via DHCP

</div>





---

## Bond
Appears in:


- <code><a href="#device">Device</a>.bond</code>



<div class="dd">

<code>interfaces</code>  <i>[]string</i>

</div>
<div class="dt">

The interfaces that make up the bond.

</div>

<div class="dd">

<code>arpIPTarget</code>  <i>[]string</i>

</div>
<div class="dt">

A bond option.
Please see the official kernel documentation.

</div>

<div class="dd">

<code>mode</code>  <i>string</i>

</div>
<div class="dt">

A bond option.
Please see the official kernel documentation.

</div>

<div class="dd">

<code>xmitHashPolicy</code>  <i>string</i>

</div>
<div class="dt">

A bond option.
Please see the official kernel documentation.

</div>

<div class="dd">

<code>lacpRate</code>  <i>string</i>

</div>
<div class="dt">

A bond option.
Please see the official kernel documentation.

</div>

<div class="dd">

<code>adActorSystem</code>  <i>string</i>

</div>
<div class="dt">

A bond option.
Please see the official kernel documentation.

</div>

<div class="dd">

<code>arpValidate</code>  <i>string</i>

</div>
<div class="dt">

A bond option.
Please see the official kernel documentation.

</div>

<div class="dd">

<code>arpAllTargets</code>  <i>string</i>

</div>
<div class="dt">

A bond option.
Please see the official kernel documentation.

</div>

<div class="dd">

<code>primary</code>  <i>string</i>

</div>
<div class="dt">

A bond option.
Please see the official kernel documentation.

</div>

<div class="dd">

<code>primaryReselect</code>  <i>string</i>

</div>
<div class="dt">

A bond option.
Please see the official kernel documentation.

</div>

<div class="dd">

<code>failOverMac</code>  <i>string</i>

</div>
<div class="dt">

A bond option.
Please see the official kernel documentation.

</div>

<div class="dd">

<code>adSelect</code>  <i>string</i>

</div>
<div class="dt">

A bond option.
Please see the official kernel documentation.

</div>

<div class="dd">

<code>miimon</code>  <i>uint32</i>

</div>
<div class="dt">

A bond option.
Please see the official kernel documentation.

</div>

<div class="dd">

<code>updelay</code>  <i>uint32</i>

</div>
<div class="dt">

A bond option.
Please see the official kernel documentation.

</div>

<div class="dd">

<code>downdelay</code>  <i>uint32</i>

</div>
<div class="dt">

A bond option.
Please see the official kernel documentation.

</div>

<div class="dd">

<code>arpInterval</code>  <i>uint32</i>

</div>
<div class="dt">

A bond option.
Please see the official kernel documentation.

</div>

<div class="dd">

<code>resendIgmp</code>  <i>uint32</i>

</div>
<div class="dt">

A bond option.
Please see the official kernel documentation.

</div>

<div class="dd">

<code>minLinks</code>  <i>uint32</i>

</div>
<div class="dt">

A bond option.
Please see the official kernel documentation.

</div>

<div class="dd">

<code>lpInterval</code>  <i>uint32</i>

</div>
<div class="dt">

A bond option.
Please see the official kernel documentation.

</div>

<div class="dd">

<code>packetsPerSlave</code>  <i>uint32</i>

</div>
<div class="dt">

A bond option.
Please see the official kernel documentation.

</div>

<div class="dd">

<code>numPeerNotif</code>  <i>uint8</i>

</div>
<div class="dt">

A bond option.
Please see the official kernel documentation.

</div>

<div class="dd">

<code>tlbDynamicLb</code>  <i>uint8</i>

</div>
<div class="dt">

A bond option.
Please see the official kernel documentation.

</div>

<div class="dd">

<code>allSlavesActive</code>  <i>uint8</i>

</div>
<div class="dt">

A bond option.
Please see the official kernel documentation.

</div>

<div class="dd">

<code>useCarrier</code>  <i>bool</i>

</div>
<div class="dt">

A bond option.
Please see the official kernel documentation.

</div>

<div class="dd">

<code>adActorSysPrio</code>  <i>uint16</i>

</div>
<div class="dt">

A bond option.
Please see the official kernel documentation.

</div>

<div class="dd">

<code>adUserPortKey</code>  <i>uint16</i>

</div>
<div class="dt">

A bond option.
Please see the official kernel documentation.

</div>

<div class="dd">

<code>peerNotifyDelay</code>  <i>uint32</i>

</div>
<div class="dt">

A bond option.
Please see the official kernel documentation.

</div>





---

## Vlan
Appears in:


- <code><a href="#device">Device</a>.vlans</code>



<div class="dd">

<code>cidr</code>  <i>string</i>

</div>
<div class="dt">

The CIDR to use.

</div>

<div class="dd">

<code>routes</code>  <i>[]<a href="#route">Route</a></i>

</div>
<div class="dt">

A list of routes associated with the VLAN.

</div>

<div class="dd">

<code>dhcp</code>  <i>bool</i>

</div>
<div class="dt">

Indicates if DHCP should be used.

</div>

<div class="dd">

<code>vlanId</code>  <i>uint16</i>

</div>
<div class="dt">

The VLAN's ID.

</div>





---

## Route
Appears in:


- <code><a href="#device">Device</a>.routes</code>

- <code><a href="#vlan">Vlan</a>.routes</code>



<div class="dd">

<code>network</code>  <i>string</i>

</div>
<div class="dt">

The route's network.

</div>

<div class="dd">

<code>gateway</code>  <i>string</i>

</div>
<div class="dt">

The route's gateway.

</div>





---

## RegistryMirrorConfig
Appears in:


- <code><a href="#registriesconfig">RegistriesConfig</a>.mirrors</code>



<div class="dd">

<code>endpoints</code>  <i>[]string</i>

</div>
<div class="dt">

List of endpoints (URLs) for registry mirrors to use.
Endpoint configures HTTP/HTTPS access mode, host name,
port and path (if path is not set, it defaults to `/v2`).

</div>





---

## RegistryConfig
Appears in:


- <code><a href="#registriesconfig">RegistriesConfig</a>.config</code>



<div class="dd">

<code>tls</code>  <i><a href="#registrytlsconfig">RegistryTLSConfig</a></i>

</div>
<div class="dt">

The TLS configuration for this registry.

</div>

<div class="dd">

<code>auth</code>  <i><a href="#registryauthconfig">RegistryAuthConfig</a></i>

</div>
<div class="dt">

The auth configuration for this registry.

</div>





---

## RegistryAuthConfig
Appears in:


- <code><a href="#registryconfig">RegistryConfig</a>.auth</code>



<div class="dd">

<code>username</code>  <i>string</i>

</div>
<div class="dt">

Optional registry authentication.
The meaning of each field is the same with the corresponding field in .docker/config.json.

</div>

<div class="dd">

<code>password</code>  <i>string</i>

</div>
<div class="dt">

Optional registry authentication.
The meaning of each field is the same with the corresponding field in .docker/config.json.

</div>

<div class="dd">

<code>auth</code>  <i>string</i>

</div>
<div class="dt">

Optional registry authentication.
The meaning of each field is the same with the corresponding field in .docker/config.json.

</div>

<div class="dd">

<code>identityToken</code>  <i>string</i>

</div>
<div class="dt">

Optional registry authentication.
The meaning of each field is the same with the corresponding field in .docker/config.json.

</div>





---

## RegistryTLSConfig
Appears in:


- <code><a href="#registryconfig">RegistryConfig</a>.tls</code>



<div class="dd">

<code>clientIdentity</code>  <i>PEMEncodedCertificateAndKey</i>

</div>
<div class="dt">

Enable mutual TLS authentication with the registry.
Client certificate and key should be base64-encoded.



Examples:


``` yaml
clientIdentity:
    crt: TFMwdExTMUNSVWRKVGlCRFJWSlVTVVpKUTBGVVJTMHRMUzB0Q2sxSlNVSklla05DTUhGLi4u
    key: TFMwdExTMUNSVWRKVGlCRlJESTFOVEU1SUZCU1NWWkJWRVVnUzBWWkxTMHRMUzBLVFVNLi4u
```


</div>

<div class="dd">

<code>ca</code>  <i>Base64Bytes</i>

</div>
<div class="dt">

CA registry certificate to add the list of trusted certificates.
Certificate should be base64-encoded.

</div>

<div class="dd">

<code>insecureSkipVerify</code>  <i>bool</i>

</div>
<div class="dt">

Skip TLS server certificate verification (not recommended).

</div>





---
