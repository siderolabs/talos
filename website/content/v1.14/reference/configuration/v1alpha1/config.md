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
    install:
        disk: /dev/sda
        image: factory.talos.dev/metal-installer/376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba:latest
        wipe: false
        grubUseUKICmdline: true
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`type` |string |Defines the role of the machine within the cluster.<br><br>**Control Plane**<br><br>Control Plane node type designates the node as a control plane member.<br>This means it will host etcd along with the Kubernetes controlplane components such as API Server, Controller Manager, Scheduler.<br><br>**Worker**<br><br>Worker node type designates the node as a worker node.<br>This means it will be an available compute node for scheduling workloads.<br><br>This node type was previously known as "join"; that value is still supported but deprecated.  |`controlplane`<br />`worker`<br /> |
|`token` |string |The `token` is used by a machine to join the PKI of the cluster.<br>Using this token, a machine will create a certificate signing request (CSR), and request a certificate that will be used as its' identity. <details><summary>Show example(s)</summary>example token:{{< highlight yaml >}}
token: 328hom.uqjzh6jnn2eie9oi
{{< /highlight >}}</details> | |
|`ca` |PEMEncodedCertificateAndKey |The root certificate authority of the PKI.<br>It is composed of a base64 encoded `crt` and `key`. <details><summary>Show example(s)</summary>machine CA example:{{< highlight yaml >}}
ca:
    crt: LS0tIEVYQU1QTEUgQ0VSVElGSUNBVEUgLS0t
    key: LS0tIEVYQU1QTEUgS0VZIC0tLQ==
{{< /highlight >}}</details> | |
|`acceptedCAs` |[]PEMEncodedCertificate |The certificates issued by certificate authorities are accepted in addition to issuing 'ca'.<br>It is composed of a base64 encoded `crt``.  | |
|`certSANs` |[]string |Extra certificate subject alternative names for the machine's certificate.<br>By default, all non-loopback interface IPs are automatically added to the certificate's SANs. <details><summary>Show example(s)</summary>Uncomment this to enable SANs.:{{< highlight yaml >}}
certSANs:
    - 10.0.0.10
    - 172.16.0.10
    - 192.168.0.10
{{< /highlight >}}</details> | |
|`kubelet` |<a href="#Config.machine.kubelet">KubeletConfig</a> |Used to provide additional options to the kubelet. <details><summary>Show example(s)</summary>Kubelet definition example.:{{< highlight yaml >}}
kubelet:
    image: ghcr.io/siderolabs/kubelet:v1.37.0-alpha.3 # The `image` field is an optional reference to an alternative kubelet image.
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
|`pods` |[]Unstructured |Used to provide static pod definitions to be run by the kubelet directly bypassing the kube-apiserver.<br><br>Static pods can be used to run components which should be started before the Kubernetes control plane is up.<br>Talos doesn't validate the pod definition.<br>Updates to this field can be applied without a reboot.<br><br>See https://kubernetes.io/docs/tasks/configure-pod-container/static-pod/. <details><summary>Show example(s)</summary>nginx static pod.:{{< highlight yaml >}}
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
|`files` |<a href="#Config.machine.files.">[]MachineFile</a> |Allows the addition of user specified files.<br>The value of `op` can be `create`, `overwrite`, or `append`.<br>In the case of `create`, `path` must not exist.<br>In the case of `overwrite`, and `append`, `path` must be a valid file.<br>If an `op` value of `append` is used, the existing file will be appended.<br>Note that the file contents are not required to be base64 encoded. <details><summary>Show example(s)</summary>MachineFiles usage example.:{{< highlight yaml >}}
files:
    - content: '...' # The contents of the file.
      permissions: 0o666 # The file's permissions in octal.
      path: /tmp/file.txt # The path of the file.
      op: append # The operation to use
{{< /highlight >}}</details> | |
|`features` |<a href="#Config.machine.features">FeaturesConfig</a> |Features describe individual Talos features that can be switched on or off. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
features:
    diskQuotaSupport: true # Enable XFS project quota support for EPHEMERAL partition and user disks.

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
|`logging` |<a href="#Config.machine.logging">LoggingConfig</a> |Configures the logging system. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
logging:
    # Logging destination.
    destinations:
        - endpoint: tcp://1.2.3.4:12345 # Where to send logs. Supported protocols are "tcp" and "udp".
          format: json_lines # Logs format.
{{< /highlight >}}{{< highlight yaml >}}
logging:
    # Logging destination.
    destinations:
        - endpoint: udp://127.0.0.1:12345 # Where to send logs. Supported protocols are "tcp" and "udp".
          format: json_lines # Logs format.
          # Extra tags (key-value) pairs to attach to every log message sent.
          extraTags:
            machine: worker-1
{{< /highlight >}}</details> | |
|`seccompProfiles` |<a href="#Config.machine.seccompProfiles.">[]MachineSeccompProfile</a> |Configures the seccomp profiles for the machine. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
seccompProfiles:
    - name: audit.json # The `name` field is used to provide the file name of the seccomp profile.
      # The `value` field is used to provide the seccomp profile.
      value:
        defaultAction: SCMP_ACT_LOG
{{< /highlight >}}</details> | |
|`baseRuntimeSpecOverrides` |Unstructured |Override (patch) settings in the default OCI runtime spec for CRI containers.<br><br>It can be used to set some default container settings which are not configurable in Kubernetes,<br>for example default ulimits.<br>Note: this change applies to all newly created containers, and it requires a reboot to take effect. <details><summary>Show example(s)</summary>override default open file limit:{{< highlight yaml >}}
baseRuntimeSpecOverrides:
    process:
        rlimits:
            - hard: 1024
              soft: 1024
              type: RLIMIT_NOFILE
{{< /highlight >}}</details> | |
|`nodeLabels` |map[string]string |Configures the node labels for the machine.<br><br>Note: In the default Kubernetes configuration, worker nodes are restricted to set<br>labels with some prefixes (see [NodeRestriction](https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/#noderestriction) admission plugin). <details><summary>Show example(s)</summary>node labels example.:{{< highlight yaml >}}
nodeLabels:
    exampleLabel: exampleLabelValue
{{< /highlight >}}</details> | |
|`nodeAnnotations` |map[string]string |Configures the node annotations for the machine. <details><summary>Show example(s)</summary>node annotations example.:{{< highlight yaml >}}
nodeAnnotations:
    customer.io/rack: r13a25
{{< /highlight >}}</details> | |
|`nodeTaints` |map[string]string |Configures the node taints for the machine. Effect is optional.<br><br>Note: In the default Kubernetes configuration, worker nodes are not allowed to<br>modify the taints (see [NodeRestriction](https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/#noderestriction) admission plugin). <details><summary>Show example(s)</summary>node taints example.:{{< highlight yaml >}}
nodeTaints:
    exampleTaint: exampleTaintValue:NoSchedule
{{< /highlight >}}</details> | |




### kubelet {#Config.machine.kubelet}

KubeletConfig represents the kubelet config values.



{{< highlight yaml >}}
machine:
    kubelet:
        image: ghcr.io/siderolabs/kubelet:v1.37.0-alpha.3 # The `image` field is an optional reference to an alternative kubelet image.
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
image: ghcr.io/siderolabs/kubelet:v1.37.0-alpha.3
{{< /highlight >}}</details> | |
|`clusterDNS` |[]string |The `ClusterDNS` field is an optional reference to an alternative kubelet clusterDNS ip list. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
clusterDNS:
    - 10.96.0.10
    - 169.254.2.53
{{< /highlight >}}</details> | |
|`extraArgs` |Args |The `extraArgs` field is used to provide additional flags to the kubelet. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
extraArgs:
    key: value
{{< /highlight >}}{{< highlight yaml >}}
extraArgs:
    key:
        - value1
        - value2
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






### features {#Config.machine.features}

FeaturesConfig describes individual Talos features that can be switched on or off.



{{< highlight yaml >}}
machine:
    features:
        diskQuotaSupport: true # Enable XFS project quota support for EPHEMERAL partition and user disks.

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
|`diskQuotaSupport` |bool |Enable XFS project quota support for EPHEMERAL partition and user disks.<br>Also enables kubelet tracking of ephemeral disk usage in the kubelet via quota.  | |
|`kubePrism` |<a href="#Config.machine.features.kubePrism">KubePrism</a> |KubePrism - local proxy/load balancer on defined port that will distribute<br>requests to all API servers in the cluster.  | |
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

{{< highlight yaml >}}
machine:
    logging:
        # Logging destination.
        destinations:
            - endpoint: udp://127.0.0.1:12345 # Where to send logs. Supported protocols are "tcp" and "udp".
              format: json_lines # Logs format.
              # Extra tags (key-value) pairs to attach to every log message sent.
              extraTags:
                machine: worker-1
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`destinations` |<a href="#Config.machine.logging.destinations.">[]LoggingDestination</a> |Logging destination.  | |




#### destinations[] {#Config.machine.logging.destinations.}

LoggingDestination struct configures Talos logging destination.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`endpoint` |<a href="#Config.machine.logging.destinations..endpoint">Endpoint</a> |Where to send logs. Supported protocols are "tcp" and "udp".  | |
|`format` |string |Logs format.  |`json_lines`<br /> |
|`extraTags` |map[string]string |Extra tags (key-value) pairs to attach to every log message sent.  | |




##### endpoint {#Config.machine.logging.destinations..endpoint}

Endpoint represents the endpoint URL parsed out of the machine config.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|










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
    clusterName: talos.local
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`controlPlane` |<a href="#Config.cluster.controlPlane">ControlPlaneConfig</a> |Provides control plane specific configuration options. <details><summary>Show example(s)</summary>Setting controlplane endpoint address to 1.2.3.4 and port to 443 example.:{{< highlight yaml >}}
controlPlane:
    endpoint: https://1.2.3.4 # Endpoint is the canonical controlplane endpoint, which can be an IP address or a DNS hostname.
{{< /highlight >}}</details> | |
|`clusterName` |string |Configures the cluster's name.  | |
|`token` |string |The [bootstrap token](https://kubernetes.io/docs/reference/access-authn-authz/bootstrap-tokens/) used to join the cluster. <details><summary>Show example(s)</summary>Bootstrap token example (do not use in production!).:{{< highlight yaml >}}
token: wlzjyw.bei2zfylhs2by0wd
{{< /highlight >}}</details> | |
|`etcd` |<a href="#Config.cluster.etcd">EtcdConfig</a> |Etcd specific configuration options. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
etcd:
    image: registry.k8s.io/etcd:v3.7.0 # The container image used to create the etcd service.
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
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`endpoint` |<a href="#Config.cluster.controlPlane.endpoint">Endpoint</a> |Endpoint is the canonical controlplane endpoint, which can be an IP address or a DNS hostname.<br>It is single-valued, and may optionally include a port number.  | |




#### endpoint {#Config.cluster.controlPlane.endpoint}

Endpoint represents the endpoint URL parsed out of the machine config.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|








### etcd {#Config.cluster.etcd}

EtcdConfig represents the etcd configuration options.



{{< highlight yaml >}}
cluster:
    etcd:
        image: registry.k8s.io/etcd:v3.7.0 # The container image used to create the etcd service.
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
image: registry.k8s.io/etcd:v3.7.0
{{< /highlight >}}</details> | |
|`ca` |PEMEncodedCertificateAndKey |The `ca` is the root certificate authority of the PKI.<br>It is composed of a base64 encoded `crt` and `key`. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
ca:
    crt: LS0tIEVYQU1QTEUgQ0VSVElGSUNBVEUgLS0t
    key: LS0tIEVYQU1QTEUgS0VZIC0tLQ==
{{< /highlight >}}</details> | |
|`extraArgs` |Args |Extra arguments to supply to etcd.<br>Note that the following args are not allowed:<br><br>- `name`<br>- `data-dir`<br>- `initial-cluster-state`<br>- `listen-peer-urls`<br>- `listen-client-urls`<br>- `cert-file`<br>- `key-file`<br>- `trusted-ca-file`<br>- `peer-client-cert-auth`<br>- `peer-cert-file`<br>- `peer-trusted-ca-file`<br>- `peer-key-file`  | |
|`advertisedSubnets` |[]string |The `advertisedSubnets` field configures the networks to pick etcd advertised IP from.<br><br>IPs can be excluded from the list by using negative match with `!`, e.g `!10.0.0.0/8`.<br>Negative subnet matches should be specified last to filter out IPs picked by positive matches.<br>If not specified, advertised IP is selected as the first routable address of the node. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
advertisedSubnets:
    - 10.0.0.0/8
{{< /highlight >}}</details> | |
|`listenSubnets` |[]string |The `listenSubnets` field configures the networks for the etcd to listen for peer and client connections.<br><br>If `listenSubnets` is not set, but `advertisedSubnets` is set, `listenSubnets` defaults to<br>`advertisedSubnets`.<br><br>If neither `advertisedSubnets` nor `listenSubnets` is set, `listenSubnets` defaults to listen on all addresses.<br><br>IPs can be excluded from the list by using negative match with `!`, e.g `!10.0.0.0/8`.<br>Negative subnet matches should be specified last to filter out IPs picked by positive matches.<br>If not specified, advertised IP is selected as the first routable address of the node.  | |






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










