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
|`features` |<a href="#Config.machine.features">FeaturesConfig</a> |Features describe individual Talos features that can be switched on or off. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
features:
    diskQuotaSupport: true # Enable XFS project quota support for EPHEMERAL partition and user disks.
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




### features {#Config.machine.features}

FeaturesConfig describes individual Talos features that can be switched on or off.



{{< highlight yaml >}}
machine:
    features:
        diskQuotaSupport: true # Enable XFS project quota support for EPHEMERAL partition and user disks.
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`diskQuotaSupport` |bool |Enable XFS project quota support for EPHEMERAL partition and user disks.<br>Also enables kubelet tracking of ephemeral disk usage in the kubelet via quota.  | |
|`nodeAddressSortAlgorithm` |string |Select the node address sort algorithm.<br>The 'v1' algorithm sorts addresses by the address itself.<br>The 'v2' algorithm prefers more specific prefixes.<br>If unset, defaults to 'v1'.  | |






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




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
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
|`adminKubeconfig` |<a href="#Config.cluster.adminKubeconfig">AdminKubeconfigConfig</a> |Settings for admin kubeconfig generation.<br>Certificate lifetime can be configured. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
adminKubeconfig:
    certLifetime: 1h0m0s # Admin kubeconfig certificate lifetime (default is 1 year).
{{< /highlight >}}</details> | |




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










