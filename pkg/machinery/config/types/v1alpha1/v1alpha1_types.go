// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1

//go:generate docgen . /tmp/v1alpha1.md

import (
	"net/url"
	"os"
	"time"

	"github.com/opencontainers/runtime-spec/specs-go"

	"github.com/talos-systems/crypto/x509"

	"github.com/talos-systems/talos/pkg/machinery/config"
)

func init() {
	config.Register("v1alpha1", func(version string) (target interface{}) {
		target = &Config{}

		return target
	})
}

// Config defines the v1alpha1 configuration file.
type Config struct {
	//   description: |
	//     Indicates the schema used to decode the contents.
	//   values:
	//     - "`v1alpha1`"
	ConfigVersion string `yaml:"version"`
	//   description: |
	//     Enable verbose logging.
	//   values:
	//     - true
	//     - yes
	//     - false
	//     - no
	ConfigDebug bool `yaml:"debug"`
	//   description: |
	//     Indicates whether to pull the machine config upon every boot.
	//   values:
	//     - true
	//     - yes
	//     - false
	//     - no
	ConfigPersist bool `yaml:"persist"`
	//   description: |
	//     Provides machine specific configuration options.
	MachineConfig *MachineConfig `yaml:"machine"`
	//   description: |
	//     Provides cluster specific configuration options.
	ClusterConfig *ClusterConfig `yaml:"cluster"`
}

// MachineConfig reperesents the machine-specific config values.
type MachineConfig struct {
	//   description: |
	//     Defines the role of the machine within the cluster.
	//
	//     ##### Init
	//
	//     Init node type designates the first control plane node to come up.
	//     You can think of it like a bootstrap node.
	//     This node will perform the initial steps to bootstrap the cluster -- generation of TLS assets, starting of the control plane, etc.
	//
	//     ##### Control Plane
	//
	//     Control Plane node type designates the node as a control plane member.
	//     This means it will host etcd along with the Kubernetes master components such as API Server, Controller Manager, Scheduler.
	//
	//     ##### Worker
	//
	//     Worker node type designates the node as a worker node.
	//     This means it will be an available compute node for scheduling workloads.
	//   values:
	//     - "`init`"
	//     - "`controlplane`"
	//     - "`join`"
	MachineType string `yaml:"type"`
	//   description: |
	//     The `token` is used by a machine to join the PKI of the cluster.
	//     Using this token, a machine will create a certificate signing request (CSR), and request a certificate that will be used as its' identity.
	//   examples:
	//     - "token: 328hom.uqjzh6jnn2eie9oi"
	MachineToken string `yaml:"token"` // Warning: It is important to ensure that this token is correct since a machine's certificate has a short TTL by default
	//   description: |
	//     The root certificate authority of the PKI.
	//     It is composed of a base64 encoded `crt` and `key`.
	//   examples:
	//     - |
	//       ca:
	//         crt: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUJIekNCMHF...
	//         key: LS0tLS1CRUdJTiBFRDI1NTE5IFBSSVZBVEUgS0VZLS0tLS0KTUM...
	MachineCA *x509.PEMEncodedCertificateAndKey `yaml:"ca,omitempty"`
	//   description: |
	//     Extra certificate subject alternative names for the machine's certificate.
	//     By default, all non-loopback interface IPs are automatically added to the certificate's SANs.
	//   examples:
	//     - |
	//       certSANs:
	//         - 10.0.0.10
	//         - 172.16.0.10
	//         - 192.168.0.10
	MachineCertSANs []string `yaml:"certSANs"`
	//   description: |
	//     Used to provide additional options to the kubelet.
	//   examples:
	//     - |
	//       kubelet:
	//         image:
	//         extraArgs:
	//           key: value
	MachineKubelet *KubeletConfig `yaml:"kubelet,omitempty"`
	//   description: |
	//     Used to configure the machine's network.
	//   examples:
	//     - |
	//       network:
	//         hostname: worker-1
	//         interfaces:
	//         nameservers:
	//           - 9.8.7.6
	//           - 8.7.6.5
	MachineNetwork *NetworkConfig `yaml:"network,omitempty"`
	//   description: |
	//     Used to partition, format and mount additional disks.
	//     Since the rootfs is read only with the exception of `/var`, mounts are only valid if they are under `/var`.
	//     Note that the partitioning and formating is done only once, if and only if no existing  partitions are found.
	//     If `size:` is omitted, the partition is sized to occupy full disk.
	//   examples:
	//     - |
	//       disks:
	//         - device: /dev/sdb
	//           partitions:
	//             - mountpoint: /var/lib/extra
	//               size: 10000000000
	//
	MachineDisks []*MachineDisk `yaml:"disks,omitempty"` // Note: `size` is in units of bytes.
	//   description: |
	//     Used to provide instructions for bare-metal installations.
	//   examples:
	//     - |
	//       install:
	//         disk: /dev/sda
	//         extraKernelArgs:
	//           - option=value
	//         image: ghcr.io/talos-systems/installer:latest
	//         bootloader: true
	//         wipe: false
	//         force: false
	MachineInstall *InstallConfig `yaml:"install,omitempty"`
	//   description: |
	//     Allows the addition of user specified files.
	//     The value of `op` can be `create`, `overwrite`, or `append`.
	//     In the case of `create`, `path` must not exist.
	//     In the case of `overwrite`, and `append`, `path` must be a valid file.
	//     If an `op` value of `append` is used, the existing file will be appended.
	//     Note that the file contents are not required to be base64 encoded.
	//   examples:
	//     - |
	//       files:
	//         - content: |
	//             ...
	//           permissions: 0666
	//           path: /tmp/file.txt
	//           op: append
	MachineFiles []*MachineFile `yaml:"files,omitempty"` // Note: The specified `path` is relative to `/var`.
	//   description: |
	//     The `env` field allows for the addition of environment variables to a machine.
	//     All environment variables are set on the machine in addition to every service.
	//   values:
	//     - "`GRPC_GO_LOG_VERBOSITY_LEVEL`"
	//     - "`GRPC_GO_LOG_SEVERITY_LEVEL`"
	//     - "`http_proxy`"
	//     - "`https_proxy`"
	//     - "`no_proxy`"
	//   examples:
	//     - |
	//       env:
	//         GRPC_GO_LOG_VERBOSITY_LEVEL: "99"
	//         GRPC_GO_LOG_SEVERITY_LEVEL: info
	//         https_proxy: http://SERVER:PORT/
	//     - |
	//       env:
	//         GRPC_GO_LOG_SEVERITY_LEVEL: error
	//         https_proxy: https://USERNAME:PASSWORD@SERVER:PORT/
	//     - |
	//       env:
	//         https_proxy: http://DOMAIN\\USERNAME:PASSWORD@SERVER:PORT/
	MachineEnv Env `yaml:"env,omitempty"`
	//   description: |
	//     Used to configure the machine's time settings.
	//   examples:
	//     - |
	//       time:
	//         servers:
	//           - time.cloudflare.com
	MachineTime *TimeConfig `yaml:"time,omitempty"`
	//   description: |
	//     Used to configure the machine's sysctls.
	//   examples:
	//     - |
	//       sysctls:
	//         kernel.domainname: talos.dev
	//         net.ipv4.ip_forward: "0"
	MachineSysctls map[string]string `yaml:"sysctls,omitempty"`
	//   description: |
	//     Used to configure the machine's container image registry mirrors.
	//
	//     Automatically generates matching CRI configuration for registry mirrors.
	//
	//     Section `mirrors` allows to redirect requests for images to non-default registry,
	//     which might be local registry or caching mirror.
	//
	//     Section `config` provides a way to authenticate to the registry with TLS client
	//     identity, provide registry CA, or authentication information.
	//     Authentication information has same meaning with the corresponding field in `.docker/config.json`.
	//
	//     See also matching configuration for [CRI containerd plugin](https://github.com/containerd/cri/blob/master/docs/registry.md).
	//   examples:
	//     - |
	//       registries:
	//         mirrors:
	//           docker.io:
	//             endpoints:
	//               - https://registry-1.docker.io
	//           '*':
	//             endpoints:
	//               - http://some.host:123/
	//        config:
	//         "some.host:123":
	//           tls:
	//             CA: ... # base64-encoded CA certificate in PEM format
	//             clientIdentity:
	//               cert: ...  # base64-encoded client certificate in PEM format
	//               key: ...  # base64-encoded client key in PEM format
	//           auth:
	//             username: ...
	//             password: ...
	//             auth: ...
	//             identityToken: ...
	MachineRegistries RegistriesConfig `yaml:"registries,omitempty"`
}

// ClusterConfig reperesents the cluster-wide config values.
type ClusterConfig struct {
	//   description: |
	//     Provides control plane specific configuration options.
	//   examples:
	//     - |
	//       controlPlane:
	//         endpoint: https://1.2.3.4
	//         localAPIServerPort: 443
	ControlPlane *ControlPlaneConfig `yaml:"controlPlane"`
	//   description: |
	//     Configures the cluster's name.
	ClusterName string `yaml:"clusterName,omitempty"`
	//   description: |
	//     Provides cluster network configuration.
	//   examples:
	//     - |
	//       network:
	//         cni:
	//           name: flannel
	//         dnsDomain: cluster.local
	//         podSubnets:
	//         - 10.244.0.0/16
	//         serviceSubnets:
	//         - 10.96.0.0/12
	ClusterNetwork *ClusterNetworkConfig `yaml:"network,omitempty"`
	//   description: |
	//     The [bootstrap token](https://kubernetes.io/docs/reference/access-authn-authz/bootstrap-tokens/).
	//   examples:
	//     - wlzjyw.bei2zfylhs2by0wd
	BootstrapToken string `yaml:"token,omitempty"`
	//   description: |
	//     The key used for the [encryption of secret data at rest](https://kubernetes.io/docs/tasks/administer-cluster/encrypt-data/).
	//   examples:
	//     - z01mye6j16bspJYtTB/5SFX8j7Ph4JXxM2Xuu4vsBPM=
	ClusterAESCBCEncryptionSecret string `yaml:"aescbcEncryptionSecret"`
	//   description: |
	//     The base64 encoded root certificate authority used by Kubernetes.
	//   examples:
	//     - |
	//       ca:
	//         crt: LS0tLS1CRUdJTiBDRV...
	//         key: LS0tLS1CRUdJTiBSU0...
	ClusterCA *x509.PEMEncodedCertificateAndKey `yaml:"ca,omitempty"`
	//   description: |
	//     API server specific configuration options.
	//   examples:
	//     - |
	//       apiServer:
	//         image: ...
	//         extraArgs:
	//           key: value
	//         certSANs:
	//           - 1.2.3.4
	//           - 5.6.7.8
	APIServerConfig *APIServerConfig `yaml:"apiServer,omitempty"`
	//   description: |
	//     Controller manager server specific configuration options.
	//   examples:
	//     - |
	//       controllerManager:
	//         image: ...
	//         extraArgs:
	//           key: value
	ControllerManagerConfig *ControllerManagerConfig `yaml:"controllerManager,omitempty"`
	//   description: |
	//     Kube-proxy server-specific configuration options
	//   examples:
	//     - |
	//       proxy:
	//         mode: ipvs
	//         extraArgs:
	//           key: value
	ProxyConfig *ProxyConfig `yaml:"proxy,omitempty"`
	//   description: |
	//     Scheduler server specific configuration options.
	//   examples:
	//     - |
	//       scheduler:
	//         image: ...
	//         extraArgs:
	//           key: value
	SchedulerConfig *SchedulerConfig `yaml:"scheduler,omitempty"`
	//   description: |
	//     Etcd specific configuration options.
	//   examples:
	//     - |
	//       etcd:
	//         ca:
	//           crt: LS0tLS1CRUdJTiBDRV...
	//           key: LS0tLS1CRUdJTiBSU0...
	//         image: ...
	EtcdConfig *EtcdConfig `yaml:"etcd,omitempty"`
	//   description: |
	//     Pod Checkpointer specific configuration options.
	//   examples:
	//     - |
	//       podCheckpointer:
	//         image: ...
	PodCheckpointerConfig *PodCheckpointer `yaml:"podCheckpointer,omitempty"`
	//   description: |
	//     Core DNS specific configuration options.
	//   examples:
	//     - |
	//       coreDNS:
	//         image: ...
	CoreDNSConfig *CoreDNS `yaml:"coreDNS,omitempty"`
	//   description: |
	//     A list of urls that point to additional manifests.
	//     These will get automatically deployed by bootkube.
	//   examples:
	//     - |
	//       extraManifests:
	//         - "https://www.mysweethttpserver.com/manifest1.yaml"
	//         - "https://www.mysweethttpserver.com/manifest2.yaml"
	ExtraManifests []string `yaml:"extraManifests,omitempty"`
	//   description: |
	//     A map of key value pairs that will be added while fetching the ExtraManifests.
	//   examples:
	//     - |
	//       extraManifestHeaders:
	//         Token: "1234567"
	//         X-ExtraInfo: info
	ExtraManifestHeaders map[string]string `yaml:"extraManifestHeaders,omitempty"`
	//   description: |
	//     Settings for admin kubeconfig generation.
	//     Certificate lifetime can be configured.
	//   examples:
	//     - |
	//       adminKubeconfig:
	//         certLifetime: 1h
	AdminKubeconfigConfig AdminKubeconfigConfig `yaml:"adminKubeconfig,omitempty"`
}

// KubeletConfig reperesents the kubelet config values.
type KubeletConfig struct {
	//   description: |
	//     The `image` field is an optional reference to an alternative kubelet image.
	//   examples:
	//     - "image: docker.io/<org>/kubelet:latest"
	KubeletImage string `yaml:"image,omitempty"`
	//   description: |
	//     The `extraArgs` field is used to provide additional flags to the kubelet.
	//   examples:
	//     - |
	//       extraArgs:
	//         key: value
	KubeletExtraArgs map[string]string `yaml:"extraArgs,omitempty"`
	//   description: |
	//     The `extraMounts` field is used to add additional mounts to the kubelet container.
	//   examples:
	//     - |
	//       extraMounts:
	//         - source: /var/lib/example
	//           destination: /var/lib/example
	//           type: bind
	//           options:
	//             - rshared
	//             - ro
	KubeletExtraMounts []specs.Mount `yaml:"extraMounts,omitempty"`
}

// NetworkConfig reperesents the machine's networking config values.
type NetworkConfig struct {
	//   description: |
	//     Used to statically set the hostname for the host.
	NetworkHostname string `yaml:"hostname,omitempty"`
	//   description: |
	//     `interfaces` is used to define the network interface configuration.
	//     By default all network interfaces will attempt a DHCP discovery.
	//     This can be further tuned through this configuration parameter.
	//
	//     ##### machine.network.interfaces.interface
	//
	//     This is the interface name that should be configured.
	//
	//     ##### machine.network.interfaces.cidr
	//
	//     `cidr` is used to specify a static IP address to the interface.
	//     This should be in proper CIDR notation ( `192.168.2.5/24` ).
	//
	//     > Note: This option is mutually exclusive with DHCP.
	//
	//     ##### machine.network.interfaces.dhcp
	//
	//     `dhcp` is used to specify that this device should be configured via DHCP.
	//
	//     The following DHCP options are supported:
	//
	//     - `OptionClasslessStaticRoute`
	//     - `OptionDomainNameServer`
	//     - `OptionDNSDomainSearchList`
	//     - `OptionHostName`
	//
	//     > Note: This option is mutually exclusive with CIDR.
	//     >
	//     > Note: To configure an interface with *only* IPv6 SLAAC addressing, CIDR should be set to "" and DHCP to false
	//     > in order for Talos to skip configuration of addresses.
	//     > All other options will still apply.
	//
	//     ##### machine.network.interfaces.ignore
	//
	//     `ignore` is used to exclude a specific interface from configuration.
	//     This parameter is optional.
	//
	//     ##### machine.network.interfaces.dummy
	//
	//     `dummy` is used to specify that this interface should be a virtual-only, dummy interface.
	//     This parameter is optional.
	//
	//     ##### machine.network.interfaces.routes
	//
	//     `routes` is used to specify static routes that may be necessary.
	//     This parameter is optional.
	//
	//     Routes can be repeated and includes a `Network` and `Gateway` field.
	NetworkInterfaces []*Device `yaml:"interfaces,omitempty"`
	//   description: |
	//     Used to statically set the nameservers for the host.
	//     Defaults to `1.1.1.1` and `8.8.8.8`
	NameServers []string `yaml:"nameservers,omitempty"`
	//   description: |
	//     Allows for extra entries to be added to /etc/hosts file
	//   examples:
	//     - |
	//       extraHostEntries:
	//         - ip: 192.168.1.100
	//           aliases:
	//             - test
	//             - test.domain.tld
	ExtraHostEntries []*ExtraHost `yaml:"extraHostEntries,omitempty"`
}

// InstallConfig represents the installation options for preparing a node.
type InstallConfig struct {
	//   description: |
	//     The disk used to install the bootloader, and ephemeral partitions.
	//   examples:
	//     - /dev/sda
	//     - /dev/nvme0
	InstallDisk string `yaml:"disk,omitempty"`
	//   description: |
	//     Allows for supplying extra kernel args to the bootloader config.
	//   examples:
	//     - |
	//       extraKernelArgs:
	//         - a=b
	InstallExtraKernelArgs []string `yaml:"extraKernelArgs,omitempty"`
	//   description: |
	//     Allows for supplying the image used to perform the installation.
	//   examples:
	//     - |
	//       image: docker.io/<org>/installer:latest
	InstallImage string `yaml:"image,omitempty"`
	//   description: |
	//     Indicates if a bootloader should be installed.
	//   values:
	//     - true
	//     - yes
	//     - false
	//     - no
	InstallBootloader bool `yaml:"bootloader,omitempty"`
	//   description: |
	//     Indicates if zeroes should be written to the `disk` before performing and installation.
	//     Defaults to `true`.
	//   values:
	//     - true
	//     - yes
	//     - false
	//     - no
	InstallWipe bool `yaml:"wipe"`
	//   description: |
	//     Indicates if filesystems should be forcefully created.
	//   values:
	//     - true
	//     - yes
	//     - false
	//     - no
	InstallForce bool `yaml:"force"`
}

// TimeConfig represents the options for configuring time on a node.
type TimeConfig struct {
	//   description: |
	//     Indicates if time (ntp) is enabled for the machine
	//     Defaults to `true`.
	TimeEnabled bool `yaml:"enabled"`
	//   description: |
	//     Specifies time (ntp) servers to use for setting system time.
	//     Defaults to `pool.ntp.org`
	//
	//     > Note: This parameter only supports a single time server
	TimeServers []string `yaml:"servers,omitempty"`
}

// RegistriesConfig represents the image pull options.
type RegistriesConfig struct {
	//   description: |
	//     Specifies mirror configuration for each registry.
	//     This setting allows to use local pull-through caching registires,
	//     air-gapped installations, etc.
	//
	//     Registry name is the first segment of image identifier, with 'docker.io'
	//     being default one.
	//     Name '*' catches any registry names not specified explicitly.
	RegistryMirrors map[string]*RegistryMirrorConfig `yaml:"mirrors,omitempty"`
	//   description: |
	//     Specifies TLS & auth configuration for HTTPS image registries.
	//     Mutual TLS can be enabled with 'clientIdentity' option.
	//
	//     TLS configuration can be skipped if registry has trusted
	//     server certificate.
	RegistryConfig map[string]*RegistryConfig `yaml:"config,omitempty"`
}

// PodCheckpointer represents the pod-checkpointer config values.
type PodCheckpointer struct {
	//   description: |
	//     The `image` field is an override to the default pod-checkpointer image.
	PodCheckpointerImage string `yaml:"image,omitempty"`
}

// CoreDNS represents the coredns config values.
type CoreDNS struct {
	//   description: |
	//     The `image` field is an override to the default coredns image.
	CoreDNSImage string `yaml:"image,omitempty"`
}

// Endpoint struct holds the endpoint url parsed out of machine config.
type Endpoint struct {
	*url.URL
}

// UnmarshalYAML is a custom unmarshaller for the endpoint struct.
func (e *Endpoint) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var endpoint string

	if err := unmarshal(&endpoint); err != nil {
		return err
	}

	url, err := url.Parse(endpoint)
	if err != nil {
		return err
	}

	*e = Endpoint{url}

	return nil
}

// MarshalYAML is a custom unmarshaller for the endpoint struct.
func (e *Endpoint) MarshalYAML() (interface{}, error) {
	return e.URL.String(), nil
}

// ControlPlaneConfig represents control plane config vals.
type ControlPlaneConfig struct {
	//   description: |
	//     Endpoint is the canonical controlplane endpoint, which can be an IP address or a DNS hostname.
	//     It is single-valued, and may optionally include a port number.
	//   examples:
	//     - https://1.2.3.4:443
	Endpoint *Endpoint `yaml:"endpoint"`
	//   description: |
	//     The port that the API server listens on internally.
	//     This may be different than the port portion listed in the endpoint field above.
	//     The default is 6443.
	LocalAPIServerPort int `yaml:"localAPIServerPort,omitempty"`
}

// APIServerConfig represents kube apiserver config vals.
type APIServerConfig struct {
	//   description: |
	//     The container image used in the API server manifest.
	ContainerImage string `yaml:"image,omitempty"`
	//   description: |
	//     Extra arguments to supply to the API server.
	ExtraArgsConfig map[string]string `yaml:"extraArgs,omitempty"`
	//   description: |
	//     Extra certificate subject alternative names for the API server's certificate.
	CertSANs []string `yaml:"certSANs,omitempty"`
}

// ControllerManagerConfig represents kube controller manager config vals.
type ControllerManagerConfig struct {
	//   description: |
	//     The container image used in the controller manager manifest.
	ContainerImage string `yaml:"image,omitempty"`
	//   description: |
	//     Extra arguments to supply to the controller manager.
	ExtraArgsConfig map[string]string `yaml:"extraArgs,omitempty"`
}

// ProxyConfig represents the kube proxy configuration values.
type ProxyConfig struct {
	//   description: |
	//     The container image used in the kube-proxy manifest.
	ContainerImage string `yaml:"image,omitempty"`
	//   description: |
	//     proxy mode of kube-proxy.
	//     By default, this is 'iptables'.
	ModeConfig string `yaml:"mode,omitempty"`
	//   description: |
	//     Extra arguments to supply to kube-proxy.
	ExtraArgsConfig map[string]string `yaml:"extraArgs,omitempty"`
}

// SchedulerConfig represents kube scheduler config vals.
type SchedulerConfig struct {
	//   description: |
	//     The container image used in the scheduler manifest.
	ContainerImage string `yaml:"image,omitempty"`
	//   description: |
	//     Extra arguments to supply to the scheduler.
	ExtraArgsConfig map[string]string `yaml:"extraArgs,omitempty"`
}

// EtcdConfig represents etcd config vals.
type EtcdConfig struct {
	//   description: |
	//     The container image used to create the etcd service.
	ContainerImage string `yaml:"image,omitempty"`
	//   description: |
	//     The `ca` is the root certificate authority of the PKI.
	//     It is composed of a base64 encoded `crt` and `key`.
	//   examples:
	//     - |
	//       ca:
	//         crt: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUJIekNCMHF...
	//         key: LS0tLS1CRUdJTiBFRDI1NTE5IFBSSVZBVEUgS0VZLS0tLS0KTUM...
	RootCA *x509.PEMEncodedCertificateAndKey `yaml:"ca"`
	//   description: |
	//     Extra arguments to supply to etcd.
	//     Note that the following args are not allowed:
	//
	//     - `name`
	//     - `data-dir`
	//     - `initial-cluster-state`
	//     - `listen-peer-urls`
	//     - `listen-client-urls`
	//     - `cert-file`
	//     - `key-file`
	//     - `trusted-ca-file`
	//     - `peer-client-cert-auth`
	//     - `peer-cert-file`
	//     - `peer-trusted-ca-file`
	//     - `peer-key-file`
	//   examples:
	//     - |
	//       extraArgs:
	//         initial-cluster: https://1.2.3.4:2380
	//         advertise-client-urls: https://1.2.3.4:2379
	EtcdExtraArgs map[string]string `yaml:"extraArgs,omitempty"`
}

// ClusterNetworkConfig represents kube networking config vals.
type ClusterNetworkConfig struct {
	//   description: |
	//     The CNI used.
	//     Composed of "name" and "url".
	//     The "name" key only supports upstream bootkube options of "flannel" or "custom".
	//     URLs is only used if name is equal to "custom".
	//     URLs should point to a single yaml file that will get deployed.
	//     Empty struct or any other name will default to bootkube's flannel.
	//   examples:
	//     - |
	//       cni:
	//         name: "custom"
	//         urls:
	//           - "https://www.mysweethttpserver.com/supersecretcni.yaml"
	CNI *CNIConfig `yaml:"cni,omitempty"`
	//   description: |
	//     The domain used by Kubernetes DNS.
	//     The default is `cluster.local`
	//   examples:
	//     - cluser.local
	DNSDomain string `yaml:"dnsDomain"`
	//   description: |
	//     The pod subnet CIDR.
	//   examples:
	//     -  |
	//       podSubnets:
	//         - 10.244.0.0/16
	PodSubnet []string `yaml:"podSubnets"`
	//   description: |
	//     The service subnet CIDR.
	//   examples:
	//     -  |
	//       serviceSubnets:
	//         - 10.96.0.0/12
	ServiceSubnet []string `yaml:"serviceSubnets"`
}

// CNIConfig contains the info about which CNI we'll deploy.
type CNIConfig struct {
	//   description: |
	//     Name of CNI to use.
	CNIName string `yaml:"name"`
	//   description: |
	//     URLs containing manifests to apply for CNI.
	CNIUrls []string `yaml:"urls,omitempty"`
}

// AdminKubeconfigConfig contains admin kubeconfig settings.
type AdminKubeconfigConfig struct {
	//   description: |
	//     Admin kubeconfig certificate lifetime (default is 1 year).
	//     Field format accepts any Go time.Duration format ('1h' for one hour, '10m' for ten minutes).
	AdminKubeconfigCertLifetime time.Duration `yaml:"certLifetime,omitempty"`
}

// MachineDisk represents the options available for partitioning, formatting, and
// mounting extra disks.
type MachineDisk struct {
	//   description: The name of the disk to use.
	DeviceName string `yaml:"device,omitempty"`
	//   description: A list of partitions to create on the disk.
	DiskPartitions []*DiskPartition `yaml:"partitions,omitempty"`
}

// DiskPartition represents the options for a device partition.
type DiskPartition struct {
	//   description: |
	//     This size of the partition in bytes.
	DiskSize uint `yaml:"size,omitempty"`
	//   description:
	//     Where to mount the partition.
	DiskMountPoint string `yaml:"mountpoint,omitempty"`
}

// Env represents a set of environment variables.
type Env = map[string]string

// MachineFile represents a file to write to disk.
type MachineFile struct {
	//   description: The contents of file.
	FileContent string `yaml:"content"`
	//   description: The file's permissions in octal.
	FilePermissions os.FileMode `yaml:"permissions"`
	//   description: The path of the file.
	FilePath string `yaml:"path"`
	//   description: The operation to use
	//   values:
	//     - create
	//     - append
	FileOp string `yaml:"op"`
}

// ExtraHost represents a host entry in /etc/hosts.
type ExtraHost struct {
	//   description: The IP of the host.
	HostIP string `yaml:"ip"`
	//   description: The host alias.
	HostAliases []string `yaml:"aliases"`
}

// Device represents a network interface.
type Device struct {
	//   description: The interface name.
	DeviceInterface string `yaml:"interface"`
	//   description: The CIDR to use.
	DeviceCIDR string `yaml:"cidr"`
	//   description: A list of routes associated with the interface.
	DeviceRoutes []*Route `yaml:"routes"`
	//   description: Bond specific options.
	DeviceBond *Bond `yaml:"bond"`
	//   description: VLAN specific options.
	DeviceVlans []*Vlan `yaml:"vlans"`
	//   description: The interface's MTU.
	DeviceMTU int `yaml:"mtu"`
	//   description: Indicates if DHCP should be used.
	DeviceDHCP bool `yaml:"dhcp"`
	//   description: Indicates if the interface should be ignored.
	DeviceIgnore bool `yaml:"ignore"`
	//   description: Indicates if the interface is a dummy interface.
	DeviceDummy bool `yaml:"dummy"`
}

// Bond contains the various options for configuring a
// bonded interface.
type Bond struct {
	//   description: The interfaces that make up the bond.
	BondInterfaces []string `yaml:"interfaces"`
	//   description: |
	//     A bond option.
	//     Please see the official kernel documentation.
	BondARPIPTarget []string `yaml:"arpIPTarget"`
	//   description: |
	//     A bond option.
	//     Please see the official kernel documentation.
	BondMode string `yaml:"mode"`
	//   description: |
	//     A bond option.
	//     Please see the official kernel documentation.
	BondHashPolicy string `yaml:"xmitHashPolicy"`
	//   description: |
	//     A bond option.
	//     Please see the official kernel documentation.
	BondLACPRate string `yaml:"lacpRate"`
	//   description: |
	//     A bond option.
	//     Please see the official kernel documentation.
	BondADActorSystem string `yaml:"adActorSystem"`
	//   description: |
	//     A bond option.
	//     Please see the official kernel documentation.
	BondARPValidate string `yaml:"arpValidate"`
	//   description: |
	//     A bond option.
	//     Please see the official kernel documentation.
	BondARPAllTargets string `yaml:"arpAllTargets"`
	//   description: |
	//     A bond option.
	//     Please see the official kernel documentation.
	BondPrimary string `yaml:"primary"`
	//   description: |
	//     A bond option.
	//     Please see the official kernel documentation.
	BondPrimaryReselect string `yaml:"primaryReselect"`
	//   description: |
	//     A bond option.
	//     Please see the official kernel documentation.
	BondFailOverMac string `yaml:"failOverMac"`
	//   description: |
	//     A bond option.
	//     Please see the official kernel documentation.
	BondADSelect string `yaml:"adSelect"`
	//   description: |
	//     A bond option.
	//     Please see the official kernel documentation.
	BondMIIMon uint32 `yaml:"miimon"`
	//   description: |
	//     A bond option.
	//     Please see the official kernel documentation.
	BondUpDelay uint32 `yaml:"updelay"`
	//   description: |
	//     A bond option.
	//     Please see the official kernel documentation.
	BondDownDelay uint32 `yaml:"downdelay"`
	//   description: |
	//     A bond option.
	//     Please see the official kernel documentation.
	BondARPInterval uint32 `yaml:"arpInterval"`
	//   description: |
	//     A bond option.
	//     Please see the official kernel documentation.
	BondResendIGMP uint32 `yaml:"resendIgmp"`
	//   description: |
	//     A bond option.
	//     Please see the official kernel documentation.
	BondMinLinks uint32 `yaml:"minLinks"`
	//   description: |
	//     A bond option.
	//     Please see the official kernel documentation.
	BondLPInterval uint32 `yaml:"lpInterval"`
	//   description: |
	//     A bond option.
	//     Please see the official kernel documentation.
	BondPacketsPerSlave uint32 `yaml:"packetsPerSlave"`
	//   description: |
	//     A bond option.
	//     Please see the official kernel documentation.
	BondNumPeerNotif uint8 `yaml:"numPeerNotif"`
	//   description: |
	//     A bond option.
	//     Please see the official kernel documentation.
	BondTLBDynamicLB uint8 `yaml:"tlbDynamicLb"`
	//   description: |
	//     A bond option.
	//     Please see the official kernel documentation.
	BondAllSlavesActive uint8 `yaml:"allSlavesActive"`
	//   description: |
	//     A bond option.
	//     Please see the official kernel documentation.
	BondUseCarrier bool `yaml:"useCarrier"`
	//   description: |
	//     A bond option.
	//     Please see the official kernel documentation.
	BondADActorSysPrio uint16 `yaml:"adActorSysPrio"`
	//   description: |
	//     A bond option.
	//     Please see the official kernel documentation.
	BondADUserPortKey uint16 `yaml:"adUserPortKey"`
	//   description: |
	//     A bond option.
	//     Please see the official kernel documentation.
	BondPeerNotifyDelay uint32 `yaml:"peerNotifyDelay"`
}

// Vlan represents vlan settings for a device.
type Vlan struct {
	//   description: The CIDR to use.
	VlanCIDR string `yaml:"cidr"`
	//   description: A list of routes associated with the VLAN.
	VlanRoutes []*Route `yaml:"routes"`
	//   description: Indicates if DHCP should be used.
	VlanDHCP bool `yaml:"dhcp"`
	//   description: The VLAN's ID.
	VlanID uint16 `yaml:"vlanId"`
}

// Route represents a network route.
type Route struct {
	//   description: The route's network.
	RouteNetwork string `yaml:"network"`
	//   description: The route's gateway.
	RouteGateway string `yaml:"gateway"`
}

// RegistryMirrorConfig represents mirror configuration for a registry.
type RegistryMirrorConfig struct {
	//   description: |
	//     List of endpoints (URLs) for registry mirrors to use.
	//     Endpoint configures HTTP/HTTPS access mode, host name,
	//     port and path (if path is not set, it defaults to `/v2`).
	MirrorEndpoints []string `yaml:"endpoints"`
}

// RegistryConfig specifies auth & TLS config per registry.
type RegistryConfig struct {
	//   description: The TLS configuration for this registry.
	RegistryTLS *RegistryTLSConfig `yaml:"tls,omitempty"`
	//   description: The auth configuration for this registry.
	RegistryAuth *RegistryAuthConfig `yaml:"auth,omitempty"`
}

// RegistryAuthConfig specifies authentication configuration for a registry.
type RegistryAuthConfig struct {
	//   description: |
	//     Optional registry authentication.
	//     The meaning of each field is the same with the corresponding field in .docker/config.json.
	RegistryUsername string `yaml:"username"`
	//   description: |
	//     Optional registry authentication.
	//     The meaning of each field is the same with the corresponding field in .docker/config.json.
	RegistryPassword string `yaml:"password"`
	//   description: |
	//     Optional registry authentication.
	//     The meaning of each field is the same with the corresponding field in .docker/config.json.
	RegistryAuth string `yaml:"auth"`
	//   description: |
	//     Optional registry authentication.
	//     The meaning of each field is the same with the corresponding field in .docker/config.json.
	RegistryIdentityToken string `yaml:"identityToken"`
}

// RegistryTLSConfig specifies TLS config for HTTPS registries.
type RegistryTLSConfig struct {
	//   description: |
	//     Enable mutual TLS authentication with the registry.
	//     Client certificate and key should be base64-encoded.
	//   examples:
	//     - |
	//       clientIdentity:
	//         crt: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUJIekNCMHF...
	//         key: LS0tLS1CRUdJTiBFRDI1NTE5IFBSSVZBVEUgS0VZLS0tLS0KTUM...
	TLSClientIdentity *x509.PEMEncodedCertificateAndKey `yaml:"clientIdentity,omitempty"`
	//   description: |
	//     CA registry certificate to add the list of trusted certificates.
	//     Certificate should be base64-encoded.
	TLSCA []byte `yaml:"ca,omitempty"`
	//   description: |
	//     Skip TLS server certificate verification (not recommended).
	TLSInsecureSkipVerify bool `yaml:"insecureSkipVerify,omitempty"`
}
