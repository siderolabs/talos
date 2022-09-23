---
title: Configuration
desription: Talos node configuration file reference.
---

<!-- markdownlint-disable -->


Package v1alpha1 configuration file contains all the options available for configuring a machine.

To generate a set of basic configuration files, run:

	talosctl gen config --version v1alpha1 <cluster name> <cluster endpoint>

This will generate a machine config for each node type, and a talosconfig for the CLI.

---
## Config
Config defines the v1alpha1 configuration file.




{{< highlight yaml >}}
version: v1alpha1
persist: true
machine: # ...
cluster: # ...
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`version` |string |Indicates the schema used to decode the contents.  |`v1alpha1`<br /> |
|`debug` |bool |<details><summary>Enable verbose logging to the console.</summary>All system containers logs will flow into serial console.<br /><br />**Note:** To avoid breaking Talos bootstrap flow enable this option only if serial console can handle high message throughput.</details>  |`true`<br />`yes`<br />`false`<br />`no`<br /> |
|`persist` |bool |Indicates whether to pull the machine config upon every boot.  |`true`<br />`yes`<br />`false`<br />`no`<br /> |
|`machine` |<a href="#machineconfig">MachineConfig</a> |Provides machine specific configuration options.  | |
|`cluster` |<a href="#clusterconfig">ClusterConfig</a> |Provides cluster specific configuration options.  | |



---
## MachineConfig
MachineConfig represents the machine-specific config values.

Appears in:

- <code><a href="#config">Config</a>.machine</code>



{{< highlight yaml >}}
type: controlplane
# InstallConfig represents the installation options for preparing a node.
install:
    disk: /dev/sda # The disk used for installations.
    # Allows for supplying extra kernel args via the bootloader.
    extraKernelArgs:
        - console=ttyS1
        - panic=10
    image: ghcr.io/siderolabs/installer:latest # Allows for supplying the image used to perform the installation.
    bootloader: true # Indicates if a bootloader should be installed.
    wipe: false # Indicates if the installation disk should be wiped at installation time.

    # # Look up disk using disk attributes like model, size, serial and others.
    # diskSelector:
    #     size: 4GB # Disk size.
    #     model: WDC* # Disk model `/sys/block/<dev>/device/model`.
    #     busPath: /pci0000:00/0000:00:17.0/ata1/host0/target0:0:0/0:0:0:0 # Disk bus path.

    # # Allows for supplying additional system extension images to install on top of base Talos image.
    # extensions:
    #     - image: ghcr.io/siderolabs/gvisor:20220117.0-v1.0.0 # System extension image.
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`type` |string |<details><summary>Defines the role of the machine within the cluster.</summary><br />**Control Plane**<br /><br />Control Plane node type designates the node as a control plane member.<br />This means it will host etcd along with the Kubernetes controlplane components such as API Server, Controller Manager, Scheduler.<br /><br />**Worker**<br /><br />Worker node type designates the node as a worker node.<br />This means it will be an available compute node for scheduling workloads.<br /><br />This node type was previously known as "join"; that value is still supported but deprecated.</details>  |`controlplane`<br />`worker`<br /> |
|`token` |string |<details><summary>The `token` is used by a machine to join the PKI of the cluster.</summary>Using this token, a machine will create a certificate signing request (CSR), and request a certificate that will be used as its' identity.</details> <details><summary>Show example(s)</summary>{{< highlight yaml >}}
token: 328hom.uqjzh6jnn2eie9oi
{{< /highlight >}}</details> | |
|`ca` |PEMEncodedCertificateAndKey |<details><summary>The root certificate authority of the PKI.</summary>It is composed of a base64 encoded `crt` and `key`.</details> <details><summary>Show example(s)</summary>{{< highlight yaml >}}
ca:
    crt: LS0tIEVYQU1QTEUgQ0VSVElGSUNBVEUgLS0t
    key: LS0tIEVYQU1QTEUgS0VZIC0tLQ==
{{< /highlight >}}</details> | |
|`certSANs` |[]string |<details><summary>Extra certificate subject alternative names for the machine's certificate.</summary>By default, all non-loopback interface IPs are automatically added to the certificate's SANs.</details> <details><summary>Show example(s)</summary>{{< highlight yaml >}}
certSANs:
    - 10.0.0.10
    - 172.16.0.10
    - 192.168.0.10
{{< /highlight >}}</details> | |
|`controlPlane` |<a href="#machinecontrolplaneconfig">MachineControlPlaneConfig</a> |Provides machine specific control plane configuration options. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
controlPlane:
    # Controller manager machine specific configuration options.
    controllerManager:
        disabled: false # Disable kube-controller-manager on the node.
    # Scheduler machine specific configuration options.
    scheduler:
        disabled: true # Disable kube-scheduler on the node.
{{< /highlight >}}</details> | |
|`kubelet` |<a href="#kubeletconfig">KubeletConfig</a> |Used to provide additional options to the kubelet. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
kubelet:
    image: ghcr.io/siderolabs/kubelet:v1.26.0-alpha.1 # The `image` field is an optional reference to an alternative kubelet image.
    # The `extraArgs` field is used to provide additional flags to the kubelet.
    extraArgs:
        feature-gates: ServerSideApply=true

    # # The `ClusterDNS` field is an optional reference to an alternative kubelet clusterDNS ip list.
    # clusterDNS:
    #     - 10.96.0.10
    #     - 169.254.2.53

    # # The `extraMounts` field is used to add additional mounts to the kubelet container.
    # extraMounts:
    #     - destination: /var/lib/example
    #       type: bind
    #       source: /var/lib/example
    #       options:
    #         - bind
    #         - rshared
    #         - rw

    # # The `extraConfig` field is used to provide kubelet configuration overrides.
    # extraConfig:
    #     serverTLSBootstrap: true

    # # The `nodeIP` field is used to configure `--node-ip` flag for the kubelet.
    # nodeIP:
    #     # The `validSubnets` field configures the networks to pick kubelet node IP from.
    #     validSubnets:
    #         - 10.0.0.0/8
    #         - '!10.0.0.3/32'
    #         - fdc7::/16
{{< /highlight >}}</details> | |
|`pods` |[]Unstructured |<details><summary>Used to provide static pod definitions to be run by the kubelet directly bypassing the kube-apiserver.</summary><br />Static pods can be used to run components which should be started before the Kubernetes control plane is up.<br />Talos doesn't validate the pod definition.<br />Updates to this field can be applied without a reboot.<br /><br />See https://kubernetes.io/docs/tasks/configure-pod-container/static-pod/.</details> <details><summary>Show example(s)</summary>{{< highlight yaml >}}
pods:
    - apiVersion: v1
      kind: pod
      metadata:
        name: nginx
      spec:
        containers:
            - image: nginx
              name: nginx
{{< /highlight >}}</details> | |
|`network` |<a href="#networkconfig">NetworkConfig</a> |Provides machine specific network configuration options. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
network:
    hostname: worker-1 # Used to statically set the hostname for the machine.
    # `interfaces` is used to define the network interface configuration.
    interfaces:
        - interface: eth0 # The interface name.
          # Assigns static IP addresses to the interface.
          addresses:
            - 192.168.2.0/24
          # A list of routes associated with the interface.
          routes:
            - network: 0.0.0.0/0 # The route's network (destination).
              gateway: 192.168.2.1 # The route's gateway (if empty, creates link scope route).
              metric: 1024 # The optional metric for the route.
          mtu: 1500 # The interface's MTU.

          # # Picks a network device using the selector.

          # # select a device with bus prefix 00:*.
          # deviceSelector:
          #     busPath: 00:* # PCI, USB bus prefix, supports matching by wildcard.
          # # select a device with mac address matching `*:f0:ab` and `virtio` kernel driver.
          # deviceSelector:
          #     hardwareAddr: '*:f0:ab' # Device hardware address, supports matching by wildcard.
          #     driver: virtio # Kernel driver, supports matching by wildcard.

          # # Bond specific options.
          # bond:
          #     # The interfaces that make up the bond.
          #     interfaces:
          #         - eth0
          #         - eth1
          #     mode: 802.3ad # A bond option.
          #     lacpRate: fast # A bond option.

          # # Bridge specific options.
          # bridge:
          #     # The interfaces that make up the bridge.
          #     interfaces:
          #         - eth0
          #         - eth1
          #     # A bridge option.
          #     stp:
          #         enabled: true # Whether Spanning Tree Protocol (STP) is enabled.

          # # Indicates if DHCP should be used to configure the interface.
          # dhcp: true

          # # DHCP specific options.
          # dhcpOptions:
          #     routeMetric: 1024 # The priority of all routes received via DHCP.

          # # Wireguard specific configuration.

          # # wireguard server example
          # wireguard:
          #     privateKey: ABCDEF... # Specifies a private key configuration (base64 encoded).
          #     listenPort: 51111 # Specifies a device's listening port.
          #     # Specifies a list of peer configurations to apply to a device.
          #     peers:
          #         - publicKey: ABCDEF... # Specifies the public key of this peer.
          #           endpoint: 192.168.1.3 # Specifies the endpoint of this peer entry.
          #           # AllowedIPs specifies a list of allowed IP addresses in CIDR notation for this peer.
          #           allowedIPs:
          #             - 192.168.1.0/24
          # # wireguard peer example
          # wireguard:
          #     privateKey: ABCDEF... # Specifies a private key configuration (base64 encoded).
          #     # Specifies a list of peer configurations to apply to a device.
          #     peers:
          #         - publicKey: ABCDEF... # Specifies the public key of this peer.
          #           endpoint: 192.168.1.2 # Specifies the endpoint of this peer entry.
          #           persistentKeepaliveInterval: 10s # Specifies the persistent keepalive interval for this peer.
          #           # AllowedIPs specifies a list of allowed IP addresses in CIDR notation for this peer.
          #           allowedIPs:
          #             - 192.168.1.0/24

          # # Virtual (shared) IP address configuration.

          # # layer2 vip example
          # vip:
          #     ip: 172.16.199.55 # Specifies the IP address to be used.
    # Used to statically set the nameservers for the machine.
    nameservers:
        - 9.8.7.6
        - 8.7.6.5

    # # Allows for extra entries to be added to the `/etc/hosts` file
    # extraHostEntries:
    #     - ip: 192.168.1.100 # The IP of the host.
    #       # The host alias.
    #       aliases:
    #         - example
    #         - example.domain.tld

    # # Configures KubeSpan feature.
    # kubespan:
    #     enabled: true # Enable the KubeSpan feature.
{{< /highlight >}}</details> | |
|`disks` |[]<a href="#machinedisk">MachineDisk</a> |<details><summary>Used to partition, format and mount additional disks.</summary>Since the rootfs is read only with the exception of `/var`, mounts are only valid if they are under `/var`.<br />Note that the partitioning and formating is done only once, if and only if no existing partitions are found.<br />If `size:` is omitted, the partition is sized to occupy the full disk.</details> <details><summary>Show example(s)</summary>{{< highlight yaml >}}
disks:
    - device: /dev/sdb # The name of the disk to use.
      # A list of partitions to create on the disk.
      partitions:
        - mountpoint: /var/mnt/extra # Where to mount the partition.

          # # The size of partition: either bytes or human readable representation. If `size:` is omitted, the partition is sized to occupy the full disk.

          # # Human readable representation.
          # size: 100 MB
          # # Precise value in bytes.
          # size: 1073741824
{{< /highlight >}}</details> | |
|`install` |<a href="#installconfig">InstallConfig</a> |Used to provide instructions for installations. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
install:
    disk: /dev/sda # The disk used for installations.
    # Allows for supplying extra kernel args via the bootloader.
    extraKernelArgs:
        - console=ttyS1
        - panic=10
    image: ghcr.io/siderolabs/installer:latest # Allows for supplying the image used to perform the installation.
    bootloader: true # Indicates if a bootloader should be installed.
    wipe: false # Indicates if the installation disk should be wiped at installation time.

    # # Look up disk using disk attributes like model, size, serial and others.
    # diskSelector:
    #     size: 4GB # Disk size.
    #     model: WDC* # Disk model `/sys/block/<dev>/device/model`.
    #     busPath: /pci0000:00/0000:00:17.0/ata1/host0/target0:0:0/0:0:0:0 # Disk bus path.

    # # Allows for supplying additional system extension images to install on top of base Talos image.
    # extensions:
    #     - image: ghcr.io/siderolabs/gvisor:20220117.0-v1.0.0 # System extension image.
{{< /highlight >}}</details> | |
|`files` |[]<a href="#machinefile">MachineFile</a> |<details><summary>Allows the addition of user specified files.</summary>The value of `op` can be `create`, `overwrite`, or `append`.<br />In the case of `create`, `path` must not exist.<br />In the case of `overwrite`, and `append`, `path` must be a valid file.<br />If an `op` value of `append` is used, the existing file will be appended.<br />Note that the file contents are not required to be base64 encoded.</details> <details><summary>Show example(s)</summary>{{< highlight yaml >}}
files:
    - content: '...' # The contents of the file.
      permissions: 0o666 # The file's permissions in octal.
      path: /tmp/file.txt # The path of the file.
      op: append # The operation to use
{{< /highlight >}}</details> | |
|`env` |Env |<details><summary>The `env` field allows for the addition of environment variables.</summary>All environment variables are set on PID 1 in addition to every service.</details> <details><summary>Show example(s)</summary>{{< highlight yaml >}}
env:
    GRPC_GO_LOG_SEVERITY_LEVEL: info
    GRPC_GO_LOG_VERBOSITY_LEVEL: "99"
    https_proxy: http://SERVER:PORT/
{{< /highlight >}}{{< highlight yaml >}}
env:
    GRPC_GO_LOG_SEVERITY_LEVEL: error
    https_proxy: https://USERNAME:PASSWORD@SERVER:PORT/
{{< /highlight >}}{{< highlight yaml >}}
env:
    https_proxy: http://DOMAIN\USERNAME:PASSWORD@SERVER:PORT/
{{< /highlight >}}</details> |``GRPC_GO_LOG_VERBOSITY_LEVEL``<br />``GRPC_GO_LOG_SEVERITY_LEVEL``<br />``http_proxy``<br />``https_proxy``<br />``no_proxy``<br /> |
|`time` |<a href="#timeconfig">TimeConfig</a> |Used to configure the machine's time settings. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
time:
    disabled: false # Indicates if the time service is disabled for the machine.
    # Specifies time (NTP) servers to use for setting the system time.
    servers:
        - time.cloudflare.com
    bootTimeout: 2m0s # Specifies the timeout when the node time is considered to be in sync unlocking the boot sequence.
{{< /highlight >}}</details> | |
|`sysctls` |map[string]string |Used to configure the machine's sysctls. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
sysctls:
    kernel.domainname: talos.dev
    net.ipv4.ip_forward: "0"
{{< /highlight >}}</details> | |
|`sysfs` |map[string]string |Used to configure the machine's sysfs. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
sysfs:
    devices.system.cpu.cpu0.cpufreq.scaling_governor: performance
{{< /highlight >}}</details> | |
|`registries` |<a href="#registriesconfig">RegistriesConfig</a> |<details><summary>Used to configure the machine's container image registry mirrors.</summary><br />Automatically generates matching CRI configuration for registry mirrors.<br /><br />The `mirrors` section allows to redirect requests for images to non-default registry,<br />which might be local registry or caching mirror.<br /><br />The `config` section provides a way to authenticate to the registry with TLS client<br />identity, provide registry CA, or authentication information.<br />Authentication information has same meaning with the corresponding field in `.docker/config.json`.<br /><br />See also matching configuration for [CRI containerd plugin](https://github.com/containerd/cri/blob/master/docs/registry.md).</details> <details><summary>Show example(s)</summary>{{< highlight yaml >}}
registries:
    # Specifies mirror configuration for each registry.
    mirrors:
        docker.io:
            # List of endpoints (URLs) for registry mirrors to use.
            endpoints:
                - https://registry.local
    # Specifies TLS & auth configuration for HTTPS image registries.
    config:
        registry.local:
            # The TLS configuration for the registry.
            tls:
                # Enable mutual TLS authentication with the registry.
                clientIdentity:
                    crt: LS0tIEVYQU1QTEUgQ0VSVElGSUNBVEUgLS0t
                    key: LS0tIEVYQU1QTEUgS0VZIC0tLQ==
            # The auth configuration for this registry.
            auth:
                username: username # Optional registry authentication.
                password: password # Optional registry authentication.
{{< /highlight >}}</details> | |
|`systemDiskEncryption` |<a href="#systemdiskencryptionconfig">SystemDiskEncryptionConfig</a> |<details><summary>Machine system disk encryption configuration.</summary>Defines each system partition encryption parameters.</details> <details><summary>Show example(s)</summary>{{< highlight yaml >}}
systemDiskEncryption:
    # Ephemeral partition encryption.
    ephemeral:
        provider: luks2 # Encryption provider to use for the encryption.
        # Defines the encryption keys generation and storage method.
        keys:
            - # Deterministically generated key from the node UUID and PartitionLabel.
              nodeID: {}
              slot: 0 # Key slot number for LUKS2 encryption.

        # # Cipher kind to use for the encryption. Depends on the encryption provider.
        # cipher: aes-xts-plain64

        # # Defines the encryption sector size.
        # blockSize: 4096

        # # Additional --perf parameters for the LUKS2 encryption.
        # options:
        #     - no_read_workqueue
        #     - no_write_workqueue
{{< /highlight >}}</details> | |
|`features` |<a href="#featuresconfig">FeaturesConfig</a> |Features describe individual Talos features that can be switched on or off. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
features:
    rbac: true # Enable role-based access control (RBAC).

    # # Configure Talos API access from Kubernetes pods.
    # kubernetesTalosAPIAccess:
    #     enabled: true # Enable Talos API access from Kubernetes pods.
    #     # The list of Talos API roles which can be granted for access from Kubernetes pods.
    #     allowedRoles:
    #         - os:reader
    #     # The list of Kubernetes namespaces Talos API access is available from.
    #     allowedKubernetesNamespaces:
    #         - kube-system
{{< /highlight >}}</details> | |
|`udev` |<a href="#udevconfig">UdevConfig</a> |Configures the udev system. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
udev:
    # List of udev rules to apply to the udev system
    rules:
        - SUBSYSTEM=="drm", KERNEL=="renderD*", GROUP="44", MODE="0660"
{{< /highlight >}}</details> | |
|`logging` |<a href="#loggingconfig">LoggingConfig</a> |Configures the logging system. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
logging:
    # Logging destination.
    destinations:
        - endpoint: tcp://1.2.3.4:12345 # Where to send logs. Supported protocols are "tcp" and "udp".
          format: json_lines # Logs format.
{{< /highlight >}}</details> | |
|`kernel` |<a href="#kernelconfig">KernelConfig</a> |Configures the kernel. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
kernel:
    # Kernel modules to load.
    modules:
        - name: brtfs # Module name.
{{< /highlight >}}</details> | |
|`seccompProfiles` |[]<a href="#machineseccompprofile">MachineSeccompProfile</a> |Configures the seccomp profiles for the machine. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
seccompProfiles:
    - name: audit.json # The `name` field is used to provide the file name of the seccomp profile.
      # The `value` field is used to provide the seccomp profile.
      value:
        defaultAction: SCMP_ACT_LOG
{{< /highlight >}}</details> | |



---
## MachineSeccompProfile
MachineSeccompProfile defines seccomp profiles for the machine.

Appears in:

- <code><a href="#machineconfig">MachineConfig</a>.seccompProfiles</code>



{{< highlight yaml >}}
- name: audit.json # The `name` field is used to provide the file name of the seccomp profile.
  # The `value` field is used to provide the seccomp profile.
  value:
    defaultAction: SCMP_ACT_LOG
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`name` |string |The `name` field is used to provide the file name of the seccomp profile.  | |
|`value` |Unstructured |The `value` field is used to provide the seccomp profile.  | |



---
## ClusterConfig
ClusterConfig represents the cluster-wide config values.

Appears in:

- <code><a href="#config">Config</a>.cluster</code>



{{< highlight yaml >}}
# ControlPlaneConfig represents the control plane configuration options.
controlPlane:
    endpoint: https://1.2.3.4 # Endpoint is the canonical controlplane endpoint, which can be an IP address or a DNS hostname.
    localAPIServerPort: 443 # The port that the API server listens on internally.
clusterName: talos.local
# ClusterNetworkConfig represents kube networking configuration options.
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
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`id` |string |Globally unique identifier for this cluster (base64 encoded random 32 bytes).  | |
|`secret` |string |<details><summary>Shared secret of cluster (base64 encoded random 32 bytes).</summary>This secret is shared among cluster members but should never be sent over the network.</details>  | |
|`controlPlane` |<a href="#controlplaneconfig">ControlPlaneConfig</a> |Provides control plane specific configuration options. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
controlPlane:
    endpoint: https://1.2.3.4 # Endpoint is the canonical controlplane endpoint, which can be an IP address or a DNS hostname.
    localAPIServerPort: 443 # The port that the API server listens on internally.
{{< /highlight >}}</details> | |
|`clusterName` |string |Configures the cluster's name.  | |
|`network` |<a href="#clusternetworkconfig">ClusterNetworkConfig</a> |Provides cluster specific network configuration options. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
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
{{< /highlight >}}</details> | |
|`token` |string |The [bootstrap token](https://kubernetes.io/docs/reference/access-authn-authz/bootstrap-tokens/) used to join the cluster. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
token: wlzjyw.bei2zfylhs2by0wd
{{< /highlight >}}</details> | |
|`aescbcEncryptionSecret` |string |The key used for the [encryption of secret data at rest](https://kubernetes.io/docs/tasks/administer-cluster/encrypt-data/). <details><summary>Show example(s)</summary>{{< highlight yaml >}}
aescbcEncryptionSecret: z01mye6j16bspJYtTB/5SFX8j7Ph4JXxM2Xuu4vsBPM=
{{< /highlight >}}</details> | |
|`ca` |PEMEncodedCertificateAndKey |The base64 encoded root certificate authority used by Kubernetes. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
ca:
    crt: LS0tIEVYQU1QTEUgQ0VSVElGSUNBVEUgLS0t
    key: LS0tIEVYQU1QTEUgS0VZIC0tLQ==
{{< /highlight >}}</details> | |
|`aggregatorCA` |PEMEncodedCertificateAndKey |<details><summary>The base64 encoded aggregator certificate authority used by Kubernetes for front-proxy certificate generation.</summary><br />This CA can be self-signed.</details> <details><summary>Show example(s)</summary>{{< highlight yaml >}}
aggregatorCA:
    crt: LS0tIEVYQU1QTEUgQ0VSVElGSUNBVEUgLS0t
    key: LS0tIEVYQU1QTEUgS0VZIC0tLQ==
{{< /highlight >}}</details> | |
|`serviceAccount` |PEMEncodedKey |The base64 encoded private key for service account token generation. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
serviceAccount:
    key: LS0tIEVYQU1QTEUgS0VZIC0tLQ==
{{< /highlight >}}</details> | |
|`apiServer` |<a href="#apiserverconfig">APIServerConfig</a> |API server specific configuration options. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
apiServer:
    image: k8s.gcr.io/kube-apiserver:v1.26.0-alpha.1 # The container image used in the API server manifest.
    # Extra arguments to supply to the API server.
    extraArgs:
        feature-gates: ServerSideApply=true
        http2-max-streams-per-connection: "32"
    # Extra certificate subject alternative names for the API server's certificate.
    certSANs:
        - 1.2.3.4
        - 4.5.6.7

    # # Configure the API server admission plugins.
    # admissionControl:
    #     - name: PodSecurity # Name is the name of the admission controller.
    #       # Configuration is an embedded configuration object to be used as the plugin's
    #       configuration:
    #         apiVersion: pod-security.admission.config.k8s.io/v1alpha1
    #         defaults:
    #             audit: restricted
    #             audit-version: latest
    #             enforce: baseline
    #             enforce-version: latest
    #             warn: restricted
    #             warn-version: latest
    #         exemptions:
    #             namespaces:
    #                 - kube-system
    #             runtimeClasses: []
    #             usernames: []
    #         kind: PodSecurityConfiguration

    # # Configure the API server audit policy.
    # auditPolicy:
    #     apiVersion: audit.k8s.io/v1
    #     kind: Policy
    #     rules:
    #         - level: Metadata
{{< /highlight >}}</details> | |
|`controllerManager` |<a href="#controllermanagerconfig">ControllerManagerConfig</a> |Controller manager server specific configuration options. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
controllerManager:
    image: k8s.gcr.io/kube-controller-manager:v1.26.0-alpha.1 # The container image used in the controller manager manifest.
    # Extra arguments to supply to the controller manager.
    extraArgs:
        feature-gates: ServerSideApply=true
{{< /highlight >}}</details> | |
|`proxy` |<a href="#proxyconfig">ProxyConfig</a> |Kube-proxy server-specific configuration options <details><summary>Show example(s)</summary>{{< highlight yaml >}}
proxy:
    image: k8s.gcr.io/kube-proxy:v1.26.0-alpha.1 # The container image used in the kube-proxy manifest.
    mode: ipvs # proxy mode of kube-proxy.
    # Extra arguments to supply to kube-proxy.
    extraArgs:
        proxy-mode: iptables

    # # Disable kube-proxy deployment on cluster bootstrap.
    # disabled: false
{{< /highlight >}}</details> | |
|`scheduler` |<a href="#schedulerconfig">SchedulerConfig</a> |Scheduler server specific configuration options. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
scheduler:
    image: k8s.gcr.io/kube-scheduler:v1.26.0-alpha.1 # The container image used in the scheduler manifest.
    # Extra arguments to supply to the scheduler.
    extraArgs:
        feature-gates: AllBeta=true
{{< /highlight >}}</details> | |
|`discovery` |<a href="#clusterdiscoveryconfig">ClusterDiscoveryConfig</a> |Configures cluster member discovery. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
discovery:
    enabled: true # Enable the cluster membership discovery feature.
    # Configure registries used for cluster member discovery.
    registries:
        # Kubernetes registry uses Kubernetes API server to discover cluster members and stores additional information
        kubernetes: {}
        # Service registry is using an external service to push and pull information about cluster members.
        service:
            endpoint: https://discovery.talos.dev/ # External service endpoint.
{{< /highlight >}}</details> | |
|`etcd` |<a href="#etcdconfig">EtcdConfig</a> |Etcd specific configuration options. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
etcd:
    image: gcr.io/etcd-development/etcd:v3.5.5 # The container image used to create the etcd service.
    # The `ca` is the root certificate authority of the PKI.
    ca:
        crt: LS0tIEVYQU1QTEUgQ0VSVElGSUNBVEUgLS0t
        key: LS0tIEVYQU1QTEUgS0VZIC0tLQ==
    # Extra arguments to supply to etcd.
    extraArgs:
        election-timeout: "5000"

    # # The `advertisedSubnets` field configures the networks to pick etcd advertised IP from.
    # advertisedSubnets:
    #     - 10.0.0.0/8
{{< /highlight >}}</details> | |
|`coreDNS` |<a href="#coredns">CoreDNS</a> |Core DNS specific configuration options. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
coreDNS:
    image: docker.io/coredns/coredns:1.10.0 # The `image` field is an override to the default coredns image.
{{< /highlight >}}</details> | |
|`externalCloudProvider` |<a href="#externalcloudproviderconfig">ExternalCloudProviderConfig</a> |External cloud provider configuration. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
externalCloudProvider:
    enabled: true # Enable external cloud provider.
    # A list of urls that point to additional manifests for an external cloud provider.
    manifests:
        - https://raw.githubusercontent.com/kubernetes/cloud-provider-aws/v1.20.0-alpha.0/manifests/rbac.yaml
        - https://raw.githubusercontent.com/kubernetes/cloud-provider-aws/v1.20.0-alpha.0/manifests/aws-cloud-controller-manager-daemonset.yaml
{{< /highlight >}}</details> | |
|`extraManifests` |[]string |<details><summary>A list of urls that point to additional manifests.</summary>These will get automatically deployed as part of the bootstrap.</details> <details><summary>Show example(s)</summary>{{< highlight yaml >}}
extraManifests:
    - https://www.example.com/manifest1.yaml
    - https://www.example.com/manifest2.yaml
{{< /highlight >}}</details> | |
|`extraManifestHeaders` |map[string]string |A map of key value pairs that will be added while fetching the extraManifests. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
extraManifestHeaders:
    Token: "1234567"
    X-ExtraInfo: info
{{< /highlight >}}</details> | |
|`inlineManifests` |ClusterInlineManifests |<details><summary>A list of inline Kubernetes manifests.</summary>These will get automatically deployed as part of the bootstrap.</details> <details><summary>Show example(s)</summary>{{< highlight yaml >}}
inlineManifests:
    - name: namespace-ci # Name of the manifest.
      contents: |- # Manifest contents as a string.
        apiVersion: v1
        kind: Namespace
        metadata:
        	name: ci
{{< /highlight >}}</details> | |
|`adminKubeconfig` |<a href="#adminkubeconfigconfig">AdminKubeconfigConfig</a> |<details><summary>Settings for admin kubeconfig generation.</summary>Certificate lifetime can be configured.</details> <details><summary>Show example(s)</summary>{{< highlight yaml >}}
adminKubeconfig:
    certLifetime: 1h0m0s # Admin kubeconfig certificate lifetime (default is 1 year).
{{< /highlight >}}</details> | |
|`allowSchedulingOnControlPlanes` |bool |Allows running workload on control-plane nodes.  |`true`<br />`yes`<br />`false`<br />`no`<br /> |



---
## ExtraMount
ExtraMount wraps OCI Mount specification.

Appears in:

- <code><a href="#kubeletconfig">KubeletConfig</a>.extraMounts</code>



{{< highlight yaml >}}
- destination: /var/lib/example
  type: bind
  source: /var/lib/example
  options:
    - bind
    - rshared
    - rw
{{< /highlight >}}




---
## MachineControlPlaneConfig
MachineControlPlaneConfig machine specific configuration options.

Appears in:

- <code><a href="#machineconfig">MachineConfig</a>.controlPlane</code>



{{< highlight yaml >}}
# Controller manager machine specific configuration options.
controllerManager:
    disabled: false # Disable kube-controller-manager on the node.
# Scheduler machine specific configuration options.
scheduler:
    disabled: true # Disable kube-scheduler on the node.
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`controllerManager` |<a href="#machinecontrollermanagerconfig">MachineControllerManagerConfig</a> |Controller manager machine specific configuration options.  | |
|`scheduler` |<a href="#machineschedulerconfig">MachineSchedulerConfig</a> |Scheduler machine specific configuration options.  | |



---
## MachineControllerManagerConfig
MachineControllerManagerConfig represents the machine specific ControllerManager config values.

Appears in:

- <code><a href="#machinecontrolplaneconfig">MachineControlPlaneConfig</a>.controllerManager</code>




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`disabled` |bool |Disable kube-controller-manager on the node.  | |



---
## MachineSchedulerConfig
MachineSchedulerConfig represents the machine specific Scheduler config values.

Appears in:

- <code><a href="#machinecontrolplaneconfig">MachineControlPlaneConfig</a>.scheduler</code>




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`disabled` |bool |Disable kube-scheduler on the node.  | |



---
## KubeletConfig
KubeletConfig represents the kubelet config values.

Appears in:

- <code><a href="#machineconfig">MachineConfig</a>.kubelet</code>



{{< highlight yaml >}}
image: ghcr.io/siderolabs/kubelet:v1.26.0-alpha.1 # The `image` field is an optional reference to an alternative kubelet image.
# The `extraArgs` field is used to provide additional flags to the kubelet.
extraArgs:
    feature-gates: ServerSideApply=true

# # The `ClusterDNS` field is an optional reference to an alternative kubelet clusterDNS ip list.
# clusterDNS:
#     - 10.96.0.10
#     - 169.254.2.53

# # The `extraMounts` field is used to add additional mounts to the kubelet container.
# extraMounts:
#     - destination: /var/lib/example
#       type: bind
#       source: /var/lib/example
#       options:
#         - bind
#         - rshared
#         - rw

# # The `extraConfig` field is used to provide kubelet configuration overrides.
# extraConfig:
#     serverTLSBootstrap: true

# # The `nodeIP` field is used to configure `--node-ip` flag for the kubelet.
# nodeIP:
#     # The `validSubnets` field configures the networks to pick kubelet node IP from.
#     validSubnets:
#         - 10.0.0.0/8
#         - '!10.0.0.3/32'
#         - fdc7::/16
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`image` |string |The `image` field is an optional reference to an alternative kubelet image. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
image: ghcr.io/siderolabs/kubelet:v1.26.0-alpha.1
{{< /highlight >}}</details> | |
|`clusterDNS` |[]string |The `ClusterDNS` field is an optional reference to an alternative kubelet clusterDNS ip list. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
clusterDNS:
    - 10.96.0.10
    - 169.254.2.53
{{< /highlight >}}</details> | |
|`extraArgs` |map[string]string |The `extraArgs` field is used to provide additional flags to the kubelet. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
extraArgs:
    key: value
{{< /highlight >}}</details> | |
|`extraMounts` |[]<a href="#extramount">ExtraMount</a> |<details><summary>The `extraMounts` field is used to add additional mounts to the kubelet container.</summary>Note that either `bind` or `rbind` are required in the `options`.</details> <details><summary>Show example(s)</summary>{{< highlight yaml >}}
extraMounts:
    - destination: /var/lib/example
      type: bind
      source: /var/lib/example
      options:
        - bind
        - rshared
        - rw
{{< /highlight >}}</details> | |
|`extraConfig` |Unstructured |<details><summary>The `extraConfig` field is used to provide kubelet configuration overrides.</summary><br />Some fields are not allowed to be overridden: authentication and authorization, cgroups<br />configuration, ports, etc.</details> <details><summary>Show example(s)</summary>{{< highlight yaml >}}
extraConfig:
    serverTLSBootstrap: true
{{< /highlight >}}</details> | |
|`defaultRuntimeSeccompProfileEnabled` |bool |Enable container runtime default Seccomp profile.  |`true`<br />`yes`<br />`false`<br />`no`<br /> |
|`registerWithFQDN` |bool |<details><summary>The `registerWithFQDN` field is used to force kubelet to use the node FQDN for registration.</summary>This is required in clouds like AWS.</details>  |`true`<br />`yes`<br />`false`<br />`no`<br /> |
|`nodeIP` |<a href="#kubeletnodeipconfig">KubeletNodeIPConfig</a> |<details><summary>The `nodeIP` field is used to configure `--node-ip` flag for the kubelet.</summary>This is used when a node has multiple addresses to choose from.</details> <details><summary>Show example(s)</summary>{{< highlight yaml >}}
nodeIP:
    # The `validSubnets` field configures the networks to pick kubelet node IP from.
    validSubnets:
        - 10.0.0.0/8
        - '!10.0.0.3/32'
        - fdc7::/16
{{< /highlight >}}</details> | |
|`skipNodeRegistration` |bool |<details><summary>The `skipNodeRegistration` is used to run the kubelet without registering with the apiserver.</summary>This runs kubelet as standalone and only runs static pods.</details>  |`true`<br />`yes`<br />`false`<br />`no`<br /> |



---
## KubeletNodeIPConfig
KubeletNodeIPConfig represents the kubelet node IP configuration.

Appears in:

- <code><a href="#kubeletconfig">KubeletConfig</a>.nodeIP</code>



{{< highlight yaml >}}
# The `validSubnets` field configures the networks to pick kubelet node IP from.
validSubnets:
    - 10.0.0.0/8
    - '!10.0.0.3/32'
    - fdc7::/16
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`validSubnets` |[]string |<details><summary>The `validSubnets` field configures the networks to pick kubelet node IP from.</summary>For dual stack configuration, there should be two subnets: one for IPv4, another for IPv6.<br />IPs can be excluded from the list by using negative match with `!`, e.g `!10.0.0.0/8`.<br />Negative subnet matches should be specified last to filter out IPs picked by positive matches.<br />If not specified, node IP is picked based on cluster podCIDRs: IPv4/IPv6 address or both.</details>  | |



---
## NetworkConfig
NetworkConfig represents the machine's networking config values.

Appears in:

- <code><a href="#machineconfig">MachineConfig</a>.network</code>



{{< highlight yaml >}}
hostname: worker-1 # Used to statically set the hostname for the machine.
# `interfaces` is used to define the network interface configuration.
interfaces:
    - interface: eth0 # The interface name.
      # Assigns static IP addresses to the interface.
      addresses:
        - 192.168.2.0/24
      # A list of routes associated with the interface.
      routes:
        - network: 0.0.0.0/0 # The route's network (destination).
          gateway: 192.168.2.1 # The route's gateway (if empty, creates link scope route).
          metric: 1024 # The optional metric for the route.
      mtu: 1500 # The interface's MTU.

      # # Picks a network device using the selector.

      # # select a device with bus prefix 00:*.
      # deviceSelector:
      #     busPath: 00:* # PCI, USB bus prefix, supports matching by wildcard.
      # # select a device with mac address matching `*:f0:ab` and `virtio` kernel driver.
      # deviceSelector:
      #     hardwareAddr: '*:f0:ab' # Device hardware address, supports matching by wildcard.
      #     driver: virtio # Kernel driver, supports matching by wildcard.

      # # Bond specific options.
      # bond:
      #     # The interfaces that make up the bond.
      #     interfaces:
      #         - eth0
      #         - eth1
      #     mode: 802.3ad # A bond option.
      #     lacpRate: fast # A bond option.

      # # Bridge specific options.
      # bridge:
      #     # The interfaces that make up the bridge.
      #     interfaces:
      #         - eth0
      #         - eth1
      #     # A bridge option.
      #     stp:
      #         enabled: true # Whether Spanning Tree Protocol (STP) is enabled.

      # # Indicates if DHCP should be used to configure the interface.
      # dhcp: true

      # # DHCP specific options.
      # dhcpOptions:
      #     routeMetric: 1024 # The priority of all routes received via DHCP.

      # # Wireguard specific configuration.

      # # wireguard server example
      # wireguard:
      #     privateKey: ABCDEF... # Specifies a private key configuration (base64 encoded).
      #     listenPort: 51111 # Specifies a device's listening port.
      #     # Specifies a list of peer configurations to apply to a device.
      #     peers:
      #         - publicKey: ABCDEF... # Specifies the public key of this peer.
      #           endpoint: 192.168.1.3 # Specifies the endpoint of this peer entry.
      #           # AllowedIPs specifies a list of allowed IP addresses in CIDR notation for this peer.
      #           allowedIPs:
      #             - 192.168.1.0/24
      # # wireguard peer example
      # wireguard:
      #     privateKey: ABCDEF... # Specifies a private key configuration (base64 encoded).
      #     # Specifies a list of peer configurations to apply to a device.
      #     peers:
      #         - publicKey: ABCDEF... # Specifies the public key of this peer.
      #           endpoint: 192.168.1.2 # Specifies the endpoint of this peer entry.
      #           persistentKeepaliveInterval: 10s # Specifies the persistent keepalive interval for this peer.
      #           # AllowedIPs specifies a list of allowed IP addresses in CIDR notation for this peer.
      #           allowedIPs:
      #             - 192.168.1.0/24

      # # Virtual (shared) IP address configuration.

      # # layer2 vip example
      # vip:
      #     ip: 172.16.199.55 # Specifies the IP address to be used.
# Used to statically set the nameservers for the machine.
nameservers:
    - 9.8.7.6
    - 8.7.6.5

# # Allows for extra entries to be added to the `/etc/hosts` file
# extraHostEntries:
#     - ip: 192.168.1.100 # The IP of the host.
#       # The host alias.
#       aliases:
#         - example
#         - example.domain.tld

# # Configures KubeSpan feature.
# kubespan:
#     enabled: true # Enable the KubeSpan feature.
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`hostname` |string |Used to statically set the hostname for the machine.  | |
|`interfaces` |[]<a href="#device">Device</a> |<details><summary>`interfaces` is used to define the network interface configuration.</summary>By default all network interfaces will attempt a DHCP discovery.<br />This can be further tuned through this configuration parameter.</details> <details><summary>Show example(s)</summary>{{< highlight yaml >}}
interfaces:
    - interface: eth0 # The interface name.
      # Assigns static IP addresses to the interface.
      addresses:
        - 192.168.2.0/24
      # A list of routes associated with the interface.
      routes:
        - network: 0.0.0.0/0 # The route's network (destination).
          gateway: 192.168.2.1 # The route's gateway (if empty, creates link scope route).
          metric: 1024 # The optional metric for the route.
      mtu: 1500 # The interface's MTU.

      # # Picks a network device using the selector.

      # # select a device with bus prefix 00:*.
      # deviceSelector:
      #     busPath: 00:* # PCI, USB bus prefix, supports matching by wildcard.
      # # select a device with mac address matching `*:f0:ab` and `virtio` kernel driver.
      # deviceSelector:
      #     hardwareAddr: '*:f0:ab' # Device hardware address, supports matching by wildcard.
      #     driver: virtio # Kernel driver, supports matching by wildcard.

      # # Bond specific options.
      # bond:
      #     # The interfaces that make up the bond.
      #     interfaces:
      #         - eth0
      #         - eth1
      #     mode: 802.3ad # A bond option.
      #     lacpRate: fast # A bond option.

      # # Bridge specific options.
      # bridge:
      #     # The interfaces that make up the bridge.
      #     interfaces:
      #         - eth0
      #         - eth1
      #     # A bridge option.
      #     stp:
      #         enabled: true # Whether Spanning Tree Protocol (STP) is enabled.

      # # Indicates if DHCP should be used to configure the interface.
      # dhcp: true

      # # DHCP specific options.
      # dhcpOptions:
      #     routeMetric: 1024 # The priority of all routes received via DHCP.

      # # Wireguard specific configuration.

      # # wireguard server example
      # wireguard:
      #     privateKey: ABCDEF... # Specifies a private key configuration (base64 encoded).
      #     listenPort: 51111 # Specifies a device's listening port.
      #     # Specifies a list of peer configurations to apply to a device.
      #     peers:
      #         - publicKey: ABCDEF... # Specifies the public key of this peer.
      #           endpoint: 192.168.1.3 # Specifies the endpoint of this peer entry.
      #           # AllowedIPs specifies a list of allowed IP addresses in CIDR notation for this peer.
      #           allowedIPs:
      #             - 192.168.1.0/24
      # # wireguard peer example
      # wireguard:
      #     privateKey: ABCDEF... # Specifies a private key configuration (base64 encoded).
      #     # Specifies a list of peer configurations to apply to a device.
      #     peers:
      #         - publicKey: ABCDEF... # Specifies the public key of this peer.
      #           endpoint: 192.168.1.2 # Specifies the endpoint of this peer entry.
      #           persistentKeepaliveInterval: 10s # Specifies the persistent keepalive interval for this peer.
      #           # AllowedIPs specifies a list of allowed IP addresses in CIDR notation for this peer.
      #           allowedIPs:
      #             - 192.168.1.0/24

      # # Virtual (shared) IP address configuration.

      # # layer2 vip example
      # vip:
      #     ip: 172.16.199.55 # Specifies the IP address to be used.
{{< /highlight >}}</details> | |
|`nameservers` |[]string |<details><summary>Used to statically set the nameservers for the machine.</summary>Defaults to `1.1.1.1` and `8.8.8.8`</details> <details><summary>Show example(s)</summary>{{< highlight yaml >}}
nameservers:
    - 8.8.8.8
    - 1.1.1.1
{{< /highlight >}}</details> | |
|`extraHostEntries` |[]<a href="#extrahost">ExtraHost</a> |Allows for extra entries to be added to the `/etc/hosts` file <details><summary>Show example(s)</summary>{{< highlight yaml >}}
extraHostEntries:
    - ip: 192.168.1.100 # The IP of the host.
      # The host alias.
      aliases:
        - example
        - example.domain.tld
{{< /highlight >}}</details> | |
|`kubespan` |<a href="#networkkubespan">NetworkKubeSpan</a> |Configures KubeSpan feature. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
kubespan:
    enabled: true # Enable the KubeSpan feature.
{{< /highlight >}}</details> | |
|`disableSearchDomain` |bool |<details><summary>Disable generating a default search domain in /etc/resolv.conf</summary>based on the machine hostname.<br />Defaults to `false`.</details>  |`true`<br />`yes`<br />`false`<br />`no`<br /> |



---
## InstallConfig
InstallConfig represents the installation options for preparing a node.

Appears in:

- <code><a href="#machineconfig">MachineConfig</a>.install</code>



{{< highlight yaml >}}
disk: /dev/sda # The disk used for installations.
# Allows for supplying extra kernel args via the bootloader.
extraKernelArgs:
    - console=ttyS1
    - panic=10
image: ghcr.io/siderolabs/installer:latest # Allows for supplying the image used to perform the installation.
bootloader: true # Indicates if a bootloader should be installed.
wipe: false # Indicates if the installation disk should be wiped at installation time.

# # Look up disk using disk attributes like model, size, serial and others.
# diskSelector:
#     size: 4GB # Disk size.
#     model: WDC* # Disk model `/sys/block/<dev>/device/model`.
#     busPath: /pci0000:00/0000:00:17.0/ata1/host0/target0:0:0/0:0:0:0 # Disk bus path.

# # Allows for supplying additional system extension images to install on top of base Talos image.
# extensions:
#     - image: ghcr.io/siderolabs/gvisor:20220117.0-v1.0.0 # System extension image.
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`disk` |string |The disk used for installations. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
disk: /dev/sda
{{< /highlight >}}{{< highlight yaml >}}
disk: /dev/nvme0
{{< /highlight >}}</details> | |
|`diskSelector` |<a href="#installdiskselector">InstallDiskSelector</a> |<details><summary>Look up disk using disk attributes like model, size, serial and others.</summary>Always has priority over `disk`.</details> <details><summary>Show example(s)</summary>{{< highlight yaml >}}
diskSelector:
    size: 4GB # Disk size.
    model: WDC* # Disk model `/sys/block/<dev>/device/model`.
    busPath: /pci0000:00/0000:00:17.0/ata1/host0/target0:0:0/0:0:0:0 # Disk bus path.
{{< /highlight >}}</details> | |
|`extraKernelArgs` |[]string |Allows for supplying extra kernel args via the bootloader. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
extraKernelArgs:
    - talos.platform=metal
    - reboot=k
{{< /highlight >}}</details> | |
|`image` |string |<details><summary>Allows for supplying the image used to perform the installation.</summary>Image reference for each Talos release can be found on<br />[GitHub releases page](https://github.com/siderolabs/talos/releases).</details> <details><summary>Show example(s)</summary>{{< highlight yaml >}}
image: ghcr.io/siderolabs/installer:latest
{{< /highlight >}}</details> | |
|`extensions` |[]<a href="#installextensionconfig">InstallExtensionConfig</a> |Allows for supplying additional system extension images to install on top of base Talos image. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
extensions:
    - image: ghcr.io/siderolabs/gvisor:20220117.0-v1.0.0 # System extension image.
{{< /highlight >}}</details> | |
|`bootloader` |bool |Indicates if a bootloader should be installed.  |`true`<br />`yes`<br />`false`<br />`no`<br /> |
|`wipe` |bool |<details><summary>Indicates if the installation disk should be wiped at installation time.</summary>Defaults to `true`.</details>  |`true`<br />`yes`<br />`false`<br />`no`<br /> |
|`legacyBIOSSupport` |bool |<details><summary>Indicates if MBR partition should be marked as bootable (active).</summary>Should be enabled only for the systems with legacy BIOS that doesn't support GPT partitioning scheme.</details>  | |



---
## InstallDiskSelector
InstallDiskSelector represents a disk query parameters for the install disk lookup.

Appears in:

- <code><a href="#installconfig">InstallConfig</a>.diskSelector</code>



{{< highlight yaml >}}
size: 4GB # Disk size.
model: WDC* # Disk model `/sys/block/<dev>/device/model`.
busPath: /pci0000:00/0000:00:17.0/ata1/host0/target0:0:0/0:0:0:0 # Disk bus path.
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`size` |InstallDiskSizeMatcher |Disk size. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
size: 4GB
{{< /highlight >}}{{< highlight yaml >}}
size: '> 1TB'
{{< /highlight >}}{{< highlight yaml >}}
size: <= 2TB
{{< /highlight >}}</details> | |
|`name` |string |Disk name `/sys/block/<dev>/device/name`.  | |
|`model` |string |Disk model `/sys/block/<dev>/device/model`.  | |
|`serial` |string |Disk serial number `/sys/block/<dev>/serial`.  | |
|`modalias` |string |Disk modalias `/sys/block/<dev>/device/modalias`.  | |
|`uuid` |string |Disk UUID `/sys/block/<dev>/uuid`.  | |
|`wwid` |string |Disk WWID `/sys/block/<dev>/wwid`.  | |
|`type` |InstallDiskType |Disk Type.  |`ssd`<br />`hdd`<br />`nvme`<br />`sd`<br /> |
|`busPath` |string |Disk bus path. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
busPath: /pci0000:00/0000:00:17.0/ata1/host0/target0:0:0/0:0:0:0
{{< /highlight >}}{{< highlight yaml >}}
busPath: /pci0000:00/*
{{< /highlight >}}</details> | |



---
## InstallExtensionConfig
InstallExtensionConfig represents a configuration for a system extension.

Appears in:

- <code><a href="#installconfig">InstallConfig</a>.extensions</code>



{{< highlight yaml >}}
- image: ghcr.io/siderolabs/gvisor:20220117.0-v1.0.0 # System extension image.
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`image` |string |System extension image.  | |



---
## TimeConfig
TimeConfig represents the options for configuring time on a machine.

Appears in:

- <code><a href="#machineconfig">MachineConfig</a>.time</code>



{{< highlight yaml >}}
disabled: false # Indicates if the time service is disabled for the machine.
# Specifies time (NTP) servers to use for setting the system time.
servers:
    - time.cloudflare.com
bootTimeout: 2m0s # Specifies the timeout when the node time is considered to be in sync unlocking the boot sequence.
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`disabled` |bool |<details><summary>Indicates if the time service is disabled for the machine.</summary>Defaults to `false`.</details>  | |
|`servers` |[]string |<details><summary>Specifies time (NTP) servers to use for setting the system time.</summary>Defaults to `pool.ntp.org`</details>  | |
|`bootTimeout` |Duration |<details><summary>Specifies the timeout when the node time is considered to be in sync unlocking the boot sequence.</summary>NTP sync will be still running in the background.<br />Defaults to "infinity" (waiting forever for time sync)</details>  | |



---
## RegistriesConfig
RegistriesConfig represents the image pull options.

Appears in:

- <code><a href="#machineconfig">MachineConfig</a>.registries</code>



{{< highlight yaml >}}
# Specifies mirror configuration for each registry.
mirrors:
    docker.io:
        # List of endpoints (URLs) for registry mirrors to use.
        endpoints:
            - https://registry.local
# Specifies TLS & auth configuration for HTTPS image registries.
config:
    registry.local:
        # The TLS configuration for the registry.
        tls:
            # Enable mutual TLS authentication with the registry.
            clientIdentity:
                crt: LS0tIEVYQU1QTEUgQ0VSVElGSUNBVEUgLS0t
                key: LS0tIEVYQU1QTEUgS0VZIC0tLQ==
        # The auth configuration for this registry.
        auth:
            username: username # Optional registry authentication.
            password: password # Optional registry authentication.
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`mirrors` |map[string]<a href="#registrymirrorconfig">RegistryMirrorConfig</a> |<details><summary>Specifies mirror configuration for each registry.</summary>This setting allows to use local pull-through caching registires,<br />air-gapped installations, etc.<br /><br />Registry name is the first segment of image identifier, with 'docker.io'<br />being default one.</details> <details><summary>Show example(s)</summary>{{< highlight yaml >}}
mirrors:
    ghcr.io:
        # List of endpoints (URLs) for registry mirrors to use.
        endpoints:
            - https://registry.insecure
            - https://ghcr.io/v2/
{{< /highlight >}}</details> | |
|`config` |map[string]<a href="#registryconfig">RegistryConfig</a> |<details><summary>Specifies TLS & auth configuration for HTTPS image registries.</summary>Mutual TLS can be enabled with 'clientIdentity' option.<br /><br />TLS configuration can be skipped if registry has trusted<br />server certificate.</details> <details><summary>Show example(s)</summary>{{< highlight yaml >}}
config:
    registry.insecure:
        # The TLS configuration for the registry.
        tls:
            insecureSkipVerify: true # Skip TLS server certificate verification (not recommended).

            # # Enable mutual TLS authentication with the registry.
            # clientIdentity:
            #     crt: LS0tIEVYQU1QTEUgQ0VSVElGSUNBVEUgLS0t
            #     key: LS0tIEVYQU1QTEUgS0VZIC0tLQ==

        # # The auth configuration for this registry.
        # auth:
        #     username: username # Optional registry authentication.
        #     password: password # Optional registry authentication.
{{< /highlight >}}</details> | |



---
## PodCheckpointer
PodCheckpointer represents the pod-checkpointer config values.





| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`image` |string |The `image` field is an override to the default pod-checkpointer image.  | |



---
## CoreDNS
CoreDNS represents the CoreDNS config values.

Appears in:

- <code><a href="#clusterconfig">ClusterConfig</a>.coreDNS</code>



{{< highlight yaml >}}
image: docker.io/coredns/coredns:1.10.0 # The `image` field is an override to the default coredns image.
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`disabled` |bool |Disable coredns deployment on cluster bootstrap.  | |
|`image` |string |The `image` field is an override to the default coredns image.  | |



---
## Endpoint
Endpoint represents the endpoint URL parsed out of the machine config.

Appears in:

- <code><a href="#controlplaneconfig">ControlPlaneConfig</a>.endpoint</code>
- <code><a href="#loggingdestination">LoggingDestination</a>.endpoint</code>



{{< highlight yaml >}}
https://1.2.3.4:6443
{{< /highlight >}}

{{< highlight yaml >}}
https://cluster1.internal:6443
{{< /highlight >}}

{{< highlight yaml >}}
udp://127.0.0.1:12345
{{< /highlight >}}

{{< highlight yaml >}}
tcp://1.2.3.4:12345
{{< /highlight >}}




---
## ControlPlaneConfig
ControlPlaneConfig represents the control plane configuration options.

Appears in:

- <code><a href="#clusterconfig">ClusterConfig</a>.controlPlane</code>



{{< highlight yaml >}}
endpoint: https://1.2.3.4 # Endpoint is the canonical controlplane endpoint, which can be an IP address or a DNS hostname.
localAPIServerPort: 443 # The port that the API server listens on internally.
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`endpoint` |<a href="#endpoint">Endpoint</a> |<details><summary>Endpoint is the canonical controlplane endpoint, which can be an IP address or a DNS hostname.</summary>It is single-valued, and may optionally include a port number.</details> <details><summary>Show example(s)</summary>{{< highlight yaml >}}
endpoint: https://1.2.3.4:6443
{{< /highlight >}}{{< highlight yaml >}}
endpoint: https://cluster1.internal:6443
{{< /highlight >}}</details> | |
|`localAPIServerPort` |int |<details><summary>The port that the API server listens on internally.</summary>This may be different than the port portion listed in the endpoint field above.<br />The default is `6443`.</details>  | |



---
## APIServerConfig
APIServerConfig represents the kube apiserver configuration options.

Appears in:

- <code><a href="#clusterconfig">ClusterConfig</a>.apiServer</code>



{{< highlight yaml >}}
image: k8s.gcr.io/kube-apiserver:v1.26.0-alpha.1 # The container image used in the API server manifest.
# Extra arguments to supply to the API server.
extraArgs:
    feature-gates: ServerSideApply=true
    http2-max-streams-per-connection: "32"
# Extra certificate subject alternative names for the API server's certificate.
certSANs:
    - 1.2.3.4
    - 4.5.6.7

# # Configure the API server admission plugins.
# admissionControl:
#     - name: PodSecurity # Name is the name of the admission controller.
#       # Configuration is an embedded configuration object to be used as the plugin's
#       configuration:
#         apiVersion: pod-security.admission.config.k8s.io/v1alpha1
#         defaults:
#             audit: restricted
#             audit-version: latest
#             enforce: baseline
#             enforce-version: latest
#             warn: restricted
#             warn-version: latest
#         exemptions:
#             namespaces:
#                 - kube-system
#             runtimeClasses: []
#             usernames: []
#         kind: PodSecurityConfiguration

# # Configure the API server audit policy.
# auditPolicy:
#     apiVersion: audit.k8s.io/v1
#     kind: Policy
#     rules:
#         - level: Metadata
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`image` |string |The container image used in the API server manifest. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
image: k8s.gcr.io/kube-apiserver:v1.26.0-alpha.1
{{< /highlight >}}</details> | |
|`extraArgs` |map[string]string |Extra arguments to supply to the API server.  | |
|`extraVolumes` |[]<a href="#volumemountconfig">VolumeMountConfig</a> |Extra volumes to mount to the API server static pod.  | |
|`env` |Env |The `env` field allows for the addition of environment variables for the control plane component.  | |
|`certSANs` |[]string |Extra certificate subject alternative names for the API server's certificate.  | |
|`disablePodSecurityPolicy` |bool |Disable PodSecurityPolicy in the API server and default manifests.  | |
|`admissionControl` |[]<a href="#admissionpluginconfig">AdmissionPluginConfig</a> |Configure the API server admission plugins. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
admissionControl:
    - name: PodSecurity # Name is the name of the admission controller.
      # Configuration is an embedded configuration object to be used as the plugin's
      configuration:
        apiVersion: pod-security.admission.config.k8s.io/v1alpha1
        defaults:
            audit: restricted
            audit-version: latest
            enforce: baseline
            enforce-version: latest
            warn: restricted
            warn-version: latest
        exemptions:
            namespaces:
                - kube-system
            runtimeClasses: []
            usernames: []
        kind: PodSecurityConfiguration
{{< /highlight >}}</details> | |
|`auditPolicy` |Unstructured |Configure the API server audit policy. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
auditPolicy:
    apiVersion: audit.k8s.io/v1
    kind: Policy
    rules:
        - level: Metadata
{{< /highlight >}}</details> | |



---
## AdmissionPluginConfig
AdmissionPluginConfig represents the API server admission plugin configuration.

Appears in:

- <code><a href="#apiserverconfig">APIServerConfig</a>.admissionControl</code>



{{< highlight yaml >}}
- name: PodSecurity # Name is the name of the admission controller.
  # Configuration is an embedded configuration object to be used as the plugin's
  configuration:
    apiVersion: pod-security.admission.config.k8s.io/v1alpha1
    defaults:
        audit: restricted
        audit-version: latest
        enforce: baseline
        enforce-version: latest
        warn: restricted
        warn-version: latest
    exemptions:
        namespaces:
            - kube-system
        runtimeClasses: []
        usernames: []
    kind: PodSecurityConfiguration
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`name` |string |<details><summary>Name is the name of the admission controller.</summary>It must match the registered admission plugin name.</details>  | |
|`configuration` |Unstructured |<details><summary>Configuration is an embedded configuration object to be used as the plugin's</summary>configuration.</details>  | |



---
## ControllerManagerConfig
ControllerManagerConfig represents the kube controller manager configuration options.

Appears in:

- <code><a href="#clusterconfig">ClusterConfig</a>.controllerManager</code>



{{< highlight yaml >}}
image: k8s.gcr.io/kube-controller-manager:v1.26.0-alpha.1 # The container image used in the controller manager manifest.
# Extra arguments to supply to the controller manager.
extraArgs:
    feature-gates: ServerSideApply=true
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`image` |string |The container image used in the controller manager manifest. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
image: k8s.gcr.io/kube-controller-manager:v1.26.0-alpha.1
{{< /highlight >}}</details> | |
|`extraArgs` |map[string]string |Extra arguments to supply to the controller manager.  | |
|`extraVolumes` |[]<a href="#volumemountconfig">VolumeMountConfig</a> |Extra volumes to mount to the controller manager static pod.  | |
|`env` |Env |The `env` field allows for the addition of environment variables for the control plane component.  | |



---
## ProxyConfig
ProxyConfig represents the kube proxy configuration options.

Appears in:

- <code><a href="#clusterconfig">ClusterConfig</a>.proxy</code>



{{< highlight yaml >}}
image: k8s.gcr.io/kube-proxy:v1.26.0-alpha.1 # The container image used in the kube-proxy manifest.
mode: ipvs # proxy mode of kube-proxy.
# Extra arguments to supply to kube-proxy.
extraArgs:
    proxy-mode: iptables

# # Disable kube-proxy deployment on cluster bootstrap.
# disabled: false
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`disabled` |bool |Disable kube-proxy deployment on cluster bootstrap. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
disabled: false
{{< /highlight >}}</details> | |
|`image` |string |The container image used in the kube-proxy manifest. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
image: k8s.gcr.io/kube-proxy:v1.26.0-alpha.1
{{< /highlight >}}</details> | |
|`mode` |string |<details><summary>proxy mode of kube-proxy.</summary>The default is 'iptables'.</details>  | |
|`extraArgs` |map[string]string |Extra arguments to supply to kube-proxy.  | |



---
## SchedulerConfig
SchedulerConfig represents the kube scheduler configuration options.

Appears in:

- <code><a href="#clusterconfig">ClusterConfig</a>.scheduler</code>



{{< highlight yaml >}}
image: k8s.gcr.io/kube-scheduler:v1.26.0-alpha.1 # The container image used in the scheduler manifest.
# Extra arguments to supply to the scheduler.
extraArgs:
    feature-gates: AllBeta=true
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`image` |string |The container image used in the scheduler manifest. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
image: k8s.gcr.io/kube-scheduler:v1.26.0-alpha.1
{{< /highlight >}}</details> | |
|`extraArgs` |map[string]string |Extra arguments to supply to the scheduler.  | |
|`extraVolumes` |[]<a href="#volumemountconfig">VolumeMountConfig</a> |Extra volumes to mount to the scheduler static pod.  | |
|`env` |Env |The `env` field allows for the addition of environment variables for the control plane component.  | |



---
## EtcdConfig
EtcdConfig represents the etcd configuration options.

Appears in:

- <code><a href="#clusterconfig">ClusterConfig</a>.etcd</code>



{{< highlight yaml >}}
image: gcr.io/etcd-development/etcd:v3.5.5 # The container image used to create the etcd service.
# The `ca` is the root certificate authority of the PKI.
ca:
    crt: LS0tIEVYQU1QTEUgQ0VSVElGSUNBVEUgLS0t
    key: LS0tIEVYQU1QTEUgS0VZIC0tLQ==
# Extra arguments to supply to etcd.
extraArgs:
    election-timeout: "5000"

# # The `advertisedSubnets` field configures the networks to pick etcd advertised IP from.
# advertisedSubnets:
#     - 10.0.0.0/8
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`image` |string |The container image used to create the etcd service. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
image: gcr.io/etcd-development/etcd:v3.5.5
{{< /highlight >}}</details> | |
|`ca` |PEMEncodedCertificateAndKey |<details><summary>The `ca` is the root certificate authority of the PKI.</summary>It is composed of a base64 encoded `crt` and `key`.</details> <details><summary>Show example(s)</summary>{{< highlight yaml >}}
ca:
    crt: LS0tIEVYQU1QTEUgQ0VSVElGSUNBVEUgLS0t
    key: LS0tIEVYQU1QTEUgS0VZIC0tLQ==
{{< /highlight >}}</details> | |
|`extraArgs` |map[string]string |<details><summary>Extra arguments to supply to etcd.</summary>Note that the following args are not allowed:<br /><br />- `name`<br />- `data-dir`<br />- `initial-cluster-state`<br />- `listen-peer-urls`<br />- `listen-client-urls`<br />- `cert-file`<br />- `key-file`<br />- `trusted-ca-file`<br />- `peer-client-cert-auth`<br />- `peer-cert-file`<br />- `peer-trusted-ca-file`<br />- `peer-key-file`</details>  | |
|`advertisedSubnets` |[]string |<details><summary>The `advertisedSubnets` field configures the networks to pick etcd advertised IP from.</summary><br />IPs can be excluded from the list by using negative match with `!`, e.g `!10.0.0.0/8`.<br />Negative subnet matches should be specified last to filter out IPs picked by positive matches.<br />If not specified, advertised IP is selected as the first routable address of the node.</details> <details><summary>Show example(s)</summary>{{< highlight yaml >}}
advertisedSubnets:
    - 10.0.0.0/8
{{< /highlight >}}</details> | |
|`listenSubnets` |[]string |<details><summary>The `listenSubnets` field configures the networks for the etcd to listen for peer and client connections.</summary><br />If `listenSubnets` is not set, but `advertisedSubnets` is set, `listenSubnets` defaults to<br />`advertisedSubnets`.<br /><br />If neither `advertisedSubnets` nor `listenSubnets` is set, `listenSubnets` defaults to listen on all addresses.<br /><br />IPs can be excluded from the list by using negative match with `!`, e.g `!10.0.0.0/8`.<br />Negative subnet matches should be specified last to filter out IPs picked by positive matches.<br />If not specified, advertised IP is selected as the first routable address of the node.</details>  | |



---
## ClusterNetworkConfig
ClusterNetworkConfig represents kube networking configuration options.

Appears in:

- <code><a href="#clusterconfig">ClusterConfig</a>.network</code>



{{< highlight yaml >}}
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
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`cni` |<a href="#cniconfig">CNIConfig</a> |<details><summary>The CNI used.</summary>Composed of "name" and "urls".<br />The "name" key supports the following options: "flannel", "custom", and "none".<br />"flannel" uses Talos-managed Flannel CNI, and that's the default option.<br />"custom" uses custom manifests that should be provided in "urls".<br />"none" indicates that Talos will not manage any CNI installation.</details> <details><summary>Show example(s)</summary>{{< highlight yaml >}}
cni:
    name: custom # Name of CNI to use.
    # URLs containing manifests to apply for the CNI.
    urls:
        - https://docs.projectcalico.org/archive/v3.20/manifests/canal.yaml
{{< /highlight >}}</details> | |
|`dnsDomain` |string |<details><summary>The domain used by Kubernetes DNS.</summary>The default is `cluster.local`</details> <details><summary>Show example(s)</summary>{{< highlight yaml >}}
dnsDomain: cluser.local
{{< /highlight >}}</details> | |
|`podSubnets` |[]string |The pod subnet CIDR. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
podSubnets:
    - 10.244.0.0/16
{{< /highlight >}}</details> | |
|`serviceSubnets` |[]string |The service subnet CIDR. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
serviceSubnets:
    - 10.96.0.0/12
{{< /highlight >}}</details> | |



---
## CNIConfig
CNIConfig represents the CNI configuration options.

Appears in:

- <code><a href="#clusternetworkconfig">ClusterNetworkConfig</a>.cni</code>



{{< highlight yaml >}}
name: custom # Name of CNI to use.
# URLs containing manifests to apply for the CNI.
urls:
    - https://docs.projectcalico.org/archive/v3.20/manifests/canal.yaml
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`name` |string |Name of CNI to use.  |`flannel`<br />`custom`<br />`none`<br /> |
|`urls` |[]string |<details><summary>URLs containing manifests to apply for the CNI.</summary>Should be present for "custom", must be empty for "flannel" and "none".</details>  | |



---
## ExternalCloudProviderConfig
ExternalCloudProviderConfig contains external cloud provider configuration.

Appears in:

- <code><a href="#clusterconfig">ClusterConfig</a>.externalCloudProvider</code>



{{< highlight yaml >}}
enabled: true # Enable external cloud provider.
# A list of urls that point to additional manifests for an external cloud provider.
manifests:
    - https://raw.githubusercontent.com/kubernetes/cloud-provider-aws/v1.20.0-alpha.0/manifests/rbac.yaml
    - https://raw.githubusercontent.com/kubernetes/cloud-provider-aws/v1.20.0-alpha.0/manifests/aws-cloud-controller-manager-daemonset.yaml
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`enabled` |bool |Enable external cloud provider.  |`true`<br />`yes`<br />`false`<br />`no`<br /> |
|`manifests` |[]string |<details><summary>A list of urls that point to additional manifests for an external cloud provider.</summary>These will get automatically deployed as part of the bootstrap.</details> <details><summary>Show example(s)</summary>{{< highlight yaml >}}
manifests:
    - https://raw.githubusercontent.com/kubernetes/cloud-provider-aws/v1.20.0-alpha.0/manifests/rbac.yaml
    - https://raw.githubusercontent.com/kubernetes/cloud-provider-aws/v1.20.0-alpha.0/manifests/aws-cloud-controller-manager-daemonset.yaml
{{< /highlight >}}</details> | |



---
## AdminKubeconfigConfig
AdminKubeconfigConfig contains admin kubeconfig settings.

Appears in:

- <code><a href="#clusterconfig">ClusterConfig</a>.adminKubeconfig</code>



{{< highlight yaml >}}
certLifetime: 1h0m0s # Admin kubeconfig certificate lifetime (default is 1 year).
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`certLifetime` |Duration |<details><summary>Admin kubeconfig certificate lifetime (default is 1 year).</summary>Field format accepts any Go time.Duration format ('1h' for one hour, '10m' for ten minutes).</details>  | |



---
## MachineDisk
MachineDisk represents the options available for partitioning, formatting, and
mounting extra disks.


Appears in:

- <code><a href="#machineconfig">MachineConfig</a>.disks</code>



{{< highlight yaml >}}
- device: /dev/sdb # The name of the disk to use.
  # A list of partitions to create on the disk.
  partitions:
    - mountpoint: /var/mnt/extra # Where to mount the partition.

      # # The size of partition: either bytes or human readable representation. If `size:` is omitted, the partition is sized to occupy the full disk.

      # # Human readable representation.
      # size: 100 MB
      # # Precise value in bytes.
      # size: 1073741824
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`device` |string |The name of the disk to use.  | |
|`partitions` |[]<a href="#diskpartition">DiskPartition</a> |A list of partitions to create on the disk.  | |



---
## DiskPartition
DiskPartition represents the options for a disk partition.

Appears in:

- <code><a href="#machinedisk">MachineDisk</a>.partitions</code>




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`size` |DiskSize |The size of partition: either bytes or human readable representation. If `size:` is omitted, the partition is sized to occupy the full disk. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
size: 100 MB
{{< /highlight >}}{{< highlight yaml >}}
size: 1073741824
{{< /highlight >}}</details> | |
|`mountpoint` |string |Where to mount the partition.  | |



---
## EncryptionConfig
EncryptionConfig represents partition encryption settings.

Appears in:

- <code><a href="#systemdiskencryptionconfig">SystemDiskEncryptionConfig</a>.state</code>
- <code><a href="#systemdiskencryptionconfig">SystemDiskEncryptionConfig</a>.ephemeral</code>




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`provider` |string |Encryption provider to use for the encryption. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
provider: luks2
{{< /highlight >}}</details> | |
|`keys` |[]<a href="#encryptionkey">EncryptionKey</a> |Defines the encryption keys generation and storage method.  | |
|`cipher` |string |Cipher kind to use for the encryption. Depends on the encryption provider. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
cipher: aes-xts-plain64
{{< /highlight >}}</details> |`aes-xts-plain64`<br />`xchacha12,aes-adiantum-plain64`<br />`xchacha20,aes-adiantum-plain64`<br /> |
|`keySize` |uint |Defines the encryption key length.  | |
|`blockSize` |uint64 |Defines the encryption sector size. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
blockSize: 4096
{{< /highlight >}}</details> | |
|`options` |[]string |Additional --perf parameters for the LUKS2 encryption. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
options:
    - no_read_workqueue
    - no_write_workqueue
{{< /highlight >}}</details> |`no_read_workqueue`<br />`no_write_workqueue`<br />`same_cpu_crypt`<br /> |



---
## EncryptionKey
EncryptionKey represents configuration for disk encryption key.

Appears in:

- <code><a href="#encryptionconfig">EncryptionConfig</a>.keys</code>




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`static` |<a href="#encryptionkeystatic">EncryptionKeyStatic</a> |Key which value is stored in the configuration file.  | |
|`nodeID` |<a href="#encryptionkeynodeid">EncryptionKeyNodeID</a> |Deterministically generated key from the node UUID and PartitionLabel.  | |
|`slot` |int |Key slot number for LUKS2 encryption.  | |



---
## EncryptionKeyStatic
EncryptionKeyStatic represents throw away key type.

Appears in:

- <code><a href="#encryptionkey">EncryptionKey</a>.static</code>




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`passphrase` |string |Defines the static passphrase value.  | |



---
## EncryptionKeyNodeID
EncryptionKeyNodeID represents deterministically generated key from the node UUID and PartitionLabel.

Appears in:

- <code><a href="#encryptionkey">EncryptionKey</a>.nodeID</code>






---
## MachineFile
MachineFile represents a file to write to disk.

Appears in:

- <code><a href="#machineconfig">MachineConfig</a>.files</code>



{{< highlight yaml >}}
- content: '...' # The contents of the file.
  permissions: 0o666 # The file's permissions in octal.
  path: /tmp/file.txt # The path of the file.
  op: append # The operation to use
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`content` |string |The contents of the file.  | |
|`permissions` |FileMode |The file's permissions in octal.  | |
|`path` |string |The path of the file.  | |
|`op` |string |The operation to use  |`create`<br />`append`<br />`overwrite`<br /> |



---
## ExtraHost
ExtraHost represents a host entry in /etc/hosts.

Appears in:

- <code><a href="#networkconfig">NetworkConfig</a>.extraHostEntries</code>



{{< highlight yaml >}}
- ip: 192.168.1.100 # The IP of the host.
  # The host alias.
  aliases:
    - example
    - example.domain.tld
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`ip` |string |The IP of the host.  | |
|`aliases` |[]string |The host alias.  | |



---
## Device
Device represents a network interface.

Appears in:

- <code><a href="#networkconfig">NetworkConfig</a>.interfaces</code>



{{< highlight yaml >}}
- interface: eth0 # The interface name.
  # Assigns static IP addresses to the interface.
  addresses:
    - 192.168.2.0/24
  # A list of routes associated with the interface.
  routes:
    - network: 0.0.0.0/0 # The route's network (destination).
      gateway: 192.168.2.1 # The route's gateway (if empty, creates link scope route).
      metric: 1024 # The optional metric for the route.
  mtu: 1500 # The interface's MTU.

  # # Picks a network device using the selector.

  # # select a device with bus prefix 00:*.
  # deviceSelector:
  #     busPath: 00:* # PCI, USB bus prefix, supports matching by wildcard.
  # # select a device with mac address matching `*:f0:ab` and `virtio` kernel driver.
  # deviceSelector:
  #     hardwareAddr: '*:f0:ab' # Device hardware address, supports matching by wildcard.
  #     driver: virtio # Kernel driver, supports matching by wildcard.

  # # Bond specific options.
  # bond:
  #     # The interfaces that make up the bond.
  #     interfaces:
  #         - eth0
  #         - eth1
  #     mode: 802.3ad # A bond option.
  #     lacpRate: fast # A bond option.

  # # Bridge specific options.
  # bridge:
  #     # The interfaces that make up the bridge.
  #     interfaces:
  #         - eth0
  #         - eth1
  #     # A bridge option.
  #     stp:
  #         enabled: true # Whether Spanning Tree Protocol (STP) is enabled.

  # # Indicates if DHCP should be used to configure the interface.
  # dhcp: true

  # # DHCP specific options.
  # dhcpOptions:
  #     routeMetric: 1024 # The priority of all routes received via DHCP.

  # # Wireguard specific configuration.

  # # wireguard server example
  # wireguard:
  #     privateKey: ABCDEF... # Specifies a private key configuration (base64 encoded).
  #     listenPort: 51111 # Specifies a device's listening port.
  #     # Specifies a list of peer configurations to apply to a device.
  #     peers:
  #         - publicKey: ABCDEF... # Specifies the public key of this peer.
  #           endpoint: 192.168.1.3 # Specifies the endpoint of this peer entry.
  #           # AllowedIPs specifies a list of allowed IP addresses in CIDR notation for this peer.
  #           allowedIPs:
  #             - 192.168.1.0/24
  # # wireguard peer example
  # wireguard:
  #     privateKey: ABCDEF... # Specifies a private key configuration (base64 encoded).
  #     # Specifies a list of peer configurations to apply to a device.
  #     peers:
  #         - publicKey: ABCDEF... # Specifies the public key of this peer.
  #           endpoint: 192.168.1.2 # Specifies the endpoint of this peer entry.
  #           persistentKeepaliveInterval: 10s # Specifies the persistent keepalive interval for this peer.
  #           # AllowedIPs specifies a list of allowed IP addresses in CIDR notation for this peer.
  #           allowedIPs:
  #             - 192.168.1.0/24

  # # Virtual (shared) IP address configuration.

  # # layer2 vip example
  # vip:
  #     ip: 172.16.199.55 # Specifies the IP address to be used.
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`interface` |string |<details><summary>The interface name.</summary>Mutually exclusive with `deviceSelector`.</details> <details><summary>Show example(s)</summary>{{< highlight yaml >}}
interface: eth0
{{< /highlight >}}</details> | |
|`deviceSelector` |<a href="#networkdeviceselector">NetworkDeviceSelector</a> |<details><summary>Picks a network device using the selector.</summary>Mutually exclusive with `interface`.<br />Supports partial match using wildcard syntax.</details> <details><summary>Show example(s)</summary>{{< highlight yaml >}}
deviceSelector:
    busPath: 00:* # PCI, USB bus prefix, supports matching by wildcard.
{{< /highlight >}}{{< highlight yaml >}}
deviceSelector:
    hardwareAddr: '*:f0:ab' # Device hardware address, supports matching by wildcard.
    driver: virtio # Kernel driver, supports matching by wildcard.
{{< /highlight >}}</details> | |
|`addresses` |[]string |<details><summary>Assigns static IP addresses to the interface.</summary>An address can be specified either in proper CIDR notation or as a standalone address (netmask of all ones is assumed).</details> <details><summary>Show example(s)</summary>{{< highlight yaml >}}
addresses:
    - 10.5.0.0/16
    - 192.168.3.7
{{< /highlight >}}</details> | |
|`routes` |[]<a href="#route">Route</a> |<details><summary>A list of routes associated with the interface.</summary>If used in combination with DHCP, these routes will be appended to routes returned by DHCP server.</details> <details><summary>Show example(s)</summary>{{< highlight yaml >}}
routes:
    - network: 0.0.0.0/0 # The route's network (destination).
      gateway: 10.5.0.1 # The route's gateway (if empty, creates link scope route).
    - network: 10.2.0.0/16 # The route's network (destination).
      gateway: 10.2.0.1 # The route's gateway (if empty, creates link scope route).
{{< /highlight >}}</details> | |
|`bond` |<a href="#bond">Bond</a> |Bond specific options. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
bond:
    # The interfaces that make up the bond.
    interfaces:
        - eth0
        - eth1
    mode: 802.3ad # A bond option.
    lacpRate: fast # A bond option.
{{< /highlight >}}</details> | |
|`bridge` |<a href="#bridge">Bridge</a> |Bridge specific options. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
bridge:
    # The interfaces that make up the bridge.
    interfaces:
        - eth0
        - eth1
    # A bridge option.
    stp:
        enabled: true # Whether Spanning Tree Protocol (STP) is enabled.
{{< /highlight >}}</details> | |
|`vlans` |[]<a href="#vlan">Vlan</a> |VLAN specific options.  | |
|`mtu` |int |<details><summary>The interface's MTU.</summary>If used in combination with DHCP, this will override any MTU settings returned from DHCP server.</details>  | |
|`dhcp` |bool |<details><summary>Indicates if DHCP should be used to configure the interface.</summary>The following DHCP options are supported:<br /><br />- `OptionClasslessStaticRoute`<br />- `OptionDomainNameServer`<br />- `OptionDNSDomainSearchList`<br />- `OptionHostName`</details> <details><summary>Show example(s)</summary>{{< highlight yaml >}}
dhcp: true
{{< /highlight >}}</details> | |
|`ignore` |bool |Indicates if the interface should be ignored (skips configuration).  | |
|`dummy` |bool |<details><summary>Indicates if the interface is a dummy interface.</summary>`dummy` is used to specify that this interface should be a virtual-only, dummy interface.</details>  | |
|`dhcpOptions` |<a href="#dhcpoptions">DHCPOptions</a> |<details><summary>DHCP specific options.</summary>`dhcp` *must* be set to true for these to take effect.</details> <details><summary>Show example(s)</summary>{{< highlight yaml >}}
dhcpOptions:
    routeMetric: 1024 # The priority of all routes received via DHCP.
{{< /highlight >}}</details> | |
|`wireguard` |<a href="#devicewireguardconfig">DeviceWireguardConfig</a> |<details><summary>Wireguard specific configuration.</summary>Includes things like private key, listen port, peers.</details> <details><summary>Show example(s)</summary>{{< highlight yaml >}}
wireguard:
    privateKey: ABCDEF... # Specifies a private key configuration (base64 encoded).
    listenPort: 51111 # Specifies a device's listening port.
    # Specifies a list of peer configurations to apply to a device.
    peers:
        - publicKey: ABCDEF... # Specifies the public key of this peer.
          endpoint: 192.168.1.3 # Specifies the endpoint of this peer entry.
          # AllowedIPs specifies a list of allowed IP addresses in CIDR notation for this peer.
          allowedIPs:
            - 192.168.1.0/24
{{< /highlight >}}{{< highlight yaml >}}
wireguard:
    privateKey: ABCDEF... # Specifies a private key configuration (base64 encoded).
    # Specifies a list of peer configurations to apply to a device.
    peers:
        - publicKey: ABCDEF... # Specifies the public key of this peer.
          endpoint: 192.168.1.2 # Specifies the endpoint of this peer entry.
          persistentKeepaliveInterval: 10s # Specifies the persistent keepalive interval for this peer.
          # AllowedIPs specifies a list of allowed IP addresses in CIDR notation for this peer.
          allowedIPs:
            - 192.168.1.0/24
{{< /highlight >}}</details> | |
|`vip` |<a href="#devicevipconfig">DeviceVIPConfig</a> |Virtual (shared) IP address configuration. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
vip:
    ip: 172.16.199.55 # Specifies the IP address to be used.
{{< /highlight >}}</details> | |



---
## DHCPOptions
DHCPOptions contains options for configuring the DHCP settings for a given interface.

Appears in:

- <code><a href="#device">Device</a>.dhcpOptions</code>
- <code><a href="#vlan">Vlan</a>.dhcpOptions</code>



{{< highlight yaml >}}
routeMetric: 1024 # The priority of all routes received via DHCP.
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`routeMetric` |uint32 |The priority of all routes received via DHCP.  | |
|`ipv4` |bool |Enables DHCPv4 protocol for the interface (default is enabled).  | |
|`ipv6` |bool |Enables DHCPv6 protocol for the interface (default is disabled).  | |
|`duidv6` |string |Set client DUID (hex string).  | |



---
## DeviceWireguardConfig
DeviceWireguardConfig contains settings for configuring Wireguard network interface.

Appears in:

- <code><a href="#device">Device</a>.wireguard</code>



{{< highlight yaml >}}
privateKey: ABCDEF... # Specifies a private key configuration (base64 encoded).
listenPort: 51111 # Specifies a device's listening port.
# Specifies a list of peer configurations to apply to a device.
peers:
    - publicKey: ABCDEF... # Specifies the public key of this peer.
      endpoint: 192.168.1.3 # Specifies the endpoint of this peer entry.
      # AllowedIPs specifies a list of allowed IP addresses in CIDR notation for this peer.
      allowedIPs:
        - 192.168.1.0/24
{{< /highlight >}}

{{< highlight yaml >}}
privateKey: ABCDEF... # Specifies a private key configuration (base64 encoded).
# Specifies a list of peer configurations to apply to a device.
peers:
    - publicKey: ABCDEF... # Specifies the public key of this peer.
      endpoint: 192.168.1.2 # Specifies the endpoint of this peer entry.
      persistentKeepaliveInterval: 10s # Specifies the persistent keepalive interval for this peer.
      # AllowedIPs specifies a list of allowed IP addresses in CIDR notation for this peer.
      allowedIPs:
        - 192.168.1.0/24
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`privateKey` |string |<details><summary>Specifies a private key configuration (base64 encoded).</summary>Can be generated by `wg genkey`.</details>  | |
|`listenPort` |int |Specifies a device's listening port.  | |
|`firewallMark` |int |Specifies a device's firewall mark.  | |
|`peers` |[]<a href="#devicewireguardpeer">DeviceWireguardPeer</a> |Specifies a list of peer configurations to apply to a device.  | |



---
## DeviceWireguardPeer
DeviceWireguardPeer a WireGuard device peer configuration.

Appears in:

- <code><a href="#devicewireguardconfig">DeviceWireguardConfig</a>.peers</code>




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`publicKey` |string |<details><summary>Specifies the public key of this peer.</summary>Can be extracted from private key by running `wg pubkey < private.key > public.key && cat public.key`.</details>  | |
|`endpoint` |string |Specifies the endpoint of this peer entry.  | |
|`persistentKeepaliveInterval` |Duration |<details><summary>Specifies the persistent keepalive interval for this peer.</summary>Field format accepts any Go time.Duration format ('1h' for one hour, '10m' for ten minutes).</details>  | |
|`allowedIPs` |[]string |AllowedIPs specifies a list of allowed IP addresses in CIDR notation for this peer.  | |



---
## DeviceVIPConfig
DeviceVIPConfig contains settings for configuring a Virtual Shared IP on an interface.

Appears in:

- <code><a href="#device">Device</a>.vip</code>
- <code><a href="#vlan">Vlan</a>.vip</code>



{{< highlight yaml >}}
ip: 172.16.199.55 # Specifies the IP address to be used.
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`ip` |string |Specifies the IP address to be used.  | |
|`equinixMetal` |<a href="#vipequinixmetalconfig">VIPEquinixMetalConfig</a> |Specifies the Equinix Metal API settings to assign VIP to the node.  | |
|`hcloud` |<a href="#viphcloudconfig">VIPHCloudConfig</a> |Specifies the Hetzner Cloud API settings to assign VIP to the node.  | |



---
## VIPEquinixMetalConfig
VIPEquinixMetalConfig contains settings for Equinix Metal VIP management.

Appears in:

- <code><a href="#devicevipconfig">DeviceVIPConfig</a>.equinixMetal</code>




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`apiToken` |string |Specifies the Equinix Metal API Token.  | |



---
## VIPHCloudConfig
VIPHCloudConfig contains settings for Hetzner Cloud VIP management.

Appears in:

- <code><a href="#devicevipconfig">DeviceVIPConfig</a>.hcloud</code>




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`apiToken` |string |Specifies the Hetzner Cloud API Token.  | |



---
## Bond
Bond contains the various options for configuring a bonded interface.

Appears in:

- <code><a href="#device">Device</a>.bond</code>



{{< highlight yaml >}}
# The interfaces that make up the bond.
interfaces:
    - eth0
    - eth1
mode: 802.3ad # A bond option.
lacpRate: fast # A bond option.
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`interfaces` |[]string |The interfaces that make up the bond.  | |
|`arpIPTarget` |[]string |<details><summary>A bond option.</summary>Please see the official kernel documentation.<br />Not supported at the moment.</details>  | |
|`mode` |string |<details><summary>A bond option.</summary>Please see the official kernel documentation.</details>  | |
|`xmitHashPolicy` |string |<details><summary>A bond option.</summary>Please see the official kernel documentation.</details>  | |
|`lacpRate` |string |<details><summary>A bond option.</summary>Please see the official kernel documentation.</details>  | |
|`adActorSystem` |string |<details><summary>A bond option.</summary>Please see the official kernel documentation.<br />Not supported at the moment.</details>  | |
|`arpValidate` |string |<details><summary>A bond option.</summary>Please see the official kernel documentation.</details>  | |
|`arpAllTargets` |string |<details><summary>A bond option.</summary>Please see the official kernel documentation.</details>  | |
|`primary` |string |<details><summary>A bond option.</summary>Please see the official kernel documentation.</details>  | |
|`primaryReselect` |string |<details><summary>A bond option.</summary>Please see the official kernel documentation.</details>  | |
|`failOverMac` |string |<details><summary>A bond option.</summary>Please see the official kernel documentation.</details>  | |
|`adSelect` |string |<details><summary>A bond option.</summary>Please see the official kernel documentation.</details>  | |
|`miimon` |uint32 |<details><summary>A bond option.</summary>Please see the official kernel documentation.</details>  | |
|`updelay` |uint32 |<details><summary>A bond option.</summary>Please see the official kernel documentation.</details>  | |
|`downdelay` |uint32 |<details><summary>A bond option.</summary>Please see the official kernel documentation.</details>  | |
|`arpInterval` |uint32 |<details><summary>A bond option.</summary>Please see the official kernel documentation.</details>  | |
|`resendIgmp` |uint32 |<details><summary>A bond option.</summary>Please see the official kernel documentation.</details>  | |
|`minLinks` |uint32 |<details><summary>A bond option.</summary>Please see the official kernel documentation.</details>  | |
|`lpInterval` |uint32 |<details><summary>A bond option.</summary>Please see the official kernel documentation.</details>  | |
|`packetsPerSlave` |uint32 |<details><summary>A bond option.</summary>Please see the official kernel documentation.</details>  | |
|`numPeerNotif` |uint8 |<details><summary>A bond option.</summary>Please see the official kernel documentation.</details>  | |
|`tlbDynamicLb` |uint8 |<details><summary>A bond option.</summary>Please see the official kernel documentation.</details>  | |
|`allSlavesActive` |uint8 |<details><summary>A bond option.</summary>Please see the official kernel documentation.</details>  | |
|`useCarrier` |bool |<details><summary>A bond option.</summary>Please see the official kernel documentation.</details>  | |
|`adActorSysPrio` |uint16 |<details><summary>A bond option.</summary>Please see the official kernel documentation.</details>  | |
|`adUserPortKey` |uint16 |<details><summary>A bond option.</summary>Please see the official kernel documentation.</details>  | |
|`peerNotifyDelay` |uint32 |<details><summary>A bond option.</summary>Please see the official kernel documentation.</details>  | |



---
## STP
STP contains the various options for configuring the STP properties of a bridge interface.

Appears in:

- <code><a href="#bridge">Bridge</a>.stp</code>




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`enabled` |bool |Whether Spanning Tree Protocol (STP) is enabled.  | |



---
## Bridge
Bridge contains the various options for configuring a bridge interface.

Appears in:

- <code><a href="#device">Device</a>.bridge</code>



{{< highlight yaml >}}
# The interfaces that make up the bridge.
interfaces:
    - eth0
    - eth1
# A bridge option.
stp:
    enabled: true # Whether Spanning Tree Protocol (STP) is enabled.
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`interfaces` |[]string |The interfaces that make up the bridge.  | |
|`stp` |<a href="#stp">STP</a> |<details><summary>A bridge option.</summary>Please see the official kernel documentation.</details>  | |



---
## Vlan
Vlan represents vlan settings for a device.

Appears in:

- <code><a href="#device">Device</a>.vlans</code>




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`addresses` |[]string |The addresses in CIDR notation or as plain IPs to use.  | |
|`routes` |[]<a href="#route">Route</a> |A list of routes associated with the VLAN.  | |
|`dhcp` |bool |Indicates if DHCP should be used.  | |
|`vlanId` |uint16 |The VLAN's ID.  | |
|`mtu` |uint32 |The VLAN's MTU.  | |
|`vip` |<a href="#devicevipconfig">DeviceVIPConfig</a> |The VLAN's virtual IP address configuration.  | |
|`dhcpOptions` |<a href="#dhcpoptions">DHCPOptions</a> |<details><summary>DHCP specific options.</summary>`dhcp` *must* be set to true for these to take effect.</details>  | |



---
## Route
Route represents a network route.

Appears in:

- <code><a href="#device">Device</a>.routes</code>
- <code><a href="#vlan">Vlan</a>.routes</code>



{{< highlight yaml >}}
- network: 0.0.0.0/0 # The route's network (destination).
  gateway: 10.5.0.1 # The route's gateway (if empty, creates link scope route).
- network: 10.2.0.0/16 # The route's network (destination).
  gateway: 10.2.0.1 # The route's gateway (if empty, creates link scope route).
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`network` |string |The route's network (destination).  | |
|`gateway` |string |The route's gateway (if empty, creates link scope route).  | |
|`source` |string |The route's source address (optional).  | |
|`metric` |uint32 |The optional metric for the route.  | |



---
## RegistryMirrorConfig
RegistryMirrorConfig represents mirror configuration for a registry.

Appears in:

- <code><a href="#registriesconfig">RegistriesConfig</a>.mirrors</code>



{{< highlight yaml >}}
ghcr.io:
    # List of endpoints (URLs) for registry mirrors to use.
    endpoints:
        - https://registry.insecure
        - https://ghcr.io/v2/
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`endpoints` |[]string |<details><summary>List of endpoints (URLs) for registry mirrors to use.</summary>Endpoint configures HTTP/HTTPS access mode, host name,<br />port and path (if path is not set, it defaults to `/v2`).</details>  | |



---
## RegistryConfig
RegistryConfig specifies auth & TLS config per registry.

Appears in:

- <code><a href="#registriesconfig">RegistriesConfig</a>.config</code>



{{< highlight yaml >}}
registry.insecure:
    # The TLS configuration for the registry.
    tls:
        insecureSkipVerify: true # Skip TLS server certificate verification (not recommended).

        # # Enable mutual TLS authentication with the registry.
        # clientIdentity:
        #     crt: LS0tIEVYQU1QTEUgQ0VSVElGSUNBVEUgLS0t
        #     key: LS0tIEVYQU1QTEUgS0VZIC0tLQ==

    # # The auth configuration for this registry.
    # auth:
    #     username: username # Optional registry authentication.
    #     password: password # Optional registry authentication.
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`tls` |<a href="#registrytlsconfig">RegistryTLSConfig</a> |The TLS configuration for the registry. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
tls:
    # Enable mutual TLS authentication with the registry.
    clientIdentity:
        crt: LS0tIEVYQU1QTEUgQ0VSVElGSUNBVEUgLS0t
        key: LS0tIEVYQU1QTEUgS0VZIC0tLQ==
{{< /highlight >}}{{< highlight yaml >}}
tls:
    insecureSkipVerify: true # Skip TLS server certificate verification (not recommended).

    # # Enable mutual TLS authentication with the registry.
    # clientIdentity:
    #     crt: LS0tIEVYQU1QTEUgQ0VSVElGSUNBVEUgLS0t
    #     key: LS0tIEVYQU1QTEUgS0VZIC0tLQ==
{{< /highlight >}}</details> | |
|`auth` |<a href="#registryauthconfig">RegistryAuthConfig</a> |<details><summary>The auth configuration for this registry.</summary>Note: changes to the registry auth will not be picked up by the CRI containerd plugin without a reboot.</details> <details><summary>Show example(s)</summary>{{< highlight yaml >}}
auth:
    username: username # Optional registry authentication.
    password: password # Optional registry authentication.
{{< /highlight >}}</details> | |



---
## RegistryAuthConfig
RegistryAuthConfig specifies authentication configuration for a registry.

Appears in:

- <code><a href="#registryconfig">RegistryConfig</a>.auth</code>



{{< highlight yaml >}}
username: username # Optional registry authentication.
password: password # Optional registry authentication.
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`username` |string |<details><summary>Optional registry authentication.</summary>The meaning of each field is the same with the corresponding field in .docker/config.json.</details>  | |
|`password` |string |<details><summary>Optional registry authentication.</summary>The meaning of each field is the same with the corresponding field in .docker/config.json.</details>  | |
|`auth` |string |<details><summary>Optional registry authentication.</summary>The meaning of each field is the same with the corresponding field in .docker/config.json.</details>  | |
|`identityToken` |string |<details><summary>Optional registry authentication.</summary>The meaning of each field is the same with the corresponding field in .docker/config.json.</details>  | |



---
## RegistryTLSConfig
RegistryTLSConfig specifies TLS config for HTTPS registries.

Appears in:

- <code><a href="#registryconfig">RegistryConfig</a>.tls</code>



{{< highlight yaml >}}
# Enable mutual TLS authentication with the registry.
clientIdentity:
    crt: LS0tIEVYQU1QTEUgQ0VSVElGSUNBVEUgLS0t
    key: LS0tIEVYQU1QTEUgS0VZIC0tLQ==
{{< /highlight >}}

{{< highlight yaml >}}
insecureSkipVerify: true # Skip TLS server certificate verification (not recommended).

# # Enable mutual TLS authentication with the registry.
# clientIdentity:
#     crt: LS0tIEVYQU1QTEUgQ0VSVElGSUNBVEUgLS0t
#     key: LS0tIEVYQU1QTEUgS0VZIC0tLQ==
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`clientIdentity` |PEMEncodedCertificateAndKey |<details><summary>Enable mutual TLS authentication with the registry.</summary>Client certificate and key should be base64-encoded.</details> <details><summary>Show example(s)</summary>{{< highlight yaml >}}
clientIdentity:
    crt: LS0tIEVYQU1QTEUgQ0VSVElGSUNBVEUgLS0t
    key: LS0tIEVYQU1QTEUgS0VZIC0tLQ==
{{< /highlight >}}</details> | |
|`ca` |Base64Bytes |<details><summary>CA registry certificate to add the list of trusted certificates.</summary>Certificate should be base64-encoded.</details>  | |
|`insecureSkipVerify` |bool |Skip TLS server certificate verification (not recommended).  | |



---
## SystemDiskEncryptionConfig
SystemDiskEncryptionConfig specifies system disk partitions encryption settings.

Appears in:

- <code><a href="#machineconfig">MachineConfig</a>.systemDiskEncryption</code>



{{< highlight yaml >}}
# Ephemeral partition encryption.
ephemeral:
    provider: luks2 # Encryption provider to use for the encryption.
    # Defines the encryption keys generation and storage method.
    keys:
        - # Deterministically generated key from the node UUID and PartitionLabel.
          nodeID: {}
          slot: 0 # Key slot number for LUKS2 encryption.

    # # Cipher kind to use for the encryption. Depends on the encryption provider.
    # cipher: aes-xts-plain64

    # # Defines the encryption sector size.
    # blockSize: 4096

    # # Additional --perf parameters for the LUKS2 encryption.
    # options:
    #     - no_read_workqueue
    #     - no_write_workqueue
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`state` |<a href="#encryptionconfig">EncryptionConfig</a> |State partition encryption.  | |
|`ephemeral` |<a href="#encryptionconfig">EncryptionConfig</a> |Ephemeral partition encryption.  | |



---
## FeaturesConfig
FeaturesConfig describes individual Talos features that can be switched on or off.

Appears in:

- <code><a href="#machineconfig">MachineConfig</a>.features</code>



{{< highlight yaml >}}
rbac: true # Enable role-based access control (RBAC).

# # Configure Talos API access from Kubernetes pods.
# kubernetesTalosAPIAccess:
#     enabled: true # Enable Talos API access from Kubernetes pods.
#     # The list of Talos API roles which can be granted for access from Kubernetes pods.
#     allowedRoles:
#         - os:reader
#     # The list of Kubernetes namespaces Talos API access is available from.
#     allowedKubernetesNamespaces:
#         - kube-system
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`rbac` |bool |Enable role-based access control (RBAC).  | |
|`stableHostname` |bool |Enable stable default hostname.  | |
|`kubernetesTalosAPIAccess` |<a href="#kubernetestalosapiaccessconfig">KubernetesTalosAPIAccessConfig</a> |<details><summary>Configure Talos API access from Kubernetes pods.</summary><br />This feature is disabled if the feature config is not specified.</details> <details><summary>Show example(s)</summary>{{< highlight yaml >}}
kubernetesTalosAPIAccess:
    enabled: true # Enable Talos API access from Kubernetes pods.
    # The list of Talos API roles which can be granted for access from Kubernetes pods.
    allowedRoles:
        - os:reader
    # The list of Kubernetes namespaces Talos API access is available from.
    allowedKubernetesNamespaces:
        - kube-system
{{< /highlight >}}</details> | |
|`apidCheckExtKeyUsage` |bool |Enable checks for extended key usage of client certificates in apid.  | |



---
## KubernetesTalosAPIAccessConfig
KubernetesTalosAPIAccessConfig describes the configuration for the Talos API access from Kubernetes pods.

Appears in:

- <code><a href="#featuresconfig">FeaturesConfig</a>.kubernetesTalosAPIAccess</code>



{{< highlight yaml >}}
enabled: true # Enable Talos API access from Kubernetes pods.
# The list of Talos API roles which can be granted for access from Kubernetes pods.
allowedRoles:
    - os:reader
# The list of Kubernetes namespaces Talos API access is available from.
allowedKubernetesNamespaces:
    - kube-system
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`enabled` |bool |Enable Talos API access from Kubernetes pods.  | |
|`allowedRoles` |[]string |<details><summary>The list of Talos API roles which can be granted for access from Kubernetes pods.</summary><br />Empty list means that no roles can be granted, so access is blocked.</details>  | |
|`allowedKubernetesNamespaces` |[]string |The list of Kubernetes namespaces Talos API access is available from.  | |



---
## VolumeMountConfig
VolumeMountConfig struct describes extra volume mount for the static pods.

Appears in:

- <code><a href="#apiserverconfig">APIServerConfig</a>.extraVolumes</code>
- <code><a href="#controllermanagerconfig">ControllerManagerConfig</a>.extraVolumes</code>
- <code><a href="#schedulerconfig">SchedulerConfig</a>.extraVolumes</code>




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`hostPath` |string |Path on the host. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
hostPath: /var/lib/auth
{{< /highlight >}}</details> | |
|`mountPath` |string |Path in the container. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
mountPath: /etc/kubernetes/auth
{{< /highlight >}}</details> | |
|`readonly` |bool |Mount the volume read only. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
readonly: true
{{< /highlight >}}</details> | |



---
## ClusterInlineManifest
ClusterInlineManifest struct describes inline bootstrap manifests for the user.





| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`name` |string |<details><summary>Name of the manifest.</summary>Name should be unique.</details> <details><summary>Show example(s)</summary>{{< highlight yaml >}}
name: csi
{{< /highlight >}}</details> | |
|`contents` |string |Manifest contents as a string. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
contents: /etc/kubernetes/auth
{{< /highlight >}}</details> | |



---
## NetworkKubeSpan
NetworkKubeSpan struct describes KubeSpan configuration.

Appears in:

- <code><a href="#networkconfig">NetworkConfig</a>.kubespan</code>



{{< highlight yaml >}}
enabled: true # Enable the KubeSpan feature.
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`enabled` |bool |<details><summary>Enable the KubeSpan feature.</summary>Cluster discovery should be enabled with .cluster.discovery.enabled for KubeSpan to be enabled.</details>  | |
|`advertiseKubernetesNetworks` |bool |<details><summary>Control whether Kubernetes pod CIDRs are announced over KubeSpan from the node.</summary>If disabled, CNI handles encapsulating pod-to-pod traffic into some node-to-node tunnel,<br />and KubeSpan handles the node-to-node traffic.<br />If enabled, KubeSpan will take over pod-to-pod traffic and send it over KubeSpan directly.<br />When enabled, KubeSpan should have a way to detect complete pod CIDRs of the node which<br />is not always the case with CNIs not relying on Kubernetes for IPAM.</details>  | |
|`allowDownPeerBypass` |bool |<details><summary>Skip sending traffic via KubeSpan if the peer connection state is not up.</summary>This provides configurable choice between connectivity and security: either traffic is always<br />forced to go via KubeSpan (even if Wireguard peer connection is not up), or traffic can go directly<br />to the peer if Wireguard connection can't be established.</details>  | |



---
## NetworkDeviceSelector
NetworkDeviceSelector struct describes network device selector.

Appears in:

- <code><a href="#device">Device</a>.deviceSelector</code>



{{< highlight yaml >}}
busPath: 00:* # PCI, USB bus prefix, supports matching by wildcard.
{{< /highlight >}}

{{< highlight yaml >}}
hardwareAddr: '*:f0:ab' # Device hardware address, supports matching by wildcard.
driver: virtio # Kernel driver, supports matching by wildcard.
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`busPath` |string |PCI, USB bus prefix, supports matching by wildcard.  | |
|`hardwareAddr` |string |Device hardware address, supports matching by wildcard.  | |
|`pciID` |string |PCI ID (vendor ID, product ID), supports matching by wildcard.  | |
|`driver` |string |Kernel driver, supports matching by wildcard.  | |



---
## ClusterDiscoveryConfig
ClusterDiscoveryConfig struct configures cluster membership discovery.

Appears in:

- <code><a href="#clusterconfig">ClusterConfig</a>.discovery</code>



{{< highlight yaml >}}
enabled: true # Enable the cluster membership discovery feature.
# Configure registries used for cluster member discovery.
registries:
    # Kubernetes registry uses Kubernetes API server to discover cluster members and stores additional information
    kubernetes: {}
    # Service registry is using an external service to push and pull information about cluster members.
    service:
        endpoint: https://discovery.talos.dev/ # External service endpoint.
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`enabled` |bool |<details><summary>Enable the cluster membership discovery feature.</summary>Cluster discovery is based on individual registries which are configured under the registries field.</details>  | |
|`registries` |<a href="#discoveryregistriesconfig">DiscoveryRegistriesConfig</a> |Configure registries used for cluster member discovery.  | |



---
## DiscoveryRegistriesConfig
DiscoveryRegistriesConfig struct configures cluster membership discovery.

Appears in:

- <code><a href="#clusterdiscoveryconfig">ClusterDiscoveryConfig</a>.registries</code>




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`kubernetes` |<a href="#registrykubernetesconfig">RegistryKubernetesConfig</a> |<details><summary>Kubernetes registry uses Kubernetes API server to discover cluster members and stores additional information</summary>as annotations on the Node resources.</details>  | |
|`service` |<a href="#registryserviceconfig">RegistryServiceConfig</a> |Service registry is using an external service to push and pull information about cluster members.  | |



---
## RegistryKubernetesConfig
RegistryKubernetesConfig struct configures Kubernetes discovery registry.

Appears in:

- <code><a href="#discoveryregistriesconfig">DiscoveryRegistriesConfig</a>.kubernetes</code>




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`disabled` |bool |Disable Kubernetes discovery registry.  | |



---
## RegistryServiceConfig
RegistryServiceConfig struct configures Kubernetes discovery registry.

Appears in:

- <code><a href="#discoveryregistriesconfig">DiscoveryRegistriesConfig</a>.service</code>




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`disabled` |bool |Disable external service discovery registry.  | |
|`endpoint` |string |External service endpoint. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
endpoint: https://discovery.talos.dev/
{{< /highlight >}}</details> | |



---
## UdevConfig
UdevConfig describes how the udev system should be configured.

Appears in:

- <code><a href="#machineconfig">MachineConfig</a>.udev</code>



{{< highlight yaml >}}
# List of udev rules to apply to the udev system
rules:
    - SUBSYSTEM=="drm", KERNEL=="renderD*", GROUP="44", MODE="0660"
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`rules` |[]string |List of udev rules to apply to the udev system  | |



---
## LoggingConfig
LoggingConfig struct configures Talos logging.

Appears in:

- <code><a href="#machineconfig">MachineConfig</a>.logging</code>



{{< highlight yaml >}}
# Logging destination.
destinations:
    - endpoint: tcp://1.2.3.4:12345 # Where to send logs. Supported protocols are "tcp" and "udp".
      format: json_lines # Logs format.
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`destinations` |[]<a href="#loggingdestination">LoggingDestination</a> |Logging destination.  | |



---
## LoggingDestination
LoggingDestination struct configures Talos logging destination.

Appears in:

- <code><a href="#loggingconfig">LoggingConfig</a>.destinations</code>




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`endpoint` |<a href="#endpoint">Endpoint</a> |Where to send logs. Supported protocols are "tcp" and "udp". <details><summary>Show example(s)</summary>{{< highlight yaml >}}
endpoint: udp://127.0.0.1:12345
{{< /highlight >}}{{< highlight yaml >}}
endpoint: tcp://1.2.3.4:12345
{{< /highlight >}}</details> | |
|`format` |string |Logs format.  |`json_lines`<br /> |



---
## KernelConfig
KernelConfig struct configures Talos Linux kernel.

Appears in:

- <code><a href="#machineconfig">MachineConfig</a>.kernel</code>



{{< highlight yaml >}}
# Kernel modules to load.
modules:
    - name: brtfs # Module name.
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`modules` |[]<a href="#kernelmoduleconfig">KernelModuleConfig</a> |Kernel modules to load.  | |



---
## KernelModuleConfig
KernelModuleConfig struct configures Linux kernel modules to load.

Appears in:

- <code><a href="#kernelconfig">KernelConfig</a>.modules</code>




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`name` |string |Module name.  | |
|`parameters` |[]string |Module parameters, changes applied after reboot.  | |


