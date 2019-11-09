// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1

//go:generate docgen . /tmp/v1alpha1.md

import (
	"net/url"

	"github.com/talos-systems/talos/pkg/config/machine"
	"github.com/talos-systems/talos/pkg/crypto/x509"
)

// Config defines the v1alpha1 configuration file.
type Config struct {
	//   description: |
	//     Indicates the schema used to decode the contents.
	//   values:
	//     - "`v1alpha1`"
	ConfigVersion string `yaml:"version"`
	//   description: |
	//     Provides machine specific configuration options.
	MachineConfig *MachineConfig `yaml:"machine"`
	//   description: |
	//     Provides cluster specific configuration options.
	ClusterConfig *ClusterConfig `yaml:"cluster"`
}

// MachineConfig reperesents the machine-specific config values
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
	//     Since the rootfs is read only with the exception of `/var`, mounts
	//     are only valid if they are under `/var`.
	//     Note that the partitioning and formating is done only once, if and
	//     only if no existing  partitions are found.
	//   examples:
	//     - |
	//       disks:
	//         - device: /dev/sdb
	//           partitions:
	//             - size: 10000000000
	//               mountpoint: /var/lib/extra
	MachineDisks []machine.Disk `yaml:"disks,omitempty"` // Note: `size` is in units of bytes.
	//   description: |
	//     Used to provide instructions for bare-metal installations.
	//   examples:
	//     - |
	//       install:
	//         disk:
	//         extraDiskArgs:
	//         extraKernelArgs:
	//         image:
	//         bootloader:
	//         wipe:
	//         force:
	MachineInstall *InstallConfig `yaml:"install,omitempty"`
	//   description: |
	//     Allows the addition of user specified files.
	//     Note that the file contents are not required to be base64 encoded.
	//   examples:
	//     - |
	//       kubelet:
	//         contents: |
	//           ...
	//         permissions: 0666
	//         path: /tmp/file.txt
	MachineFiles []machine.File `yaml:"files,omitempty"` // Note: The specified `path` is relative to `/var`.
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
	MachineEnv machine.Env `yaml:"env,omitempty"`
	//   description: |
	//     Used to configure the machine's time settings.
	//   examples:
	//     - |
	//       time:
	//         servers:
	//           - time.cloudflare.com
	MachineTime *TimeConfig `yaml:"time,omitempty"`
}

// ClusterConfig reperesents the cluster-wide config values
type ClusterConfig struct {
	//   description: |
	//     Provides control plane specific configuration options.
	//   examples:
	//     - |
	//       controlPlane:
	//         version: 1.16.2
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
	//         cni: flannel
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
	//     TODO: Remove this.
	//   examples:
	//     - 20d9aafb46d6db4c0958db5b3fc481c8c14fc9b1abd8ac43194f4246b77131be
	CertificateKey string `yaml:"certificateKey"`
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
	APIServer *APIServerConfig `yaml:"apiServer,omitempty"`
	//   description: |
	//     Controller manager server specific configuration options.
	//   examples:
	//     - |
	//       controllerManager:
	//         image: ...
	//         extraArgs:
	//           key: value
	ControllerManager *ControllerManagerConfig `yaml:"controllerManager,omitempty"`
	//   description: |
	//     Scheduler server specific configuration options.
	//   examples:
	//     - |
	//       scheduler:
	//         image: ...
	//         extraArgs:
	//           key: value
	Scheduler *SchedulerConfig `yaml:"scheduler,omitempty"`
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
}

// KubeletConfig reperesents the kubelet config values
type KubeletConfig struct {
	//   description: |
	//     The `image` field is an optional reference to an alternative hyperkube image.
	//   examples:
	//     - "image: docker.io/<org>/hyperkube:latest"
	Image string `yaml:"image,omitempty"`
	//   description: |
	//     The `extraArgs` field is used to provide additional flags to the kubelet.
	//   examples:
	//     - |
	//       extraArgs:
	//         key: value
	ExtraArgs map[string]string `yaml:"extraArgs,omitempty"`
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
	//
	//     ##### machine.network.interfaces.ignore
	//
	//     `ignore` is used to exclude a specific interface from configuration.
	//     This parameter is optional.
	//
	//     ##### machine.network.interfaces.routes
	//
	//     `routes` is used to specify static routes that may be necessary.
	//     This parameter is optional.
	//
	//     Routes can be repeated and includes a `Network` and `Gateway` field.
	NetworkInterfaces []machine.Device `yaml:"interfaces,omitempty"`
	//   description: |
	//     Used to statically set the nameservers for the host.
	//     Defaults to `1.1.1.1` and `8.8.8.8`
	NameServers []string `yaml:"nameservers,omitempty"`
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
	//     Specifies time (ntp) servers to use for setting system time.
	//     Defaults to `pool.ntp.org`
	//
	//     > Note: This parameter only supports a single time server
	TimeServers []string `yaml:"servers,omitempty"`
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
	//     Indicates which version of Kubernetes for all control plane components.
	//   examples:
	//     - 1.16.2
	Version string `yaml:"version"` // Note: The version must be of the format `major.minor.patch`, _without_ a leading `v`.
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
	Image string `yaml:"image,omitempty"`
	//   description: |
	//     Extra arguments to supply to the API server.
	ExtraArgs map[string]string `yaml:"extraArgs,omitempty"`
	//   description: |
	//     Extra certificate subject alternative names for the API server's certificate.
	CertSANs []string `yaml:"certSANs,omitempty"`
}

// ControllerManagerConfig represents kube controller manager config vals.
type ControllerManagerConfig struct {
	//   description: |
	//     The container image used in the controller manager manifest.
	Image string `yaml:"image,omitempty"`
	//   description: |
	//     Extra arguments to supply to the controller manager.
	ExtraArgs map[string]string `yaml:"extraArgs,omitempty"`
}

// SchedulerConfig represents kube scheduler config vals.
type SchedulerConfig struct {
	//   description: |
	//     The container image used in the scheduler manifest.
	Image string `yaml:"image,omitempty"`
	//   description: |
	//     Extra arguments to supply to the scheduler.
	ExtraArgs map[string]string `yaml:"extraArgs,omitempty"`
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
}

// ClusterNetworkConfig represents kube networking config vals.
type ClusterNetworkConfig struct {
	//   description: |
	//     The CNI used.
	//   values:
	//     - flannel
	CNI string `yaml:"cni"`
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

// Bond contains the various options for configuring a bonded interface.
type Bond struct {
	//   description: |
	//     The bond mode.
	Mode string `yaml:"mode"`
	//   description: |
	//     The hash policy.
	HashPolicy string `yaml:"hashpolicy"`
	//   description: |
	//     The LACP rate.
	LACPRate string `yaml:"lacprate"`
	//   description: |
	//     The interfaces if which the bond should be comprised of.
	Interfaces []string `yaml:"interfaces"`
}

// Route represents a network route.
type Route struct {
	//   description: |
	//     TODO.
	Network string `yaml:"network"`
	//   description: |
	//     TODO.
	Gateway string `yaml:"gateway"`
}
