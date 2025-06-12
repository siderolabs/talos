---
description: Config defines the v1alpha1.Config Talos machine configuration document.
title: Config
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
version: v1alpha1
machine: # ...
cluster: # ...
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`version` |string |Indicates the schema used to decode the contents.  |`v1alpha1`<br /> |
|`debug` |bool |Enable verbose logging to the console.<br>All system containers logs will flow into serial console.<br><br>**Note:** To avoid breaking Talos bootstrap flow enable this option only if serial console can handle high message throughput.  |`true`<br />`yes`<br />`false`<br />`no`<br /> |
|`machine` |<a href="#Config.machine">MachineConfig</a> |Provides machine specific configuration options.  | |
|`cluster` |<a href="#Config.cluster">ClusterConfig</a> |Provides cluster specific configuration options.  | |




## machine {#Config.machine}

MachineConfig represents the machine-specific config values.



{{< highlight yaml >}}
machine:
    type: controlplane
    # InstallConfig represents the installation options for preparing a node.
    install:
        disk: /dev/sda # The disk used for installations.
        # Allows for supplying extra kernel args via the bootloader.
        extraKernelArgs:
            - console=ttyS1
            - panic=10
        image: ghcr.io/siderolabs/installer:latest # Allows for supplying the image used to perform the installation.
        wipe: false # Indicates if the installation disk should be wiped at installation time.

        # # Look up disk using disk attributes like model, size, serial and others.
        # diskSelector:
        #     size: 4GB # Disk size.
        #     model: WDC* # Disk model `/sys/block/<dev>/device/model`.
        #     busPath: /pci0000:00/0000:00:17.0/ata1/host0/target0:0:0/0:0:0:0 # Disk bus path.
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`type` |string |Defines the role of the machine within the cluster.<br><br>**Control Plane**<br><br>Control Plane node type designates the node as a control plane member.<br>This means it will host etcd along with the Kubernetes controlplane components such as API Server, Controller Manager, Scheduler.<br><br>**Worker**<br><br>Worker node type designates the node as a worker node.<br>This means it will be an available compute node for scheduling workloads.<br><br>This node type was previously known as "join"; that value is still supported but deprecated.  |`controlplane`<br />`worker`<br /> |
|`token` |string |The `token` is used by a machine to join the PKI of the cluster.<br>Using this token, a machine will create a certificate signing request (CSR), and request a certificate that will be used as its' identity. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
token: 328hom.uqjzh6jnn2eie9oi
{{< /highlight >}}</details> | |
|`ca` |PEMEncodedCertificateAndKey |The root certificate authority of the PKI.<br>It is composed of a base64 encoded `crt` and `key`. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
ca:
    crt: LS0tIEVYQU1QTEUgQ0VSVElGSUNBVEUgLS0t
    key: LS0tIEVYQU1QTEUgS0VZIC0tLQ==
{{< /highlight >}}</details> | |
|`acceptedCAs` |[]PEMEncodedCertificate |The certificates issued by certificate authorities are accepted in addition to issuing 'ca'.<br>It is composed of a base64 encoded `crt``.  | |
|`certSANs` |[]string |Extra certificate subject alternative names for the machine's certificate.<br>By default, all non-loopback interface IPs are automatically added to the certificate's SANs. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
certSANs:
    - 10.0.0.10
    - 172.16.0.10
    - 192.168.0.10
{{< /highlight >}}</details> | |
|`controlPlane` |<a href="#Config.machine.controlPlane">MachineControlPlaneConfig</a> |Provides machine specific control plane configuration options. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
controlPlane:
    # Controller manager machine specific configuration options.
    controllerManager:
        disabled: false # Disable kube-controller-manager on the node.
    # Scheduler machine specific configuration options.
    scheduler:
        disabled: true # Disable kube-scheduler on the node.
{{< /highlight >}}</details> | |
|`kubelet` |<a href="#Config.machine.kubelet">KubeletConfig</a> |Used to provide additional options to the kubelet. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
kubelet:
    image: ghcr.io/siderolabs/kubelet:v1.33.1 # The `image` field is an optional reference to an alternative kubelet image.
    # The `extraArgs` field is used to provide additional flags to the kubelet.
    extraArgs:
        feature-gates: ServerSideApply=true

    # # The `ClusterDNS` field is an optional reference to an alternative kubelet clusterDNS ip list.
    # clusterDNS:
    #     - 10.96.0.10
    #     - 169.254.2.53

    # # The `extraMounts` field is used to add additional mounts to the kubelet container.
    # extraMounts:
    #     - destination: /var/lib/example # Destination is the absolute path where the mount will be placed in the container.
    #       type: bind # Type specifies the mount kind.
    #       source: /var/lib/example # Source specifies the source path of the mount.
    #       # Options are fstab style mount options.
    #       options:
    #         - bind
    #         - rshared
    #         - rw

    # # The `extraConfig` field is used to provide kubelet configuration overrides.
    # extraConfig:
    #     serverTLSBootstrap: true

    # # The `KubeletCredentialProviderConfig` field is used to provide kubelet credential configuration.
    # credentialProviderConfig:
    #     apiVersion: kubelet.config.k8s.io/v1
    #     kind: CredentialProviderConfig
    #     providers:
    #         - apiVersion: credentialprovider.kubelet.k8s.io/v1
    #           defaultCacheDuration: 12h
    #           matchImages:
    #             - '*.dkr.ecr.*.amazonaws.com'
    #             - '*.dkr.ecr.*.amazonaws.com.cn'
    #             - '*.dkr.ecr-fips.*.amazonaws.com'
    #             - '*.dkr.ecr.us-iso-east-1.c2s.ic.gov'
    #             - '*.dkr.ecr.us-isob-east-1.sc2s.sgov.gov'
    #           name: ecr-credential-provider

    # # The `nodeIP` field is used to configure `--node-ip` flag for the kubelet.
    # nodeIP:
    #     # The `validSubnets` field configures the networks to pick kubelet node IP from.
    #     validSubnets:
    #         - 10.0.0.0/8
    #         - '!10.0.0.3/32'
    #         - fdc7::/16
{{< /highlight >}}</details> | |
|`pods` |[]Unstructured |Used to provide static pod definitions to be run by the kubelet directly bypassing the kube-apiserver.<br><br>Static pods can be used to run components which should be started before the Kubernetes control plane is up.<br>Talos doesn't validate the pod definition.<br>Updates to this field can be applied without a reboot.<br><br>See https://kubernetes.io/docs/tasks/configure-pod-container/static-pod/. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
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
|`network` |<a href="#Config.machine.network">NetworkConfig</a> |Provides machine specific network configuration options. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
network:
    hostname: worker-1 # Used to statically set the hostname for the machine.
    # `interfaces` is used to define the network interface configuration.
    interfaces:
        - interface: enp0s1 # The interface name.
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
          #     hardwareAddr: '*:f0:ab' # Device hardware (MAC) address, supports matching by wildcard.
          #     driver: virtio_net # Kernel driver, supports matching by wildcard.
          # # select a device with bus prefix 00:*, a device with mac address matching `*:f0:ab` and `virtio` kernel driver.
          # deviceSelector:
          #     - busPath: 00:* # PCI, USB bus prefix, supports matching by wildcard.
          #     - hardwareAddr: '*:f0:ab' # Device hardware (MAC) address, supports matching by wildcard.
          #       driver: virtio_net # Kernel driver, supports matching by wildcard.

          # # Bond specific options.
          # bond:
          #     # The interfaces that make up the bond.
          #     interfaces:
          #         - enp2s0
          #         - enp2s1
          #     # Picks a network device using the selector.
          #     deviceSelectors:
          #         - busPath: 00:* # PCI, USB bus prefix, supports matching by wildcard.
          #         - hardwareAddr: '*:f0:ab' # Device hardware (MAC) address, supports matching by wildcard.
          #           driver: virtio_net # Kernel driver, supports matching by wildcard.
          #     mode: 802.3ad # A bond option.
          #     lacpRate: fast # A bond option.

          # # Bridge specific options.
          # bridge:
          #     # The interfaces that make up the bridge.
          #     interfaces:
          #         - enxda4042ca9a51
          #         - enxae2a6774c259
          #     # Enable STP on this bridge.
          #     stp:
          #         enabled: true # Whether Spanning Tree Protocol (STP) is enabled.

          # # Configure this device as a bridge port.
          # bridgePort:
          #     master: br0 # The name of the bridge master interface

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
          #           endpoint: 192.168.1.2:51822 # Specifies the endpoint of this peer entry.
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
    # Used to statically set arbitrary search domains.
    searchDomains:
        - example.org
        - example.com

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
|`install` |<a href="#Config.machine.install">InstallConfig</a> |Used to provide instructions for installations.<br><br>Note that this configuration section gets silently ignored by Talos images that are considered pre-installed.<br>To make sure Talos installs according to the provided configuration, Talos should be booted with ISO or PXE-booted. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
install:
    disk: /dev/sda # The disk used for installations.
    # Allows for supplying extra kernel args via the bootloader.
    extraKernelArgs:
        - console=ttyS1
        - panic=10
    image: ghcr.io/siderolabs/installer:latest # Allows for supplying the image used to perform the installation.
    wipe: false # Indicates if the installation disk should be wiped at installation time.

    # # Look up disk using disk attributes like model, size, serial and others.
    # diskSelector:
    #     size: 4GB # Disk size.
    #     model: WDC* # Disk model `/sys/block/<dev>/device/model`.
    #     busPath: /pci0000:00/0000:00:17.0/ata1/host0/target0:0:0/0:0:0:0 # Disk bus path.
{{< /highlight >}}</details> | |
|`files` |<a href="#Config.machine.files.">[]MachineFile</a> |Allows the addition of user specified files.<br>The value of `op` can be `create`, `overwrite`, or `append`.<br>In the case of `create`, `path` must not exist.<br>In the case of `overwrite`, and `append`, `path` must be a valid file.<br>If an `op` value of `append` is used, the existing file will be appended.<br>Note that the file contents are not required to be base64 encoded. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
files:
    - content: '...' # The contents of the file.
      permissions: 0o666 # The file's permissions in octal.
      path: /tmp/file.txt # The path of the file.
      op: append # The operation to use
{{< /highlight >}}</details> | |
|`env` |Env |The `env` field allows for the addition of environment variables.<br>All environment variables are set on PID 1 in addition to every service. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
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
|`time` |<a href="#Config.machine.time">TimeConfig</a> |Used to configure the machine's time settings. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
time:
    disabled: false # Indicates if the time service is disabled for the machine.
    # description: |
    servers:
        - time.cloudflare.com
    bootTimeout: 2m0s # Specifies the timeout when the node time is considered to be in sync unlocking the boot sequence.
{{< /highlight >}}</details> | |
|`sysctls` |map[string]string |Used to configure the machine's sysctls. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
sysctls:
    kernel.domainname: talos.dev
    net.ipv4.ip_forward: "0"
    net/ipv6/conf/eth0.100/disable_ipv6: "1"
{{< /highlight >}}</details> | |
|`sysfs` |map[string]string |Used to configure the machine's sysfs. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
sysfs:
    devices.system.cpu.cpu0.cpufreq.scaling_governor: performance
{{< /highlight >}}</details> | |
|`registries` |<a href="#Config.machine.registries">RegistriesConfig</a> |Used to configure the machine's container image registry mirrors.<br><br>Automatically generates matching CRI configuration for registry mirrors.<br><br>The `mirrors` section allows to redirect requests for images to a non-default registry,<br>which might be a local registry or a caching mirror.<br><br>The `config` section provides a way to authenticate to the registry with TLS client<br>identity, provide registry CA, or authentication information.<br>Authentication information has same meaning with the corresponding field in [`.docker/config.json`](https://docs.docker.com/engine/api/v1.41/#section/Authentication).<br><br>See also matching configuration for [CRI containerd plugin](https://github.com/containerd/cri/blob/master/docs/registry.md). <details><summary>Show example(s)</summary>{{< highlight yaml >}}
registries:
    # Specifies mirror configuration for each registry host namespace.
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
|`systemDiskEncryption` |<a href="#Config.machine.systemDiskEncryption">SystemDiskEncryptionConfig</a> |Machine system disk encryption configuration.<br>Defines each system partition encryption parameters. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
systemDiskEncryption:
    # Ephemeral partition encryption.
    ephemeral:
        provider: luks2 # Encryption provider to use for the encryption.
        # Defines the encryption keys generation and storage method.
        keys:
            - # Deterministically generated key from the node UUID and PartitionLabel.
              nodeID: {}
              slot: 0 # Key slot number for LUKS2 encryption.

              # # KMS managed encryption key.
              # kms:
              #     endpoint: https://192.168.88.21:4443 # KMS endpoint to Seal/Unseal the key.

        # # Cipher kind to use for the encryption. Depends on the encryption provider.
        # cipher: aes-xts-plain64

        # # Defines the encryption sector size.
        # blockSize: 4096

        # # Additional --perf parameters for the LUKS2 encryption.
        # options:
        #     - no_read_workqueue
        #     - no_write_workqueue
{{< /highlight >}}</details> | |
|`features` |<a href="#Config.machine.features">FeaturesConfig</a> |Features describe individual Talos features that can be switched on or off. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
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
|`udev` |<a href="#Config.machine.udev">UdevConfig</a> |Configures the udev system. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
udev:
    # List of udev rules to apply to the udev system
    rules:
        - SUBSYSTEM=="drm", KERNEL=="renderD*", GROUP="44", MODE="0660"
{{< /highlight >}}</details> | |
|`logging` |<a href="#Config.machine.logging">LoggingConfig</a> |Configures the logging system. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
logging:
    # Logging destination.
    destinations:
        - endpoint: tcp://1.2.3.4:12345 # Where to send logs. Supported protocols are "tcp" and "udp".
          format: json_lines # Logs format.
{{< /highlight >}}</details> | |
|`kernel` |<a href="#Config.machine.kernel">KernelConfig</a> |Configures the kernel. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
kernel:
    # Kernel modules to load.
    modules:
        - name: brtfs # Module name.
{{< /highlight >}}</details> | |
|`seccompProfiles` |<a href="#Config.machine.seccompProfiles.">[]MachineSeccompProfile</a> |Configures the seccomp profiles for the machine. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
seccompProfiles:
    - name: audit.json # The `name` field is used to provide the file name of the seccomp profile.
      # The `value` field is used to provide the seccomp profile.
      value:
        defaultAction: SCMP_ACT_LOG
{{< /highlight >}}</details> | |
|`baseRuntimeSpecOverrides` |Unstructured |Override (patch) settings in the default OCI runtime spec for CRI containers.<br><br>It can be used to set some default container settings which are not configurable in Kubernetes,<br>for example default ulimits.<br>Note: this change applies to all newly created containers, and it requires a reboot to take effect. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
baseRuntimeSpecOverrides:
    process:
        rlimits:
            - hard: 1024
              soft: 1024
              type: RLIMIT_NOFILE
{{< /highlight >}}</details> | |
|`nodeLabels` |map[string]string |Configures the node labels for the machine.<br><br>Note: In the default Kubernetes configuration, worker nodes are restricted to set<br>labels with some prefixes (see [NodeRestriction](https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/#noderestriction) admission plugin). <details><summary>Show example(s)</summary>{{< highlight yaml >}}
nodeLabels:
    exampleLabel: exampleLabelValue
{{< /highlight >}}</details> | |
|`nodeAnnotations` |map[string]string |Configures the node annotations for the machine. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
nodeAnnotations:
    customer.io/rack: r13a25
{{< /highlight >}}</details> | |
|`nodeTaints` |map[string]string |Configures the node taints for the machine. Effect is optional.<br><br>Note: In the default Kubernetes configuration, worker nodes are not allowed to<br>modify the taints (see [NodeRestriction](https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/#noderestriction) admission plugin). <details><summary>Show example(s)</summary>{{< highlight yaml >}}
nodeTaints:
    exampleTaint: exampleTaintValue:NoSchedule
{{< /highlight >}}</details> | |




### controlPlane {#Config.machine.controlPlane}

MachineControlPlaneConfig machine specific configuration options.



{{< highlight yaml >}}
machine:
    controlPlane:
        # Controller manager machine specific configuration options.
        controllerManager:
            disabled: false # Disable kube-controller-manager on the node.
        # Scheduler machine specific configuration options.
        scheduler:
            disabled: true # Disable kube-scheduler on the node.
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`controllerManager` |<a href="#Config.machine.controlPlane.controllerManager">MachineControllerManagerConfig</a> |Controller manager machine specific configuration options.  | |
|`scheduler` |<a href="#Config.machine.controlPlane.scheduler">MachineSchedulerConfig</a> |Scheduler machine specific configuration options.  | |




#### controllerManager {#Config.machine.controlPlane.controllerManager}

MachineControllerManagerConfig represents the machine specific ControllerManager config values.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`disabled` |bool |Disable kube-controller-manager on the node.  | |






#### scheduler {#Config.machine.controlPlane.scheduler}

MachineSchedulerConfig represents the machine specific Scheduler config values.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`disabled` |bool |Disable kube-scheduler on the node.  | |








### kubelet {#Config.machine.kubelet}

KubeletConfig represents the kubelet config values.



{{< highlight yaml >}}
machine:
    kubelet:
        image: ghcr.io/siderolabs/kubelet:v1.33.1 # The `image` field is an optional reference to an alternative kubelet image.
        # The `extraArgs` field is used to provide additional flags to the kubelet.
        extraArgs:
            feature-gates: ServerSideApply=true

        # # The `ClusterDNS` field is an optional reference to an alternative kubelet clusterDNS ip list.
        # clusterDNS:
        #     - 10.96.0.10
        #     - 169.254.2.53

        # # The `extraMounts` field is used to add additional mounts to the kubelet container.
        # extraMounts:
        #     - destination: /var/lib/example # Destination is the absolute path where the mount will be placed in the container.
        #       type: bind # Type specifies the mount kind.
        #       source: /var/lib/example # Source specifies the source path of the mount.
        #       # Options are fstab style mount options.
        #       options:
        #         - bind
        #         - rshared
        #         - rw

        # # The `extraConfig` field is used to provide kubelet configuration overrides.
        # extraConfig:
        #     serverTLSBootstrap: true

        # # The `KubeletCredentialProviderConfig` field is used to provide kubelet credential configuration.
        # credentialProviderConfig:
        #     apiVersion: kubelet.config.k8s.io/v1
        #     kind: CredentialProviderConfig
        #     providers:
        #         - apiVersion: credentialprovider.kubelet.k8s.io/v1
        #           defaultCacheDuration: 12h
        #           matchImages:
        #             - '*.dkr.ecr.*.amazonaws.com'
        #             - '*.dkr.ecr.*.amazonaws.com.cn'
        #             - '*.dkr.ecr-fips.*.amazonaws.com'
        #             - '*.dkr.ecr.us-iso-east-1.c2s.ic.gov'
        #             - '*.dkr.ecr.us-isob-east-1.sc2s.sgov.gov'
        #           name: ecr-credential-provider

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
image: ghcr.io/siderolabs/kubelet:v1.33.1
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
|`extraMounts` |<a href="#Config.machine.kubelet.extraMounts.">[]ExtraMount</a> |The `extraMounts` field is used to add additional mounts to the kubelet container.<br>Note that either `bind` or `rbind` are required in the `options`. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
extraMounts:
    - destination: /var/lib/example # Destination is the absolute path where the mount will be placed in the container.
      type: bind # Type specifies the mount kind.
      source: /var/lib/example # Source specifies the source path of the mount.
      # Options are fstab style mount options.
      options:
        - bind
        - rshared
        - rw
{{< /highlight >}}</details> | |
|`extraConfig` |Unstructured |The `extraConfig` field is used to provide kubelet configuration overrides.<br><br>Some fields are not allowed to be overridden: authentication and authorization, cgroups<br>configuration, ports, etc. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
extraConfig:
    serverTLSBootstrap: true
{{< /highlight >}}</details> | |
|`credentialProviderConfig` |Unstructured |The `KubeletCredentialProviderConfig` field is used to provide kubelet credential configuration. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
credentialProviderConfig:
    apiVersion: kubelet.config.k8s.io/v1
    kind: CredentialProviderConfig
    providers:
        - apiVersion: credentialprovider.kubelet.k8s.io/v1
          defaultCacheDuration: 12h
          matchImages:
            - '*.dkr.ecr.*.amazonaws.com'
            - '*.dkr.ecr.*.amazonaws.com.cn'
            - '*.dkr.ecr-fips.*.amazonaws.com'
            - '*.dkr.ecr.us-iso-east-1.c2s.ic.gov'
            - '*.dkr.ecr.us-isob-east-1.sc2s.sgov.gov'
          name: ecr-credential-provider
{{< /highlight >}}</details> | |
|`defaultRuntimeSeccompProfileEnabled` |bool |Enable container runtime default Seccomp profile.  |`true`<br />`yes`<br />`false`<br />`no`<br /> |
|`registerWithFQDN` |bool |The `registerWithFQDN` field is used to force kubelet to use the node FQDN for registration.<br>This is required in clouds like AWS.  |`true`<br />`yes`<br />`false`<br />`no`<br /> |
|`nodeIP` |<a href="#Config.machine.kubelet.nodeIP">KubeletNodeIPConfig</a> |The `nodeIP` field is used to configure `--node-ip` flag for the kubelet.<br>This is used when a node has multiple addresses to choose from. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
nodeIP:
    # The `validSubnets` field configures the networks to pick kubelet node IP from.
    validSubnets:
        - 10.0.0.0/8
        - '!10.0.0.3/32'
        - fdc7::/16
{{< /highlight >}}</details> | |
|`skipNodeRegistration` |bool |The `skipNodeRegistration` is used to run the kubelet without registering with the apiserver.<br>This runs kubelet as standalone and only runs static pods.  |`true`<br />`yes`<br />`false`<br />`no`<br /> |
|`disableManifestsDirectory` |bool |The `disableManifestsDirectory` field configures the kubelet to get static pod manifests from the /etc/kubernetes/manifests directory.<br>It's recommended to configure static pods with the "pods" key instead.  |`true`<br />`yes`<br />`false`<br />`no`<br /> |




#### extraMounts[] {#Config.machine.kubelet.extraMounts.}

ExtraMount wraps OCI Mount specification.



{{< highlight yaml >}}
machine:
    kubelet:
        extraMounts:
            - destination: /var/lib/example # Destination is the absolute path where the mount will be placed in the container.
              type: bind # Type specifies the mount kind.
              source: /var/lib/example # Source specifies the source path of the mount.
              # Options are fstab style mount options.
              options:
                - bind
                - rshared
                - rw
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`destination` |string |Destination is the absolute path where the mount will be placed in the container.  | |
|`type` |string |Type specifies the mount kind.  | |
|`source` |string |Source specifies the source path of the mount.  | |
|`options` |[]string |Options are fstab style mount options.  | |
|`uidMappings` |<a href="#Config.machine.kubelet.extraMounts..uidMappings.">[]LinuxIDMapping</a> |UID/GID mappings used for changing file owners w/o calling chown, fs should support it.<br><br>Every mount point could have its own mapping.  | |
|`gidMappings` |<a href="#Config.machine.kubelet.extraMounts..gidMappings.">[]LinuxIDMapping</a> |UID/GID mappings used for changing file owners w/o calling chown, fs should support it.<br><br>Every mount point could have its own mapping.  | |




##### uidMappings[] {#Config.machine.kubelet.extraMounts..uidMappings.}

LinuxIDMapping represents the Linux ID mapping.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`containerID` |uint32 |ContainerID is the starting UID/GID in the container.  | |
|`hostID` |uint32 |HostID is the starting UID/GID on the host to be mapped to 'ContainerID'.  | |
|`size` |uint32 |Size is the number of IDs to be mapped.  | |






##### gidMappings[] {#Config.machine.kubelet.extraMounts..gidMappings.}

LinuxIDMapping represents the Linux ID mapping.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`containerID` |uint32 |ContainerID is the starting UID/GID in the container.  | |
|`hostID` |uint32 |HostID is the starting UID/GID on the host to be mapped to 'ContainerID'.  | |
|`size` |uint32 |Size is the number of IDs to be mapped.  | |








#### nodeIP {#Config.machine.kubelet.nodeIP}

KubeletNodeIPConfig represents the kubelet node IP configuration.



{{< highlight yaml >}}
machine:
    kubelet:
        nodeIP:
            # The `validSubnets` field configures the networks to pick kubelet node IP from.
            validSubnets:
                - 10.0.0.0/8
                - '!10.0.0.3/32'
                - fdc7::/16
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`validSubnets` |[]string |The `validSubnets` field configures the networks to pick kubelet node IP from.<br>For dual stack configuration, there should be two subnets: one for IPv4, another for IPv6.<br>IPs can be excluded from the list by using negative match with `!`, e.g `!10.0.0.0/8`.<br>Negative subnet matches should be specified last to filter out IPs picked by positive matches.<br>If not specified, node IP is picked based on cluster podCIDRs: IPv4/IPv6 address or both.  | |








### network {#Config.machine.network}

NetworkConfig represents the machine's networking config values.



{{< highlight yaml >}}
machine:
    network:
        hostname: worker-1 # Used to statically set the hostname for the machine.
        # `interfaces` is used to define the network interface configuration.
        interfaces:
            - interface: enp0s1 # The interface name.
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
              #     hardwareAddr: '*:f0:ab' # Device hardware (MAC) address, supports matching by wildcard.
              #     driver: virtio_net # Kernel driver, supports matching by wildcard.
              # # select a device with bus prefix 00:*, a device with mac address matching `*:f0:ab` and `virtio` kernel driver.
              # deviceSelector:
              #     - busPath: 00:* # PCI, USB bus prefix, supports matching by wildcard.
              #     - hardwareAddr: '*:f0:ab' # Device hardware (MAC) address, supports matching by wildcard.
              #       driver: virtio_net # Kernel driver, supports matching by wildcard.

              # # Bond specific options.
              # bond:
              #     # The interfaces that make up the bond.
              #     interfaces:
              #         - enp2s0
              #         - enp2s1
              #     # Picks a network device using the selector.
              #     deviceSelectors:
              #         - busPath: 00:* # PCI, USB bus prefix, supports matching by wildcard.
              #         - hardwareAddr: '*:f0:ab' # Device hardware (MAC) address, supports matching by wildcard.
              #           driver: virtio_net # Kernel driver, supports matching by wildcard.
              #     mode: 802.3ad # A bond option.
              #     lacpRate: fast # A bond option.

              # # Bridge specific options.
              # bridge:
              #     # The interfaces that make up the bridge.
              #     interfaces:
              #         - enxda4042ca9a51
              #         - enxae2a6774c259
              #     # Enable STP on this bridge.
              #     stp:
              #         enabled: true # Whether Spanning Tree Protocol (STP) is enabled.

              # # Configure this device as a bridge port.
              # bridgePort:
              #     master: br0 # The name of the bridge master interface

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
              #           endpoint: 192.168.1.2:51822 # Specifies the endpoint of this peer entry.
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
        # Used to statically set arbitrary search domains.
        searchDomains:
            - example.org
            - example.com

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
|`interfaces` |<a href="#Config.machine.network.interfaces.">[]Device</a> |`interfaces` is used to define the network interface configuration.<br>By default all network interfaces will attempt a DHCP discovery.<br>This can be further tuned through this configuration parameter. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
interfaces:
    - interface: enp0s1 # The interface name.
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
      #     hardwareAddr: '*:f0:ab' # Device hardware (MAC) address, supports matching by wildcard.
      #     driver: virtio_net # Kernel driver, supports matching by wildcard.
      # # select a device with bus prefix 00:*, a device with mac address matching `*:f0:ab` and `virtio` kernel driver.
      # deviceSelector:
      #     - busPath: 00:* # PCI, USB bus prefix, supports matching by wildcard.
      #     - hardwareAddr: '*:f0:ab' # Device hardware (MAC) address, supports matching by wildcard.
      #       driver: virtio_net # Kernel driver, supports matching by wildcard.

      # # Bond specific options.
      # bond:
      #     # The interfaces that make up the bond.
      #     interfaces:
      #         - enp2s0
      #         - enp2s1
      #     # Picks a network device using the selector.
      #     deviceSelectors:
      #         - busPath: 00:* # PCI, USB bus prefix, supports matching by wildcard.
      #         - hardwareAddr: '*:f0:ab' # Device hardware (MAC) address, supports matching by wildcard.
      #           driver: virtio_net # Kernel driver, supports matching by wildcard.
      #     mode: 802.3ad # A bond option.
      #     lacpRate: fast # A bond option.

      # # Bridge specific options.
      # bridge:
      #     # The interfaces that make up the bridge.
      #     interfaces:
      #         - enxda4042ca9a51
      #         - enxae2a6774c259
      #     # Enable STP on this bridge.
      #     stp:
      #         enabled: true # Whether Spanning Tree Protocol (STP) is enabled.

      # # Configure this device as a bridge port.
      # bridgePort:
      #     master: br0 # The name of the bridge master interface

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
      #           endpoint: 192.168.1.2:51822 # Specifies the endpoint of this peer entry.
      #           persistentKeepaliveInterval: 10s # Specifies the persistent keepalive interval for this peer.
      #           # AllowedIPs specifies a list of allowed IP addresses in CIDR notation for this peer.
      #           allowedIPs:
      #             - 192.168.1.0/24

      # # Virtual (shared) IP address configuration.

      # # layer2 vip example
      # vip:
      #     ip: 172.16.199.55 # Specifies the IP address to be used.
{{< /highlight >}}</details> | |
|`nameservers` |[]string |Used to statically set the nameservers for the machine.<br>Defaults to `1.1.1.1` and `8.8.8.8` <details><summary>Show example(s)</summary>{{< highlight yaml >}}
nameservers:
    - 8.8.8.8
    - 1.1.1.1
{{< /highlight >}}</details> | |
|`searchDomains` |[]string |Used to statically set arbitrary search domains. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
searchDomains:
    - example.org
    - example.com
{{< /highlight >}}</details> | |
|`extraHostEntries` |<a href="#Config.machine.network.extraHostEntries.">[]ExtraHost</a> |Allows for extra entries to be added to the `/etc/hosts` file <details><summary>Show example(s)</summary>{{< highlight yaml >}}
extraHostEntries:
    - ip: 192.168.1.100 # The IP of the host.
      # The host alias.
      aliases:
        - example
        - example.domain.tld
{{< /highlight >}}</details> | |
|`kubespan` |<a href="#Config.machine.network.kubespan">NetworkKubeSpan</a> |Configures KubeSpan feature. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
kubespan:
    enabled: true # Enable the KubeSpan feature.
{{< /highlight >}}</details> | |
|`disableSearchDomain` |bool |Disable generating a default search domain in /etc/resolv.conf<br>based on the machine hostname.<br>Defaults to `false`.  |`true`<br />`yes`<br />`false`<br />`no`<br /> |




#### interfaces[] {#Config.machine.network.interfaces.}

Device represents a network interface.



{{< highlight yaml >}}
machine:
    network:
        interfaces:
            - interface: enp0s1 # The interface name.
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
              #     hardwareAddr: '*:f0:ab' # Device hardware (MAC) address, supports matching by wildcard.
              #     driver: virtio_net # Kernel driver, supports matching by wildcard.
              # # select a device with bus prefix 00:*, a device with mac address matching `*:f0:ab` and `virtio` kernel driver.
              # deviceSelector:
              #     - busPath: 00:* # PCI, USB bus prefix, supports matching by wildcard.
              #     - hardwareAddr: '*:f0:ab' # Device hardware (MAC) address, supports matching by wildcard.
              #       driver: virtio_net # Kernel driver, supports matching by wildcard.

              # # Bond specific options.
              # bond:
              #     # The interfaces that make up the bond.
              #     interfaces:
              #         - enp2s0
              #         - enp2s1
              #     # Picks a network device using the selector.
              #     deviceSelectors:
              #         - busPath: 00:* # PCI, USB bus prefix, supports matching by wildcard.
              #         - hardwareAddr: '*:f0:ab' # Device hardware (MAC) address, supports matching by wildcard.
              #           driver: virtio_net # Kernel driver, supports matching by wildcard.
              #     mode: 802.3ad # A bond option.
              #     lacpRate: fast # A bond option.

              # # Bridge specific options.
              # bridge:
              #     # The interfaces that make up the bridge.
              #     interfaces:
              #         - enxda4042ca9a51
              #         - enxae2a6774c259
              #     # Enable STP on this bridge.
              #     stp:
              #         enabled: true # Whether Spanning Tree Protocol (STP) is enabled.

              # # Configure this device as a bridge port.
              # bridgePort:
              #     master: br0 # The name of the bridge master interface

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
              #           endpoint: 192.168.1.2:51822 # Specifies the endpoint of this peer entry.
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
|`interface` |string |The interface name.<br>Mutually exclusive with `deviceSelector`. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
interface: enp0s3
{{< /highlight >}}</details> | |
|`deviceSelector` |<a href="#Config.machine.network.interfaces..deviceSelector">NetworkDeviceSelector</a> |Picks a network device using the selector.<br>Mutually exclusive with `interface`.<br>Supports partial match using wildcard syntax. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
deviceSelector:
    busPath: 00:* # PCI, USB bus prefix, supports matching by wildcard.
{{< /highlight >}}{{< highlight yaml >}}
deviceSelector:
    hardwareAddr: '*:f0:ab' # Device hardware (MAC) address, supports matching by wildcard.
    driver: virtio_net # Kernel driver, supports matching by wildcard.
{{< /highlight >}}</details> | |
|`addresses` |[]string |Assigns static IP addresses to the interface.<br>An address can be specified either in proper CIDR notation or as a standalone address (netmask of all ones is assumed). <details><summary>Show example(s)</summary>{{< highlight yaml >}}
addresses:
    - 10.5.0.0/16
    - 192.168.3.7
{{< /highlight >}}</details> | |
|`routes` |<a href="#Config.machine.network.interfaces..routes.">[]Route</a> |A list of routes associated with the interface.<br>If used in combination with DHCP, these routes will be appended to routes returned by DHCP server. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
routes:
    - network: 0.0.0.0/0 # The route's network (destination).
      gateway: 10.5.0.1 # The route's gateway (if empty, creates link scope route).
    - network: 10.2.0.0/16 # The route's network (destination).
      gateway: 10.2.0.1 # The route's gateway (if empty, creates link scope route).
{{< /highlight >}}</details> | |
|`bond` |<a href="#Config.machine.network.interfaces..bond">Bond</a> |Bond specific options. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
bond:
    # The interfaces that make up the bond.
    interfaces:
        - enp2s0
        - enp2s1
    mode: 802.3ad # A bond option.
    lacpRate: fast # A bond option.

    # # Picks a network device using the selector.

    # # select a device with bus prefix 00:*, a device with mac address matching `*:f0:ab` and `virtio` kernel driver.
    # deviceSelectors:
    #     - busPath: 00:* # PCI, USB bus prefix, supports matching by wildcard.
    #     - hardwareAddr: '*:f0:ab' # Device hardware (MAC) address, supports matching by wildcard.
    #       driver: virtio_net # Kernel driver, supports matching by wildcard.
{{< /highlight >}}</details> | |
|`bridge` |<a href="#Config.machine.network.interfaces..bridge">Bridge</a> |Bridge specific options. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
bridge:
    # The interfaces that make up the bridge.
    interfaces:
        - enxda4042ca9a51
        - enxae2a6774c259
    # Enable STP on this bridge.
    stp:
        enabled: true # Whether Spanning Tree Protocol (STP) is enabled.
{{< /highlight >}}</details> | |
|`bridgePort` |<a href="#Config.machine.network.interfaces..bridgePort">BridgePort</a> |Configure this device as a bridge port.<br>This can be used to dynamically assign network interfaces to a bridge. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
bridgePort:
    master: br0 # The name of the bridge master interface
{{< /highlight >}}</details> | |
|`vlans` |<a href="#Config.machine.network.interfaces..vlans.">[]Vlan</a> |VLAN specific options.  | |
|`mtu` |int |The interface's MTU.<br>If used in combination with DHCP, this will override any MTU settings returned from DHCP server.  | |
|`dhcp` |bool |Indicates if DHCP should be used to configure the interface.<br>The following DHCP options are supported:<br><br>- `OptionClasslessStaticRoute`<br>- `OptionDomainNameServer`<br>- `OptionDNSDomainSearchList`<br>- `OptionHostName` <details><summary>Show example(s)</summary>{{< highlight yaml >}}
dhcp: true
{{< /highlight >}}</details> | |
|`ignore` |bool |Indicates if the interface should be ignored (skips configuration).  | |
|`dummy` |bool |Indicates if the interface is a dummy interface.<br>`dummy` is used to specify that this interface should be a virtual-only, dummy interface.  | |
|`dhcpOptions` |<a href="#Config.machine.network.interfaces..dhcpOptions">DHCPOptions</a> |DHCP specific options.<br>`dhcp` *must* be set to true for these to take effect. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
dhcpOptions:
    routeMetric: 1024 # The priority of all routes received via DHCP.
{{< /highlight >}}</details> | |
|`wireguard` |<a href="#Config.machine.network.interfaces..wireguard">DeviceWireguardConfig</a> |Wireguard specific configuration.<br>Includes things like private key, listen port, peers. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
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
          endpoint: 192.168.1.2:51822 # Specifies the endpoint of this peer entry.
          persistentKeepaliveInterval: 10s # Specifies the persistent keepalive interval for this peer.
          # AllowedIPs specifies a list of allowed IP addresses in CIDR notation for this peer.
          allowedIPs:
            - 192.168.1.0/24
{{< /highlight >}}</details> | |
|`vip` |<a href="#Config.machine.network.interfaces..vip">DeviceVIPConfig</a> |Virtual (shared) IP address configuration. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
vip:
    ip: 172.16.199.55 # Specifies the IP address to be used.
{{< /highlight >}}</details> | |




##### deviceSelector {#Config.machine.network.interfaces..deviceSelector}

NetworkDeviceSelector struct describes network device selector.



{{< highlight yaml >}}
machine:
    network:
        interfaces:
            - deviceSelector:
                busPath: 00:* # PCI, USB bus prefix, supports matching by wildcard.
{{< /highlight >}}

{{< highlight yaml >}}
machine:
    network:
        interfaces:
            - deviceSelector:
                hardwareAddr: '*:f0:ab' # Device hardware (MAC) address, supports matching by wildcard.
                driver: virtio_net # Kernel driver, supports matching by wildcard.
{{< /highlight >}}

{{< highlight yaml >}}
machine:
    network:
        interfaces:
            - deviceSelector:
                - busPath: 00:* # PCI, USB bus prefix, supports matching by wildcard.
                - hardwareAddr: '*:f0:ab' # Device hardware (MAC) address, supports matching by wildcard.
                  driver: virtio_net # Kernel driver, supports matching by wildcard.
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`busPath` |string |PCI, USB bus prefix, supports matching by wildcard.  | |
|`hardwareAddr` |string |Device hardware (MAC) address, supports matching by wildcard.  | |
|`permanentAddr` |string |Device permanent hardware address, supports matching by wildcard.<br>The permanent address doesn't change when the link is enslaved to a bond,<br>so it's recommended to use this field for bond members.  | |
|`pciID` |string |PCI ID (vendor ID, product ID), supports matching by wildcard.  | |
|`driver` |string |Kernel driver, supports matching by wildcard.  | |
|`physical` |bool |Select only physical devices.  | |






##### routes[] {#Config.machine.network.interfaces..routes.}

Route represents a network route.



{{< highlight yaml >}}
machine:
    network:
        interfaces:
            - routes:
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
|`mtu` |uint32 |The optional MTU for the route.  | |






##### bond {#Config.machine.network.interfaces..bond}

Bond contains the various options for configuring a bonded interface.



{{< highlight yaml >}}
machine:
    network:
        interfaces:
            - bond:
                # The interfaces that make up the bond.
                interfaces:
                    - enp2s0
                    - enp2s1
                mode: 802.3ad # A bond option.
                lacpRate: fast # A bond option.

                # # Picks a network device using the selector.

                # # select a device with bus prefix 00:*, a device with mac address matching `*:f0:ab` and `virtio` kernel driver.
                # deviceSelectors:
                #     - busPath: 00:* # PCI, USB bus prefix, supports matching by wildcard.
                #     - hardwareAddr: '*:f0:ab' # Device hardware (MAC) address, supports matching by wildcard.
                #       driver: virtio_net # Kernel driver, supports matching by wildcard.
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`interfaces` |[]string |The interfaces that make up the bond.  | |
|`deviceSelectors` |<a href="#Config.machine.network.interfaces..bond.deviceSelectors.">[]NetworkDeviceSelector</a> |Picks a network device using the selector.<br>Mutually exclusive with `interfaces`.<br>Supports partial match using wildcard syntax. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
deviceSelectors:
    - busPath: 00:* # PCI, USB bus prefix, supports matching by wildcard.
    - hardwareAddr: '*:f0:ab' # Device hardware (MAC) address, supports matching by wildcard.
      driver: virtio_net # Kernel driver, supports matching by wildcard.
{{< /highlight >}}</details> | |
|`arpIPTarget` |[]string |A bond option.<br>Please see the official kernel documentation.<br>Not supported at the moment.  | |
|`mode` |string |A bond option.<br>Please see the official kernel documentation.  | |
|`xmitHashPolicy` |string |A bond option.<br>Please see the official kernel documentation.  | |
|`lacpRate` |string |A bond option.<br>Please see the official kernel documentation.  | |
|`adActorSystem` |string |A bond option.<br>Please see the official kernel documentation.<br>Not supported at the moment.  | |
|`arpValidate` |string |A bond option.<br>Please see the official kernel documentation.  | |
|`arpAllTargets` |string |A bond option.<br>Please see the official kernel documentation.  | |
|`primary` |string |A bond option.<br>Please see the official kernel documentation.  | |
|`primaryReselect` |string |A bond option.<br>Please see the official kernel documentation.  | |
|`failOverMac` |string |A bond option.<br>Please see the official kernel documentation.  | |
|`adSelect` |string |A bond option.<br>Please see the official kernel documentation.  | |
|`miimon` |uint32 |A bond option.<br>Please see the official kernel documentation.  | |
|`updelay` |uint32 |A bond option.<br>Please see the official kernel documentation.  | |
|`downdelay` |uint32 |A bond option.<br>Please see the official kernel documentation.  | |
|`arpInterval` |uint32 |A bond option.<br>Please see the official kernel documentation.  | |
|`resendIgmp` |uint32 |A bond option.<br>Please see the official kernel documentation.  | |
|`minLinks` |uint32 |A bond option.<br>Please see the official kernel documentation.  | |
|`lpInterval` |uint32 |A bond option.<br>Please see the official kernel documentation.  | |
|`packetsPerSlave` |uint32 |A bond option.<br>Please see the official kernel documentation.  | |
|`numPeerNotif` |uint8 |A bond option.<br>Please see the official kernel documentation.  | |
|`tlbDynamicLb` |uint8 |A bond option.<br>Please see the official kernel documentation.  | |
|`allSlavesActive` |uint8 |A bond option.<br>Please see the official kernel documentation.  | |
|`useCarrier` |bool |A bond option.<br>Please see the official kernel documentation.  | |
|`adActorSysPrio` |uint16 |A bond option.<br>Please see the official kernel documentation.  | |
|`adUserPortKey` |uint16 |A bond option.<br>Please see the official kernel documentation.  | |
|`peerNotifyDelay` |uint32 |A bond option.<br>Please see the official kernel documentation.  | |




###### deviceSelectors[] {#Config.machine.network.interfaces..bond.deviceSelectors.}

NetworkDeviceSelector struct describes network device selector.



{{< highlight yaml >}}
machine:
    network:
        interfaces:
            - bond:
                deviceSelectors:
                    busPath: 00:* # PCI, USB bus prefix, supports matching by wildcard.
{{< /highlight >}}

{{< highlight yaml >}}
machine:
    network:
        interfaces:
            - bond:
                deviceSelectors:
                    hardwareAddr: '*:f0:ab' # Device hardware (MAC) address, supports matching by wildcard.
                    driver: virtio_net # Kernel driver, supports matching by wildcard.
{{< /highlight >}}

{{< highlight yaml >}}
machine:
    network:
        interfaces:
            - bond:
                deviceSelectors:
                    - busPath: 00:* # PCI, USB bus prefix, supports matching by wildcard.
                    - hardwareAddr: '*:f0:ab' # Device hardware (MAC) address, supports matching by wildcard.
                      driver: virtio_net # Kernel driver, supports matching by wildcard.
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`busPath` |string |PCI, USB bus prefix, supports matching by wildcard.  | |
|`hardwareAddr` |string |Device hardware (MAC) address, supports matching by wildcard.  | |
|`permanentAddr` |string |Device permanent hardware address, supports matching by wildcard.<br>The permanent address doesn't change when the link is enslaved to a bond,<br>so it's recommended to use this field for bond members.  | |
|`pciID` |string |PCI ID (vendor ID, product ID), supports matching by wildcard.  | |
|`driver` |string |Kernel driver, supports matching by wildcard.  | |
|`physical` |bool |Select only physical devices.  | |








##### bridge {#Config.machine.network.interfaces..bridge}

Bridge contains the various options for configuring a bridge interface.



{{< highlight yaml >}}
machine:
    network:
        interfaces:
            - bridge:
                # The interfaces that make up the bridge.
                interfaces:
                    - enxda4042ca9a51
                    - enxae2a6774c259
                # Enable STP on this bridge.
                stp:
                    enabled: true # Whether Spanning Tree Protocol (STP) is enabled.
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`interfaces` |[]string |The interfaces that make up the bridge.  | |
|`stp` |<a href="#Config.machine.network.interfaces..bridge.stp">STP</a> |Enable STP on this bridge.<br>Please see the official kernel documentation.  | |
|`vlan` |<a href="#Config.machine.network.interfaces..bridge.vlan">BridgeVLAN</a> |Enable VLAN-awareness on this bridge.<br>Please see the official kernel documentation.  | |




###### stp {#Config.machine.network.interfaces..bridge.stp}

STP contains the various options for configuring the STP properties of a bridge interface.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`enabled` |bool |Whether Spanning Tree Protocol (STP) is enabled.  | |






###### vlan {#Config.machine.network.interfaces..bridge.vlan}

BridgeVLAN contains the various options for configuring the VLAN properties of a bridge interface.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`vlanFiltering` |bool |Whether VLAN filtering is enabled.  | |








##### bridgePort {#Config.machine.network.interfaces..bridgePort}

BridgePort contains settings for assigning a link to a bridge interface.



{{< highlight yaml >}}
machine:
    network:
        interfaces:
            - bridgePort:
                master: br0 # The name of the bridge master interface
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`master` |string |The name of the bridge master interface  | |






##### vlans[] {#Config.machine.network.interfaces..vlans.}

Vlan represents vlan settings for a device.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`addresses` |[]string |The addresses in CIDR notation or as plain IPs to use.  | |
|`routes` |<a href="#Config.machine.network.interfaces..vlans..routes.">[]Route</a> |A list of routes associated with the VLAN.  | |
|`dhcp` |bool |Indicates if DHCP should be used.  | |
|`vlanId` |uint16 |The VLAN's ID.  | |
|`mtu` |uint32 |The VLAN's MTU.  | |
|`vip` |<a href="#Config.machine.network.interfaces..vlans..vip">DeviceVIPConfig</a> |The VLAN's virtual IP address configuration.  | |
|`dhcpOptions` |<a href="#Config.machine.network.interfaces..vlans..dhcpOptions">DHCPOptions</a> |DHCP specific options.<br>`dhcp` *must* be set to true for these to take effect.  | |




###### routes[] {#Config.machine.network.interfaces..vlans..routes.}

Route represents a network route.



{{< highlight yaml >}}
machine:
    network:
        interfaces:
            - vlans:
                - routes:
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
|`mtu` |uint32 |The optional MTU for the route.  | |






###### vip {#Config.machine.network.interfaces..vlans..vip}

DeviceVIPConfig contains settings for configuring a Virtual Shared IP on an interface.



{{< highlight yaml >}}
machine:
    network:
        interfaces:
            - vlans:
                - vip:
                    ip: 172.16.199.55 # Specifies the IP address to be used.
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`ip` |string |Specifies the IP address to be used.  | |
|`equinixMetal` |<a href="#Config.machine.network.interfaces..vlans..vip.equinixMetal">VIPEquinixMetalConfig</a> |Specifies the Equinix Metal API settings to assign VIP to the node.  | |
|`hcloud` |<a href="#Config.machine.network.interfaces..vlans..vip.hcloud">VIPHCloudConfig</a> |Specifies the Hetzner Cloud API settings to assign VIP to the node.  | |




###### equinixMetal {#Config.machine.network.interfaces..vlans..vip.equinixMetal}

VIPEquinixMetalConfig contains settings for Equinix Metal VIP management.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`apiToken` |string |Specifies the Equinix Metal API Token.  | |






###### hcloud {#Config.machine.network.interfaces..vlans..vip.hcloud}

VIPHCloudConfig contains settings for Hetzner Cloud VIP management.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`apiToken` |string |Specifies the Hetzner Cloud API Token.  | |








###### dhcpOptions {#Config.machine.network.interfaces..vlans..dhcpOptions}

DHCPOptions contains options for configuring the DHCP settings for a given interface.



{{< highlight yaml >}}
machine:
    network:
        interfaces:
            - vlans:
                - dhcpOptions:
                    routeMetric: 1024 # The priority of all routes received via DHCP.
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`routeMetric` |uint32 |The priority of all routes received via DHCP.  | |
|`ipv4` |bool |Enables DHCPv4 protocol for the interface (default is enabled).  | |
|`ipv6` |bool |Enables DHCPv6 protocol for the interface (default is disabled).  | |
|`duidv6` |string |Set client DUID (hex string).  | |








##### dhcpOptions {#Config.machine.network.interfaces..dhcpOptions}

DHCPOptions contains options for configuring the DHCP settings for a given interface.



{{< highlight yaml >}}
machine:
    network:
        interfaces:
            - dhcpOptions:
                routeMetric: 1024 # The priority of all routes received via DHCP.
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`routeMetric` |uint32 |The priority of all routes received via DHCP.  | |
|`ipv4` |bool |Enables DHCPv4 protocol for the interface (default is enabled).  | |
|`ipv6` |bool |Enables DHCPv6 protocol for the interface (default is disabled).  | |
|`duidv6` |string |Set client DUID (hex string).  | |






##### wireguard {#Config.machine.network.interfaces..wireguard}

DeviceWireguardConfig contains settings for configuring Wireguard network interface.



{{< highlight yaml >}}
machine:
    network:
        interfaces:
            - wireguard:
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
machine:
    network:
        interfaces:
            - wireguard:
                privateKey: ABCDEF... # Specifies a private key configuration (base64 encoded).
                # Specifies a list of peer configurations to apply to a device.
                peers:
                    - publicKey: ABCDEF... # Specifies the public key of this peer.
                      endpoint: 192.168.1.2:51822 # Specifies the endpoint of this peer entry.
                      persistentKeepaliveInterval: 10s # Specifies the persistent keepalive interval for this peer.
                      # AllowedIPs specifies a list of allowed IP addresses in CIDR notation for this peer.
                      allowedIPs:
                        - 192.168.1.0/24
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`privateKey` |string |Specifies a private key configuration (base64 encoded).<br>Can be generated by `wg genkey`.  | |
|`listenPort` |int |Specifies a device's listening port.  | |
|`firewallMark` |int |Specifies a device's firewall mark.  | |
|`peers` |<a href="#Config.machine.network.interfaces..wireguard.peers.">[]DeviceWireguardPeer</a> |Specifies a list of peer configurations to apply to a device.  | |




###### peers[] {#Config.machine.network.interfaces..wireguard.peers.}

DeviceWireguardPeer a WireGuard device peer configuration.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`publicKey` |string |Specifies the public key of this peer.<br>Can be extracted from private key by running `wg pubkey < private.key > public.key && cat public.key`.  | |
|`endpoint` |string |Specifies the endpoint of this peer entry.  | |
|`persistentKeepaliveInterval` |Duration |Specifies the persistent keepalive interval for this peer.<br>Field format accepts any Go time.Duration format ('1h' for one hour, '10m' for ten minutes).  | |
|`allowedIPs` |[]string |AllowedIPs specifies a list of allowed IP addresses in CIDR notation for this peer.  | |








##### vip {#Config.machine.network.interfaces..vip}

DeviceVIPConfig contains settings for configuring a Virtual Shared IP on an interface.



{{< highlight yaml >}}
machine:
    network:
        interfaces:
            - vip:
                ip: 172.16.199.55 # Specifies the IP address to be used.
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`ip` |string |Specifies the IP address to be used.  | |
|`equinixMetal` |<a href="#Config.machine.network.interfaces..vip.equinixMetal">VIPEquinixMetalConfig</a> |Specifies the Equinix Metal API settings to assign VIP to the node.  | |
|`hcloud` |<a href="#Config.machine.network.interfaces..vip.hcloud">VIPHCloudConfig</a> |Specifies the Hetzner Cloud API settings to assign VIP to the node.  | |




###### equinixMetal {#Config.machine.network.interfaces..vip.equinixMetal}

VIPEquinixMetalConfig contains settings for Equinix Metal VIP management.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`apiToken` |string |Specifies the Equinix Metal API Token.  | |






###### hcloud {#Config.machine.network.interfaces..vip.hcloud}

VIPHCloudConfig contains settings for Hetzner Cloud VIP management.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`apiToken` |string |Specifies the Hetzner Cloud API Token.  | |










#### extraHostEntries[] {#Config.machine.network.extraHostEntries.}

ExtraHost represents a host entry in /etc/hosts.



{{< highlight yaml >}}
machine:
    network:
        extraHostEntries:
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






#### kubespan {#Config.machine.network.kubespan}

NetworkKubeSpan struct describes KubeSpan configuration.



{{< highlight yaml >}}
machine:
    network:
        kubespan:
            enabled: true # Enable the KubeSpan feature.
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`enabled` |bool |Enable the KubeSpan feature.<br>Cluster discovery should be enabled with .cluster.discovery.enabled for KubeSpan to be enabled.  | |
|`advertiseKubernetesNetworks` |bool |Control whether Kubernetes pod CIDRs are announced over KubeSpan from the node.<br>If disabled, CNI handles encapsulating pod-to-pod traffic into some node-to-node tunnel,<br>and KubeSpan handles the node-to-node traffic.<br>If enabled, KubeSpan will take over pod-to-pod traffic and send it over KubeSpan directly.<br>When enabled, KubeSpan should have a way to detect complete pod CIDRs of the node which<br>is not always the case with CNIs not relying on Kubernetes for IPAM.  | |
|`allowDownPeerBypass` |bool |Skip sending traffic via KubeSpan if the peer connection state is not up.<br>This provides configurable choice between connectivity and security: either traffic is always<br>forced to go via KubeSpan (even if Wireguard peer connection is not up), or traffic can go directly<br>to the peer if Wireguard connection can't be established.  | |
|`harvestExtraEndpoints` |bool |KubeSpan can collect and publish extra endpoints for each member of the cluster<br>based on Wireguard endpoint information for each peer.<br>This feature is disabled by default, don't enable it<br>with high number of peers (>50) in the KubeSpan network (performance issues).  | |
|`mtu` |uint32 |KubeSpan link MTU size.<br>Default value is 1420.  | |
|`filters` |<a href="#Config.machine.network.kubespan.filters">KubeSpanFilters</a> |KubeSpan advanced filtering of network addresses .<br><br>Settings in this section are optional, and settings apply only to the node.  | |




##### filters {#Config.machine.network.kubespan.filters}

KubeSpanFilters struct describes KubeSpan advanced network addresses filtering.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`endpoints` |[]string |Filter node addresses which will be advertised as KubeSpan endpoints for peer-to-peer Wireguard connections.<br><br>By default, all addresses are advertised, and KubeSpan cycles through all endpoints until it finds one that works.<br><br>Default value: no filtering. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
endpoints:
    - 0.0.0.0/0
    - '!192.168.0.0/16'
    - ::/0
{{< /highlight >}}</details> | |










### install {#Config.machine.install}

InstallConfig represents the installation options for preparing a node.



{{< highlight yaml >}}
machine:
    install:
        disk: /dev/sda # The disk used for installations.
        # Allows for supplying extra kernel args via the bootloader.
        extraKernelArgs:
            - console=ttyS1
            - panic=10
        image: ghcr.io/siderolabs/installer:latest # Allows for supplying the image used to perform the installation.
        wipe: false # Indicates if the installation disk should be wiped at installation time.

        # # Look up disk using disk attributes like model, size, serial and others.
        # diskSelector:
        #     size: 4GB # Disk size.
        #     model: WDC* # Disk model `/sys/block/<dev>/device/model`.
        #     busPath: /pci0000:00/0000:00:17.0/ata1/host0/target0:0:0/0:0:0:0 # Disk bus path.
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`disk` |string |The disk used for installations. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
disk: /dev/sda
{{< /highlight >}}{{< highlight yaml >}}
disk: /dev/nvme0
{{< /highlight >}}</details> | |
|`diskSelector` |<a href="#Config.machine.install.diskSelector">InstallDiskSelector</a> |Look up disk using disk attributes like model, size, serial and others.<br>Always has priority over `disk`. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
diskSelector:
    size: '>= 1TB' # Disk size.
    model: WDC* # Disk model `/sys/block/<dev>/device/model`.

    # # Disk bus path.
    # busPath: /pci0000:00/0000:00:17.0/ata1/host0/target0:0:0/0:0:0:0
    # busPath: /pci0000:00/*
{{< /highlight >}}</details> | |
|`extraKernelArgs` |[]string |Allows for supplying extra kernel args via the bootloader.<br>Existing kernel args can be removed by prefixing the argument with a `-`.<br>For example `-console` removes all `console=<value>` arguments, whereas `-console=tty0` removes the `console=tty0` default argument.<br>If Talos is using systemd-boot as a bootloader (default for UEFI) this setting will be ignored. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
extraKernelArgs:
    - talos.platform=metal
    - reboot=k
{{< /highlight >}}</details> | |
|`image` |string |Allows for supplying the image used to perform the installation.<br>Image reference for each Talos release can be found on<br>[GitHub releases page](https://github.com/siderolabs/talos/releases). <details><summary>Show example(s)</summary>{{< highlight yaml >}}
image: ghcr.io/siderolabs/installer:latest
{{< /highlight >}}</details> | |
|`wipe` |bool |Indicates if the installation disk should be wiped at installation time.<br>Defaults to `true`.  |`true`<br />`yes`<br />`false`<br />`no`<br /> |
|`legacyBIOSSupport` |bool |Indicates if MBR partition should be marked as bootable (active).<br>Should be enabled only for the systems with legacy BIOS that doesn't support GPT partitioning scheme.  | |




#### diskSelector {#Config.machine.install.diskSelector}

InstallDiskSelector represents a disk query parameters for the install disk lookup.



{{< highlight yaml >}}
machine:
    install:
        diskSelector:
            size: '>= 1TB' # Disk size.
            model: WDC* # Disk model `/sys/block/<dev>/device/model`.

            # # Disk bus path.
            # busPath: /pci0000:00/0000:00:17.0/ata1/host0/target0:0:0/0:0:0:0
            # busPath: /pci0000:00/*
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








### files[] {#Config.machine.files.}

MachineFile represents a file to write to disk.



{{< highlight yaml >}}
machine:
    files:
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






### time {#Config.machine.time}

TimeConfig represents the options for configuring time on a machine.



{{< highlight yaml >}}
machine:
    time:
        disabled: false # Indicates if the time service is disabled for the machine.
        # description: |
        servers:
            - time.cloudflare.com
        bootTimeout: 2m0s # Specifies the timeout when the node time is considered to be in sync unlocking the boot sequence.
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`disabled` |bool |Indicates if the time service is disabled for the machine.<br>Defaults to `false`.  | |
|`servers` |[]string |description: |<br>    Specifies time (NTP) servers to use for setting the system time.<br>    Defaults to `time.cloudflare.com`.<br><br>   Talos can also sync to the PTP time source (e.g provided by the hypervisor),<br>    provide the path to the PTP device as "/dev/ptp0" or "/dev/ptp_kvm".<br>  | |
|`bootTimeout` |Duration |Specifies the timeout when the node time is considered to be in sync unlocking the boot sequence.<br>NTP sync will be still running in the background.<br>Defaults to "infinity" (waiting forever for time sync)  | |






### registries {#Config.machine.registries}

RegistriesConfig represents the image pull options.



{{< highlight yaml >}}
machine:
    registries:
        # Specifies mirror configuration for each registry host namespace.
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
|`mirrors` |<a href="#Config.machine.registries.mirrors.-">map[string]RegistryMirrorConfig</a> |Specifies mirror configuration for each registry host namespace.<br>This setting allows to configure local pull-through caching registires,<br>air-gapped installations, etc.<br><br>For example, when pulling an image with the reference `example.com:123/image:v1`,<br>the `example.com:123` key will be used to lookup the mirror configuration.<br><br>Optionally the `*` key can be used to configure a fallback mirror.<br><br>Registry name is the first segment of image identifier, with 'docker.io'<br>being default one. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
mirrors:
    ghcr.io:
        # List of endpoints (URLs) for registry mirrors to use.
        endpoints:
            - https://registry.insecure
            - https://ghcr.io/v2/
{{< /highlight >}}</details> | |
|`config` |<a href="#Config.machine.registries.config.-">map[string]RegistryConfig</a> |Specifies TLS & auth configuration for HTTPS image registries.<br>Mutual TLS can be enabled with 'clientIdentity' option.<br><br>The full hostname and port (if not using a default port 443)<br>should be used as the key.<br>The fallback key `*` can't be used for TLS configuration.<br><br>TLS configuration can be skipped if registry has trusted<br>server certificate. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
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




#### mirrors.* {#Config.machine.registries.mirrors.-}

RegistryMirrorConfig represents mirror configuration for a registry.



{{< highlight yaml >}}
machine:
    registries:
        mirrors:
            ghcr.io:
                # List of endpoints (URLs) for registry mirrors to use.
                endpoints:
                    - https://registry.insecure
                    - https://ghcr.io/v2/
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`endpoints` |[]string |List of endpoints (URLs) for registry mirrors to use.<br>Endpoint configures HTTP/HTTPS access mode, host name,<br>port and path (if path is not set, it defaults to `/v2`).  | |
|`overridePath` |bool |Use the exact path specified for the endpoint (don't append /v2/).<br>This setting is often required for setting up multiple mirrors<br>on a single instance of a registry.  | |
|`skipFallback` |bool |Skip fallback to the upstream endpoint, for example the mirror configuration<br>for `docker.io` will not fallback to `registry-1.docker.io`.  | |






#### config.* {#Config.machine.registries.config.-}

RegistryConfig specifies auth & TLS config per registry.



{{< highlight yaml >}}
machine:
    registries:
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
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`tls` |<a href="#Config.machine.registries.config.-.tls">RegistryTLSConfig</a> |The TLS configuration for the registry. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
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
|`auth` |<a href="#Config.machine.registries.config.-.auth">RegistryAuthConfig</a> |The auth configuration for this registry.<br>Note: changes to the registry auth will not be picked up by the CRI containerd plugin without a reboot. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
auth:
    username: username # Optional registry authentication.
    password: password # Optional registry authentication.
{{< /highlight >}}</details> | |




##### tls {#Config.machine.registries.config.-.tls}

RegistryTLSConfig specifies TLS config for HTTPS registries.



{{< highlight yaml >}}
machine:
    registries:
        config:
            example.com:
                tls:
                    # Enable mutual TLS authentication with the registry.
                    clientIdentity:
                        crt: LS0tIEVYQU1QTEUgQ0VSVElGSUNBVEUgLS0t
                        key: LS0tIEVYQU1QTEUgS0VZIC0tLQ==
{{< /highlight >}}

{{< highlight yaml >}}
machine:
    registries:
        config:
            example.com:
                tls:
                    insecureSkipVerify: true # Skip TLS server certificate verification (not recommended).

                    # # Enable mutual TLS authentication with the registry.
                    # clientIdentity:
                    #     crt: LS0tIEVYQU1QTEUgQ0VSVElGSUNBVEUgLS0t
                    #     key: LS0tIEVYQU1QTEUgS0VZIC0tLQ==
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`clientIdentity` |PEMEncodedCertificateAndKey |Enable mutual TLS authentication with the registry.<br>Client certificate and key should be base64-encoded. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
clientIdentity:
    crt: LS0tIEVYQU1QTEUgQ0VSVElGSUNBVEUgLS0t
    key: LS0tIEVYQU1QTEUgS0VZIC0tLQ==
{{< /highlight >}}</details> | |
|`ca` |Base64Bytes |CA registry certificate to add the list of trusted certificates.<br>Certificate should be base64-encoded.  | |
|`insecureSkipVerify` |bool |Skip TLS server certificate verification (not recommended).  | |






##### auth {#Config.machine.registries.config.-.auth}

RegistryAuthConfig specifies authentication configuration for a registry.



{{< highlight yaml >}}
machine:
    registries:
        config:
            example.com:
                auth:
                    username: username # Optional registry authentication.
                    password: password # Optional registry authentication.
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`username` |string |Optional registry authentication.<br>The meaning of each field is the same with the corresponding field in [`.docker/config.json`](https://docs.docker.com/engine/api/v1.41/#section/Authentication).  | |
|`password` |string |Optional registry authentication.<br>The meaning of each field is the same with the corresponding field in [`.docker/config.json`](https://docs.docker.com/engine/api/v1.41/#section/Authentication).  | |
|`auth` |string |Optional registry authentication.<br>The meaning of each field is the same with the corresponding field in [`.docker/config.json`](https://docs.docker.com/engine/api/v1.41/#section/Authentication).  | |
|`identityToken` |string |Optional registry authentication.<br>The meaning of each field is the same with the corresponding field in [`.docker/config.json`](https://docs.docker.com/engine/api/v1.41/#section/Authentication).  | |










### systemDiskEncryption {#Config.machine.systemDiskEncryption}

SystemDiskEncryptionConfig specifies system disk partitions encryption settings.



{{< highlight yaml >}}
machine:
    systemDiskEncryption:
        # Ephemeral partition encryption.
        ephemeral:
            provider: luks2 # Encryption provider to use for the encryption.
            # Defines the encryption keys generation and storage method.
            keys:
                - # Deterministically generated key from the node UUID and PartitionLabel.
                  nodeID: {}
                  slot: 0 # Key slot number for LUKS2 encryption.

                  # # KMS managed encryption key.
                  # kms:
                  #     endpoint: https://192.168.88.21:4443 # KMS endpoint to Seal/Unseal the key.

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
|`state` |<a href="#Config.machine.systemDiskEncryption.state">EncryptionConfig</a> |State partition encryption.  | |
|`ephemeral` |<a href="#Config.machine.systemDiskEncryption.ephemeral">EncryptionConfig</a> |Ephemeral partition encryption.  | |




#### state {#Config.machine.systemDiskEncryption.state}

EncryptionConfig represents partition encryption settings.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`provider` |string |Encryption provider to use for the encryption. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
provider: luks2
{{< /highlight >}}</details> | |
|`keys` |<a href="#Config.machine.systemDiskEncryption.state.keys.">[]EncryptionKey</a> |Defines the encryption keys generation and storage method.  | |
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




##### keys[] {#Config.machine.systemDiskEncryption.state.keys.}

EncryptionKey represents configuration for disk encryption key.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`static` |<a href="#Config.machine.systemDiskEncryption.state.keys..static">EncryptionKeyStatic</a> |Key which value is stored in the configuration file.  | |
|`nodeID` |<a href="#Config.machine.systemDiskEncryption.state.keys..nodeID">EncryptionKeyNodeID</a> |Deterministically generated key from the node UUID and PartitionLabel.  | |
|`kms` |<a href="#Config.machine.systemDiskEncryption.state.keys..kms">EncryptionKeyKMS</a> |KMS managed encryption key. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
kms:
    endpoint: https://192.168.88.21:4443 # KMS endpoint to Seal/Unseal the key.
{{< /highlight >}}</details> | |
|`slot` |int |Key slot number for LUKS2 encryption.  | |
|`tpm` |<a href="#Config.machine.systemDiskEncryption.state.keys..tpm">EncryptionKeyTPM</a> |Enable TPM based disk encryption.  | |




###### static {#Config.machine.systemDiskEncryption.state.keys..static}

EncryptionKeyStatic represents throw away key type.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`passphrase` |string |Defines the static passphrase value.  | |






###### nodeID {#Config.machine.systemDiskEncryption.state.keys..nodeID}

EncryptionKeyNodeID represents deterministically generated key from the node UUID and PartitionLabel.









###### kms {#Config.machine.systemDiskEncryption.state.keys..kms}

EncryptionKeyKMS represents a key that is generated and then sealed/unsealed by the KMS server.



{{< highlight yaml >}}
machine:
    systemDiskEncryption:
        state:
            keys:
                - kms:
                    endpoint: https://192.168.88.21:4443 # KMS endpoint to Seal/Unseal the key.
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`endpoint` |string |KMS endpoint to Seal/Unseal the key.  | |






###### tpm {#Config.machine.systemDiskEncryption.state.keys..tpm}

EncryptionKeyTPM represents a key that is generated and then sealed/unsealed by the TPM.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`checkSecurebootStatusOnEnroll` |bool |Check that Secureboot is enabled in the EFI firmware.<br>If Secureboot is not enabled, the enrollment of the key will fail. As the TPM key is anyways bound to the value of PCR 7, changing Secureboot status or configuration after the initial enrollment will make the key unusable.  | |










#### ephemeral {#Config.machine.systemDiskEncryption.ephemeral}

EncryptionConfig represents partition encryption settings.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`provider` |string |Encryption provider to use for the encryption. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
provider: luks2
{{< /highlight >}}</details> | |
|`keys` |<a href="#Config.machine.systemDiskEncryption.ephemeral.keys.">[]EncryptionKey</a> |Defines the encryption keys generation and storage method.  | |
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




##### keys[] {#Config.machine.systemDiskEncryption.ephemeral.keys.}

EncryptionKey represents configuration for disk encryption key.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`static` |<a href="#Config.machine.systemDiskEncryption.ephemeral.keys..static">EncryptionKeyStatic</a> |Key which value is stored in the configuration file.  | |
|`nodeID` |<a href="#Config.machine.systemDiskEncryption.ephemeral.keys..nodeID">EncryptionKeyNodeID</a> |Deterministically generated key from the node UUID and PartitionLabel.  | |
|`kms` |<a href="#Config.machine.systemDiskEncryption.ephemeral.keys..kms">EncryptionKeyKMS</a> |KMS managed encryption key. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
kms:
    endpoint: https://192.168.88.21:4443 # KMS endpoint to Seal/Unseal the key.
{{< /highlight >}}</details> | |
|`slot` |int |Key slot number for LUKS2 encryption.  | |
|`tpm` |<a href="#Config.machine.systemDiskEncryption.ephemeral.keys..tpm">EncryptionKeyTPM</a> |Enable TPM based disk encryption.  | |




###### static {#Config.machine.systemDiskEncryption.ephemeral.keys..static}

EncryptionKeyStatic represents throw away key type.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`passphrase` |string |Defines the static passphrase value.  | |






###### nodeID {#Config.machine.systemDiskEncryption.ephemeral.keys..nodeID}

EncryptionKeyNodeID represents deterministically generated key from the node UUID and PartitionLabel.









###### kms {#Config.machine.systemDiskEncryption.ephemeral.keys..kms}

EncryptionKeyKMS represents a key that is generated and then sealed/unsealed by the KMS server.



{{< highlight yaml >}}
machine:
    systemDiskEncryption:
        ephemeral:
            keys:
                - kms:
                    endpoint: https://192.168.88.21:4443 # KMS endpoint to Seal/Unseal the key.
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`endpoint` |string |KMS endpoint to Seal/Unseal the key.  | |






###### tpm {#Config.machine.systemDiskEncryption.ephemeral.keys..tpm}

EncryptionKeyTPM represents a key that is generated and then sealed/unsealed by the TPM.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`checkSecurebootStatusOnEnroll` |bool |Check that Secureboot is enabled in the EFI firmware.<br>If Secureboot is not enabled, the enrollment of the key will fail. As the TPM key is anyways bound to the value of PCR 7, changing Secureboot status or configuration after the initial enrollment will make the key unusable.  | |












### features {#Config.machine.features}

FeaturesConfig describes individual Talos features that can be switched on or off.



{{< highlight yaml >}}
machine:
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
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`rbac` |bool |Enable role-based access control (RBAC).  | |
|`stableHostname` |bool |Enable stable default hostname.  | |
|`kubernetesTalosAPIAccess` |<a href="#Config.machine.features.kubernetesTalosAPIAccess">KubernetesTalosAPIAccessConfig</a> |Configure Talos API access from Kubernetes pods.<br><br>This feature is disabled if the feature config is not specified. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
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
|`diskQuotaSupport` |bool |Enable XFS project quota support for EPHEMERAL partition and user disks.<br>Also enables kubelet tracking of ephemeral disk usage in the kubelet via quota.  | |
|`kubePrism` |<a href="#Config.machine.features.kubePrism">KubePrism</a> |KubePrism - local proxy/load balancer on defined port that will distribute<br>requests to all API servers in the cluster.  | |
|`hostDNS` |<a href="#Config.machine.features.hostDNS">HostDNSConfig</a> |Configures host DNS caching resolver.  | |
|`imageCache` |<a href="#Config.machine.features.imageCache">ImageCacheConfig</a> |Enable Image Cache feature.  | |
|`nodeAddressSortAlgorithm` |string |Select the node address sort algorithm.<br>The 'v1' algorithm sorts addresses by the address itself.<br>The 'v2' algorithm prefers more specific prefixes.<br>If unset, defaults to 'v1'.  | |




#### kubernetesTalosAPIAccess {#Config.machine.features.kubernetesTalosAPIAccess}

KubernetesTalosAPIAccessConfig describes the configuration for the Talos API access from Kubernetes pods.



{{< highlight yaml >}}
machine:
    features:
        kubernetesTalosAPIAccess:
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
|`allowedRoles` |[]string |The list of Talos API roles which can be granted for access from Kubernetes pods.<br><br>Empty list means that no roles can be granted, so access is blocked.  | |
|`allowedKubernetesNamespaces` |[]string |The list of Kubernetes namespaces Talos API access is available from.  | |






#### kubePrism {#Config.machine.features.kubePrism}

KubePrism describes the configuration for the KubePrism load balancer.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`enabled` |bool |Enable KubePrism support - will start local load balancing proxy.  | |
|`port` |int |KubePrism port.  | |






#### hostDNS {#Config.machine.features.hostDNS}

HostDNSConfig describes the configuration for the host DNS resolver.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`enabled` |bool |Enable host DNS caching resolver.  | |
|`forwardKubeDNSToHost` |bool |Use the host DNS resolver as upstream for Kubernetes CoreDNS pods.<br><br>When enabled, CoreDNS pods use host DNS server as the upstream DNS (instead of<br>using configured upstream DNS resolvers directly).  | |
|`resolveMemberNames` |bool |Resolve member hostnames using the host DNS resolver.<br><br>When enabled, cluster member hostnames and node names are resolved using the host DNS resolver.<br>This requires service discovery to be enabled.  | |






#### imageCache {#Config.machine.features.imageCache}

ImageCacheConfig describes the configuration for the Image Cache feature.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`localEnabled` |bool |Enable local image cache.  | |








### udev {#Config.machine.udev}

UdevConfig describes how the udev system should be configured.



{{< highlight yaml >}}
machine:
    udev:
        # List of udev rules to apply to the udev system
        rules:
            - SUBSYSTEM=="drm", KERNEL=="renderD*", GROUP="44", MODE="0660"
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`rules` |[]string |List of udev rules to apply to the udev system  | |






### logging {#Config.machine.logging}

LoggingConfig struct configures Talos logging.



{{< highlight yaml >}}
machine:
    logging:
        # Logging destination.
        destinations:
            - endpoint: tcp://1.2.3.4:12345 # Where to send logs. Supported protocols are "tcp" and "udp".
              format: json_lines # Logs format.
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`destinations` |<a href="#Config.machine.logging.destinations.">[]LoggingDestination</a> |Logging destination.  | |




#### destinations[] {#Config.machine.logging.destinations.}

LoggingDestination struct configures Talos logging destination.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`endpoint` |<a href="#Config.machine.logging.destinations..endpoint">Endpoint</a> |Where to send logs. Supported protocols are "tcp" and "udp". <details><summary>Show example(s)</summary>{{< highlight yaml >}}
endpoint: udp://127.0.0.1:12345
{{< /highlight >}}{{< highlight yaml >}}
endpoint: tcp://1.2.3.4:12345
{{< /highlight >}}</details> | |
|`format` |string |Logs format.  |`json_lines`<br /> |
|`extraTags` |map[string]string |Extra tags (key-value) pairs to attach to every log message sent.  | |




##### endpoint {#Config.machine.logging.destinations..endpoint}

Endpoint represents the endpoint URL parsed out of the machine config.



{{< highlight yaml >}}
machine:
    logging:
        destinations:
            - endpoint: https://1.2.3.4:6443
{{< /highlight >}}

{{< highlight yaml >}}
machine:
    logging:
        destinations:
            - endpoint: https://cluster1.internal:6443
{{< /highlight >}}

{{< highlight yaml >}}
machine:
    logging:
        destinations:
            - endpoint: udp://127.0.0.1:12345
{{< /highlight >}}

{{< highlight yaml >}}
machine:
    logging:
        destinations:
            - endpoint: tcp://1.2.3.4:12345
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|










### kernel {#Config.machine.kernel}

KernelConfig struct configures Talos Linux kernel.



{{< highlight yaml >}}
machine:
    kernel:
        # Kernel modules to load.
        modules:
            - name: brtfs # Module name.
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`modules` |<a href="#Config.machine.kernel.modules.">[]KernelModuleConfig</a> |Kernel modules to load.  | |




#### modules[] {#Config.machine.kernel.modules.}

KernelModuleConfig struct configures Linux kernel modules to load.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`name` |string |Module name.  | |
|`parameters` |[]string |Module parameters, changes applied after reboot.  | |








### seccompProfiles[] {#Config.machine.seccompProfiles.}

MachineSeccompProfile defines seccomp profiles for the machine.



{{< highlight yaml >}}
machine:
    seccompProfiles:
        - name: audit.json # The `name` field is used to provide the file name of the seccomp profile.
          # The `value` field is used to provide the seccomp profile.
          value:
            defaultAction: SCMP_ACT_LOG
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`name` |string |The `name` field is used to provide the file name of the seccomp profile.  | |
|`value` |Unstructured |The `value` field is used to provide the seccomp profile.  | |








## cluster {#Config.cluster}

ClusterConfig represents the cluster-wide config values.



{{< highlight yaml >}}
cluster:
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
|`secret` |string |Shared secret of cluster (base64 encoded random 32 bytes).<br>This secret is shared among cluster members but should never be sent over the network.  | |
|`controlPlane` |<a href="#Config.cluster.controlPlane">ControlPlaneConfig</a> |Provides control plane specific configuration options. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
controlPlane:
    endpoint: https://1.2.3.4 # Endpoint is the canonical controlplane endpoint, which can be an IP address or a DNS hostname.
    localAPIServerPort: 443 # The port that the API server listens on internally.
{{< /highlight >}}</details> | |
|`clusterName` |string |Configures the cluster's name.  | |
|`network` |<a href="#Config.cluster.network">ClusterNetworkConfig</a> |Provides cluster specific network configuration options. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
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
|`aescbcEncryptionSecret` |string |A key used for the [encryption of secret data at rest](https://kubernetes.io/docs/tasks/administer-cluster/encrypt-data/).<br>Enables encryption with AESCBC. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
aescbcEncryptionSecret: z01mye6j16bspJYtTB/5SFX8j7Ph4JXxM2Xuu4vsBPM=
{{< /highlight >}}</details> | |
|`secretboxEncryptionSecret` |string |A key used for the [encryption of secret data at rest](https://kubernetes.io/docs/tasks/administer-cluster/encrypt-data/).<br>Enables encryption with secretbox.<br>Secretbox has precedence over AESCBC. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
secretboxEncryptionSecret: z01mye6j16bspJYtTB/5SFX8j7Ph4JXxM2Xuu4vsBPM=
{{< /highlight >}}</details> | |
|`ca` |PEMEncodedCertificateAndKey |The base64 encoded root certificate authority used by Kubernetes. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
ca:
    crt: LS0tIEVYQU1QTEUgQ0VSVElGSUNBVEUgLS0t
    key: LS0tIEVYQU1QTEUgS0VZIC0tLQ==
{{< /highlight >}}</details> | |
|`acceptedCAs` |[]PEMEncodedCertificate |The list of base64 encoded accepted certificate authorities used by Kubernetes.  | |
|`aggregatorCA` |PEMEncodedCertificateAndKey |The base64 encoded aggregator certificate authority used by Kubernetes for front-proxy certificate generation.<br><br>This CA can be self-signed. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
aggregatorCA:
    crt: LS0tIEVYQU1QTEUgQ0VSVElGSUNBVEUgLS0t
    key: LS0tIEVYQU1QTEUgS0VZIC0tLQ==
{{< /highlight >}}</details> | |
|`serviceAccount` |PEMEncodedKey |The base64 encoded private key for service account token generation. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
serviceAccount:
    key: LS0tIEVYQU1QTEUgS0VZIC0tLQ==
{{< /highlight >}}</details> | |
|`apiServer` |<a href="#Config.cluster.apiServer">APIServerConfig</a> |API server specific configuration options. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
apiServer:
    image: registry.k8s.io/kube-apiserver:v1.33.1 # The container image used in the API server manifest.
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

    # # Configure the API server authorization config. Node and RBAC authorizers are always added irrespective of the configuration.
    # authorizationConfig:
    #     - type: Webhook # Type is the name of the authorizer. Allowed values are `Node`, `RBAC`, and `Webhook`.
    #       name: webhook # Name is used to describe the authorizer.
    #       # webhook is the configuration for the webhook authorizer.
    #       webhook:
    #         connectionInfo:
    #             type: InClusterConfig
    #         failurePolicy: Deny
    #         matchConditionSubjectAccessReviewVersion: v1
    #         matchConditions:
    #             - expression: has(request.resourceAttributes)
    #             - expression: '!(\''system:serviceaccounts:kube-system\'' in request.groups)'
    #         subjectAccessReviewVersion: v1
    #         timeout: 3s
    #     - type: Webhook # Type is the name of the authorizer. Allowed values are `Node`, `RBAC`, and `Webhook`.
    #       name: in-cluster-authorizer # Name is used to describe the authorizer.
    #       # webhook is the configuration for the webhook authorizer.
    #       webhook:
    #         connectionInfo:
    #             type: InClusterConfig
    #         failurePolicy: NoOpinion
    #         matchConditionSubjectAccessReviewVersion: v1
    #         subjectAccessReviewVersion: v1
    #         timeout: 3s
{{< /highlight >}}</details> | |
|`controllerManager` |<a href="#Config.cluster.controllerManager">ControllerManagerConfig</a> |Controller manager server specific configuration options. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
controllerManager:
    image: registry.k8s.io/kube-controller-manager:v1.33.1 # The container image used in the controller manager manifest.
    # Extra arguments to supply to the controller manager.
    extraArgs:
        feature-gates: ServerSideApply=true
{{< /highlight >}}</details> | |
|`proxy` |<a href="#Config.cluster.proxy">ProxyConfig</a> |Kube-proxy server-specific configuration options <details><summary>Show example(s)</summary>{{< highlight yaml >}}
proxy:
    image: registry.k8s.io/kube-proxy:v1.33.1 # The container image used in the kube-proxy manifest.
    mode: ipvs # proxy mode of kube-proxy.
    # Extra arguments to supply to kube-proxy.
    extraArgs:
        proxy-mode: iptables

    # # Disable kube-proxy deployment on cluster bootstrap.
    # disabled: false
{{< /highlight >}}</details> | |
|`scheduler` |<a href="#Config.cluster.scheduler">SchedulerConfig</a> |Scheduler server specific configuration options. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
scheduler:
    image: registry.k8s.io/kube-scheduler:v1.33.1 # The container image used in the scheduler manifest.
    # Extra arguments to supply to the scheduler.
    extraArgs:
        feature-gates: AllBeta=true
{{< /highlight >}}</details> | |
|`discovery` |<a href="#Config.cluster.discovery">ClusterDiscoveryConfig</a> |Configures cluster member discovery. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
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
|`etcd` |<a href="#Config.cluster.etcd">EtcdConfig</a> |Etcd specific configuration options. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
etcd:
    image: gcr.io/etcd-development/etcd:v3.5.21 # The container image used to create the etcd service.
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
|`coreDNS` |<a href="#Config.cluster.coreDNS">CoreDNS</a> |Core DNS specific configuration options. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
coreDNS:
    image: registry.k8s.io/coredns/coredns:v1.12.1 # The `image` field is an override to the default coredns image.
{{< /highlight >}}</details> | |
|`externalCloudProvider` |<a href="#Config.cluster.externalCloudProvider">ExternalCloudProviderConfig</a> |External cloud provider configuration. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
externalCloudProvider:
    enabled: true # Enable external cloud provider.
    # A list of urls that point to additional manifests for an external cloud provider.
    manifests:
        - https://raw.githubusercontent.com/kubernetes/cloud-provider-aws/v1.20.0-alpha.0/manifests/rbac.yaml
        - https://raw.githubusercontent.com/kubernetes/cloud-provider-aws/v1.20.0-alpha.0/manifests/aws-cloud-controller-manager-daemonset.yaml
{{< /highlight >}}</details> | |
|`extraManifests` |[]string |A list of urls that point to additional manifests.<br>These will get automatically deployed as part of the bootstrap. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
extraManifests:
    - https://www.example.com/manifest1.yaml
    - https://www.example.com/manifest2.yaml
{{< /highlight >}}</details> | |
|`extraManifestHeaders` |map[string]string |A map of key value pairs that will be added while fetching the extraManifests. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
extraManifestHeaders:
    Token: "1234567"
    X-ExtraInfo: info
{{< /highlight >}}</details> | |
|`inlineManifests` |<a href="#Config.cluster.inlineManifests.">[]ClusterInlineManifest</a> |A list of inline Kubernetes manifests.<br>These will get automatically deployed as part of the bootstrap. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
inlineManifests:
    - name: namespace-ci # Name of the manifest.
      contents: |- # Manifest contents as a string.
        apiVersion: v1
        kind: Namespace
        metadata:
        	name: ci
{{< /highlight >}}</details> | |
|`adminKubeconfig` |<a href="#Config.cluster.adminKubeconfig">AdminKubeconfigConfig</a> |Settings for admin kubeconfig generation.<br>Certificate lifetime can be configured. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
adminKubeconfig:
    certLifetime: 1h0m0s # Admin kubeconfig certificate lifetime (default is 1 year).
{{< /highlight >}}</details> | |
|`allowSchedulingOnControlPlanes` |bool |Allows running workload on control-plane nodes. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
allowSchedulingOnControlPlanes: true
{{< /highlight >}}</details> |`true`<br />`yes`<br />`false`<br />`no`<br /> |




### controlPlane {#Config.cluster.controlPlane}

ControlPlaneConfig represents the control plane configuration options.



{{< highlight yaml >}}
cluster:
    controlPlane:
        endpoint: https://1.2.3.4 # Endpoint is the canonical controlplane endpoint, which can be an IP address or a DNS hostname.
        localAPIServerPort: 443 # The port that the API server listens on internally.
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`endpoint` |<a href="#Config.cluster.controlPlane.endpoint">Endpoint</a> |Endpoint is the canonical controlplane endpoint, which can be an IP address or a DNS hostname.<br>It is single-valued, and may optionally include a port number. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
endpoint: https://1.2.3.4:6443
{{< /highlight >}}{{< highlight yaml >}}
endpoint: https://cluster1.internal:6443
{{< /highlight >}}</details> | |
|`localAPIServerPort` |int |The port that the API server listens on internally.<br>This may be different than the port portion listed in the endpoint field above.<br>The default is `6443`.  | |




#### endpoint {#Config.cluster.controlPlane.endpoint}

Endpoint represents the endpoint URL parsed out of the machine config.



{{< highlight yaml >}}
cluster:
    controlPlane:
        endpoint: https://1.2.3.4:6443
{{< /highlight >}}

{{< highlight yaml >}}
cluster:
    controlPlane:
        endpoint: https://cluster1.internal:6443
{{< /highlight >}}

{{< highlight yaml >}}
cluster:
    controlPlane:
        endpoint: udp://127.0.0.1:12345
{{< /highlight >}}

{{< highlight yaml >}}
cluster:
    controlPlane:
        endpoint: tcp://1.2.3.4:12345
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|








### network {#Config.cluster.network}

ClusterNetworkConfig represents kube networking configuration options.



{{< highlight yaml >}}
cluster:
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
|`cni` |<a href="#Config.cluster.network.cni">CNIConfig</a> |The CNI used.<br>Composed of "name" and "urls".<br>The "name" key supports the following options: "flannel", "custom", and "none".<br>"flannel" uses Talos-managed Flannel CNI, and that's the default option.<br>"custom" uses custom manifests that should be provided in "urls".<br>"none" indicates that Talos will not manage any CNI installation. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
cni:
    name: custom # Name of CNI to use.
    # URLs containing manifests to apply for the CNI.
    urls:
        - https://docs.projectcalico.org/archive/v3.20/manifests/canal.yaml
{{< /highlight >}}</details> | |
|`dnsDomain` |string |The domain used by Kubernetes DNS.<br>The default is `cluster.local` <details><summary>Show example(s)</summary>{{< highlight yaml >}}
dnsDomain: cluster.local
{{< /highlight >}}</details> | |
|`podSubnets` |[]string |The pod subnet CIDR. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
podSubnets:
    - 10.244.0.0/16
{{< /highlight >}}</details> | |
|`serviceSubnets` |[]string |The service subnet CIDR. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
serviceSubnets:
    - 10.96.0.0/12
{{< /highlight >}}</details> | |




#### cni {#Config.cluster.network.cni}

CNIConfig represents the CNI configuration options.



{{< highlight yaml >}}
cluster:
    network:
        cni:
            name: custom # Name of CNI to use.
            # URLs containing manifests to apply for the CNI.
            urls:
                - https://docs.projectcalico.org/archive/v3.20/manifests/canal.yaml
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`name` |string |Name of CNI to use.  |`flannel`<br />`custom`<br />`none`<br /> |
|`urls` |[]string |URLs containing manifests to apply for the CNI.<br>Should be present for "custom", must be empty for "flannel" and "none".  | |
|`flannel` |<a href="#Config.cluster.network.cni.flannel">FlannelCNIConfig</a> |description: |<br>Flannel configuration options.<br>  | |




##### flannel {#Config.cluster.network.cni.flannel}

FlannelCNIConfig represents the Flannel CNI configuration options.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`extraArgs` |[]string |Extra arguments for 'flanneld'. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
extraArgs:
    - --iface-can-reach=192.168.1.1
{{< /highlight >}}</details> | |










### apiServer {#Config.cluster.apiServer}

APIServerConfig represents the kube apiserver configuration options.



{{< highlight yaml >}}
cluster:
    apiServer:
        image: registry.k8s.io/kube-apiserver:v1.33.1 # The container image used in the API server manifest.
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

        # # Configure the API server authorization config. Node and RBAC authorizers are always added irrespective of the configuration.
        # authorizationConfig:
        #     - type: Webhook # Type is the name of the authorizer. Allowed values are `Node`, `RBAC`, and `Webhook`.
        #       name: webhook # Name is used to describe the authorizer.
        #       # webhook is the configuration for the webhook authorizer.
        #       webhook:
        #         connectionInfo:
        #             type: InClusterConfig
        #         failurePolicy: Deny
        #         matchConditionSubjectAccessReviewVersion: v1
        #         matchConditions:
        #             - expression: has(request.resourceAttributes)
        #             - expression: '!(\''system:serviceaccounts:kube-system\'' in request.groups)'
        #         subjectAccessReviewVersion: v1
        #         timeout: 3s
        #     - type: Webhook # Type is the name of the authorizer. Allowed values are `Node`, `RBAC`, and `Webhook`.
        #       name: in-cluster-authorizer # Name is used to describe the authorizer.
        #       # webhook is the configuration for the webhook authorizer.
        #       webhook:
        #         connectionInfo:
        #             type: InClusterConfig
        #         failurePolicy: NoOpinion
        #         matchConditionSubjectAccessReviewVersion: v1
        #         subjectAccessReviewVersion: v1
        #         timeout: 3s
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`image` |string |The container image used in the API server manifest. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
image: registry.k8s.io/kube-apiserver:v1.33.1
{{< /highlight >}}</details> | |
|`extraArgs` |map[string]string |Extra arguments to supply to the API server.  | |
|`extraVolumes` |<a href="#Config.cluster.apiServer.extraVolumes.">[]VolumeMountConfig</a> |Extra volumes to mount to the API server static pod.  | |
|`env` |Env |The `env` field allows for the addition of environment variables for the control plane component.  | |
|`certSANs` |[]string |Extra certificate subject alternative names for the API server's certificate.  | |
|`disablePodSecurityPolicy` |bool |Disable PodSecurityPolicy in the API server and default manifests.  | |
|`admissionControl` |<a href="#Config.cluster.apiServer.admissionControl.">[]AdmissionPluginConfig</a> |Configure the API server admission plugins. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
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
|`resources` |<a href="#Config.cluster.apiServer.resources">ResourcesConfig</a> |Configure the API server resources.  | |
|`authorizationConfig` |<a href="#Config.cluster.apiServer.authorizationConfig.">[]AuthorizationConfigAuthorizerConfig</a> |Configure the API server authorization config. Node and RBAC authorizers are always added irrespective of the configuration. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
authorizationConfig:
    - type: Webhook # Type is the name of the authorizer. Allowed values are `Node`, `RBAC`, and `Webhook`.
      name: webhook # Name is used to describe the authorizer.
      # webhook is the configuration for the webhook authorizer.
      webhook:
        connectionInfo:
            type: InClusterConfig
        failurePolicy: Deny
        matchConditionSubjectAccessReviewVersion: v1
        matchConditions:
            - expression: has(request.resourceAttributes)
            - expression: '!(\''system:serviceaccounts:kube-system\'' in request.groups)'
        subjectAccessReviewVersion: v1
        timeout: 3s
    - type: Webhook # Type is the name of the authorizer. Allowed values are `Node`, `RBAC`, and `Webhook`.
      name: in-cluster-authorizer # Name is used to describe the authorizer.
      # webhook is the configuration for the webhook authorizer.
      webhook:
        connectionInfo:
            type: InClusterConfig
        failurePolicy: NoOpinion
        matchConditionSubjectAccessReviewVersion: v1
        subjectAccessReviewVersion: v1
        timeout: 3s
{{< /highlight >}}</details> | |




#### extraVolumes[] {#Config.cluster.apiServer.extraVolumes.}

VolumeMountConfig struct describes extra volume mount for the static pods.




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






#### admissionControl[] {#Config.cluster.apiServer.admissionControl.}

AdmissionPluginConfig represents the API server admission plugin configuration.



{{< highlight yaml >}}
cluster:
    apiServer:
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
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`name` |string |Name is the name of the admission controller.<br>It must match the registered admission plugin name.  | |
|`configuration` |Unstructured |Configuration is an embedded configuration object to be used as the plugin's<br>configuration.  | |






#### resources {#Config.cluster.apiServer.resources}

ResourcesConfig represents the pod resources.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`requests` |Unstructured |Requests configures the reserved cpu/memory resources. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
requests:
    cpu: 1
    memory: 1Gi
{{< /highlight >}}</details> | |
|`limits` |Unstructured |Limits configures the maximum cpu/memory resources a container can use. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
limits:
    cpu: 2
    memory: 2500Mi
{{< /highlight >}}</details> | |






#### authorizationConfig[] {#Config.cluster.apiServer.authorizationConfig.}

AuthorizationConfigAuthorizerConfig represents the API server authorization config authorizer configuration.



{{< highlight yaml >}}
cluster:
    apiServer:
        authorizationConfig:
            - type: Webhook # Type is the name of the authorizer. Allowed values are `Node`, `RBAC`, and `Webhook`.
              name: webhook # Name is used to describe the authorizer.
              # webhook is the configuration for the webhook authorizer.
              webhook:
                connectionInfo:
                    type: InClusterConfig
                failurePolicy: Deny
                matchConditionSubjectAccessReviewVersion: v1
                matchConditions:
                    - expression: has(request.resourceAttributes)
                    - expression: '!(\''system:serviceaccounts:kube-system\'' in request.groups)'
                subjectAccessReviewVersion: v1
                timeout: 3s
            - type: Webhook # Type is the name of the authorizer. Allowed values are `Node`, `RBAC`, and `Webhook`.
              name: in-cluster-authorizer # Name is used to describe the authorizer.
              # webhook is the configuration for the webhook authorizer.
              webhook:
                connectionInfo:
                    type: InClusterConfig
                failurePolicy: NoOpinion
                matchConditionSubjectAccessReviewVersion: v1
                subjectAccessReviewVersion: v1
                timeout: 3s
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`type` |string |Type is the name of the authorizer. Allowed values are `Node`, `RBAC`, and `Webhook`.  | |
|`name` |string |Name is used to describe the authorizer.  | |
|`webhook` |Unstructured |webhook is the configuration for the webhook authorizer.  | |








### controllerManager {#Config.cluster.controllerManager}

ControllerManagerConfig represents the kube controller manager configuration options.



{{< highlight yaml >}}
cluster:
    controllerManager:
        image: registry.k8s.io/kube-controller-manager:v1.33.1 # The container image used in the controller manager manifest.
        # Extra arguments to supply to the controller manager.
        extraArgs:
            feature-gates: ServerSideApply=true
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`image` |string |The container image used in the controller manager manifest. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
image: registry.k8s.io/kube-controller-manager:v1.33.1
{{< /highlight >}}</details> | |
|`extraArgs` |map[string]string |Extra arguments to supply to the controller manager.  | |
|`extraVolumes` |<a href="#Config.cluster.controllerManager.extraVolumes.">[]VolumeMountConfig</a> |Extra volumes to mount to the controller manager static pod.  | |
|`env` |Env |The `env` field allows for the addition of environment variables for the control plane component.  | |
|`resources` |<a href="#Config.cluster.controllerManager.resources">ResourcesConfig</a> |Configure the controller manager resources.  | |




#### extraVolumes[] {#Config.cluster.controllerManager.extraVolumes.}

VolumeMountConfig struct describes extra volume mount for the static pods.




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






#### resources {#Config.cluster.controllerManager.resources}

ResourcesConfig represents the pod resources.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`requests` |Unstructured |Requests configures the reserved cpu/memory resources. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
requests:
    cpu: 1
    memory: 1Gi
{{< /highlight >}}</details> | |
|`limits` |Unstructured |Limits configures the maximum cpu/memory resources a container can use. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
limits:
    cpu: 2
    memory: 2500Mi
{{< /highlight >}}</details> | |








### proxy {#Config.cluster.proxy}

ProxyConfig represents the kube proxy configuration options.



{{< highlight yaml >}}
cluster:
    proxy:
        image: registry.k8s.io/kube-proxy:v1.33.1 # The container image used in the kube-proxy manifest.
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
image: registry.k8s.io/kube-proxy:v1.33.1
{{< /highlight >}}</details> | |
|`mode` |string |proxy mode of kube-proxy.<br>The default is 'iptables'.  | |
|`extraArgs` |map[string]string |Extra arguments to supply to kube-proxy.  | |






### scheduler {#Config.cluster.scheduler}

SchedulerConfig represents the kube scheduler configuration options.



{{< highlight yaml >}}
cluster:
    scheduler:
        image: registry.k8s.io/kube-scheduler:v1.33.1 # The container image used in the scheduler manifest.
        # Extra arguments to supply to the scheduler.
        extraArgs:
            feature-gates: AllBeta=true
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`image` |string |The container image used in the scheduler manifest. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
image: registry.k8s.io/kube-scheduler:v1.33.1
{{< /highlight >}}</details> | |
|`extraArgs` |map[string]string |Extra arguments to supply to the scheduler.  | |
|`extraVolumes` |<a href="#Config.cluster.scheduler.extraVolumes.">[]VolumeMountConfig</a> |Extra volumes to mount to the scheduler static pod.  | |
|`env` |Env |The `env` field allows for the addition of environment variables for the control plane component.  | |
|`resources` |<a href="#Config.cluster.scheduler.resources">ResourcesConfig</a> |Configure the scheduler resources.  | |
|`config` |Unstructured |Specify custom kube-scheduler configuration.  | |




#### extraVolumes[] {#Config.cluster.scheduler.extraVolumes.}

VolumeMountConfig struct describes extra volume mount for the static pods.




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






#### resources {#Config.cluster.scheduler.resources}

ResourcesConfig represents the pod resources.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`requests` |Unstructured |Requests configures the reserved cpu/memory resources. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
requests:
    cpu: 1
    memory: 1Gi
{{< /highlight >}}</details> | |
|`limits` |Unstructured |Limits configures the maximum cpu/memory resources a container can use. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
limits:
    cpu: 2
    memory: 2500Mi
{{< /highlight >}}</details> | |








### discovery {#Config.cluster.discovery}

ClusterDiscoveryConfig struct configures cluster membership discovery.



{{< highlight yaml >}}
cluster:
    discovery:
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
|`enabled` |bool |Enable the cluster membership discovery feature.<br>Cluster discovery is based on individual registries which are configured under the registries field.  | |
|`registries` |<a href="#Config.cluster.discovery.registries">DiscoveryRegistriesConfig</a> |Configure registries used for cluster member discovery.  | |




#### registries {#Config.cluster.discovery.registries}

DiscoveryRegistriesConfig struct configures cluster membership discovery.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`kubernetes` |<a href="#Config.cluster.discovery.registries.kubernetes">RegistryKubernetesConfig</a> |Kubernetes registry uses Kubernetes API server to discover cluster members and stores additional information<br>as annotations on the Node resources.<br><br>This feature is deprecated as it is not compatible with Kubernetes 1.32+.<br>See https://github.com/siderolabs/talos/issues/9980 for more information.  | |
|`service` |<a href="#Config.cluster.discovery.registries.service">RegistryServiceConfig</a> |Service registry is using an external service to push and pull information about cluster members.  | |




##### kubernetes {#Config.cluster.discovery.registries.kubernetes}

RegistryKubernetesConfig struct configures Kubernetes discovery registry.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`disabled` |bool |Disable Kubernetes discovery registry.  | |






##### service {#Config.cluster.discovery.registries.service}

RegistryServiceConfig struct configures Kubernetes discovery registry.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`disabled` |bool |Disable external service discovery registry.  | |
|`endpoint` |string |External service endpoint. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
endpoint: https://discovery.talos.dev/
{{< /highlight >}}</details> | |










### etcd {#Config.cluster.etcd}

EtcdConfig represents the etcd configuration options.



{{< highlight yaml >}}
cluster:
    etcd:
        image: gcr.io/etcd-development/etcd:v3.5.21 # The container image used to create the etcd service.
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
image: gcr.io/etcd-development/etcd:v3.5.21
{{< /highlight >}}</details> | |
|`ca` |PEMEncodedCertificateAndKey |The `ca` is the root certificate authority of the PKI.<br>It is composed of a base64 encoded `crt` and `key`. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
ca:
    crt: LS0tIEVYQU1QTEUgQ0VSVElGSUNBVEUgLS0t
    key: LS0tIEVYQU1QTEUgS0VZIC0tLQ==
{{< /highlight >}}</details> | |
|`extraArgs` |map[string]string |Extra arguments to supply to etcd.<br>Note that the following args are not allowed:<br><br>- `name`<br>- `data-dir`<br>- `initial-cluster-state`<br>- `listen-peer-urls`<br>- `listen-client-urls`<br>- `cert-file`<br>- `key-file`<br>- `trusted-ca-file`<br>- `peer-client-cert-auth`<br>- `peer-cert-file`<br>- `peer-trusted-ca-file`<br>- `peer-key-file`  | |
|`advertisedSubnets` |[]string |The `advertisedSubnets` field configures the networks to pick etcd advertised IP from.<br><br>IPs can be excluded from the list by using negative match with `!`, e.g `!10.0.0.0/8`.<br>Negative subnet matches should be specified last to filter out IPs picked by positive matches.<br>If not specified, advertised IP is selected as the first routable address of the node. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
advertisedSubnets:
    - 10.0.0.0/8
{{< /highlight >}}</details> | |
|`listenSubnets` |[]string |The `listenSubnets` field configures the networks for the etcd to listen for peer and client connections.<br><br>If `listenSubnets` is not set, but `advertisedSubnets` is set, `listenSubnets` defaults to<br>`advertisedSubnets`.<br><br>If neither `advertisedSubnets` nor `listenSubnets` is set, `listenSubnets` defaults to listen on all addresses.<br><br>IPs can be excluded from the list by using negative match with `!`, e.g `!10.0.0.0/8`.<br>Negative subnet matches should be specified last to filter out IPs picked by positive matches.<br>If not specified, advertised IP is selected as the first routable address of the node.  | |






### coreDNS {#Config.cluster.coreDNS}

CoreDNS represents the CoreDNS config values.



{{< highlight yaml >}}
cluster:
    coreDNS:
        image: registry.k8s.io/coredns/coredns:v1.12.1 # The `image` field is an override to the default coredns image.
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`disabled` |bool |Disable coredns deployment on cluster bootstrap.  | |
|`image` |string |The `image` field is an override to the default coredns image.  | |






### externalCloudProvider {#Config.cluster.externalCloudProvider}

ExternalCloudProviderConfig contains external cloud provider configuration.



{{< highlight yaml >}}
cluster:
    externalCloudProvider:
        enabled: true # Enable external cloud provider.
        # A list of urls that point to additional manifests for an external cloud provider.
        manifests:
            - https://raw.githubusercontent.com/kubernetes/cloud-provider-aws/v1.20.0-alpha.0/manifests/rbac.yaml
            - https://raw.githubusercontent.com/kubernetes/cloud-provider-aws/v1.20.0-alpha.0/manifests/aws-cloud-controller-manager-daemonset.yaml
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`enabled` |bool |Enable external cloud provider.  |`true`<br />`yes`<br />`false`<br />`no`<br /> |
|`manifests` |[]string |A list of urls that point to additional manifests for an external cloud provider.<br>These will get automatically deployed as part of the bootstrap. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
manifests:
    - https://raw.githubusercontent.com/kubernetes/cloud-provider-aws/v1.20.0-alpha.0/manifests/rbac.yaml
    - https://raw.githubusercontent.com/kubernetes/cloud-provider-aws/v1.20.0-alpha.0/manifests/aws-cloud-controller-manager-daemonset.yaml
{{< /highlight >}}</details> | |






### inlineManifests[] {#Config.cluster.inlineManifests.}

ClusterInlineManifest struct describes inline bootstrap manifests for the user.



{{< highlight yaml >}}
cluster:
    inlineManifests:
        - name: namespace-ci # Name of the manifest.
          contents: |- # Manifest contents as a string.
            apiVersion: v1
            kind: Namespace
            metadata:
            	name: ci
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`name` |string |Name of the manifest.<br>Name should be unique. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
name: csi
{{< /highlight >}}</details> | |
|`contents` |string |Manifest contents as a string. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
contents: /etc/kubernetes/auth
{{< /highlight >}}</details> | |






### adminKubeconfig {#Config.cluster.adminKubeconfig}

AdminKubeconfigConfig contains admin kubeconfig settings.



{{< highlight yaml >}}
cluster:
    adminKubeconfig:
        certLifetime: 1h0m0s # Admin kubeconfig certificate lifetime (default is 1 year).
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`certLifetime` |Duration |Admin kubeconfig certificate lifetime (default is 1 year).<br>Field format accepts any Go time.Duration format ('1h' for one hour, '10m' for ten minutes).  | |










