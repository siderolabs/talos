// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config

import (
	"crypto/tls"
	"net/url"
	"os"
	"time"

	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/siderolabs/crypto/x509"

	"github.com/siderolabs/talos/pkg/machinery/cel"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
)

// MachineConfig defines the requirements for a config that pertains to machine
// related options.
//
//nolint:interfacebloat
type MachineConfig interface {
	Install() Install
	Security() Security
	Network() MachineNetwork
	Disks() []Disk
	Time() Time
	Env() Env
	Files() ([]File, error)
	Type() machine.Type
	Controlplane() MachineControlPlane
	Pods() []map[string]any
	Kubelet() Kubelet
	Sysctls() map[string]string
	Sysfs() map[string]string
	Registries() Registries
	SystemDiskEncryption() SystemDiskEncryption
	Features() Features
	Udev() UdevConfig
	Logging() Logging
	Kernel() Kernel
	SeccompProfiles() []SeccompProfile
	NodeLabels() NodeLabels
	NodeAnnotations() NodeAnnotations
	NodeTaints() NodeTaints
	BaseRuntimeSpecOverrides() map[string]any
}

// SeccompProfile defines the requirements for a config that pertains to seccomp
// related options.
type SeccompProfile interface {
	Name() string
	Value() map[string]any
}

// NodeLabels defines the labels that should be set on a node.
type NodeLabels map[string]string

// NodeAnnotations defines the annotations that should be set on a node.
type NodeAnnotations map[string]string

// NodeTaints defines the taints that should be set on a node.
type NodeTaints map[string]string

// Disk represents the options available for partitioning, formatting, and
// mounting extra disks.
type Disk interface {
	Device() string
	Partitions() []Partition
}

// Partition represents the options for a device partition.
type Partition interface {
	Size() uint64
	MountPoint() string
}

// Env represents a set of environment variables.
type Env = map[string]string

// File represents a file to write to disk.
type File interface {
	Content() string
	Permissions() os.FileMode
	Path() string
	Op() string
}

// Install defines the requirements for a config that pertains to install
// related options.
type Install interface {
	Image() string
	Extensions() []Extension
	Disk() string
	DiskMatchExpression() (*cel.Expression, error)
	ExtraKernelArgs() []string
	Zero() bool
	LegacyBIOSSupport() bool
	WithBootloader() bool
}

// Extension defines the system extension.
type Extension interface {
	Image() string
}

// Security defines the requirements for a config that pertains to security
// related options.
type Security interface {
	IssuingCA() *x509.PEMEncodedCertificateAndKey
	AcceptedCAs() []*x509.PEMEncodedCertificate
	Token() string
	CertSANs() []string
}

// MachineControlPlane defines the requirements for a config that pertains to Controlplane
// related options.
type MachineControlPlane interface {
	ControllerManager() MachineControllerManager
	Scheduler() MachineScheduler
}

// MachineControllerManager defines the requirements for a config that pertains to ControllerManager
// related options.
//
//nolint:iface
type MachineControllerManager interface {
	Disabled() bool
}

// MachineScheduler defines the requirements for a config that pertains to Scheduler
// related options.
//
//nolint:iface
type MachineScheduler interface {
	Disabled() bool
}

// MachineNetwork defines the requirements for a config that pertains to network
// related options.
type MachineNetwork interface {
	Hostname() string
	Resolvers() []string
	SearchDomains() []string
	Devices() []Device
	ExtraHosts() []ExtraHost
	KubeSpan() KubeSpan
	DisableSearchDomain() bool
}

// ExtraHost represents a host entry in /etc/hosts.
type ExtraHost interface {
	IP() string
	Aliases() []string
}

// Device represents a network interface.
//
//nolint:interfacebloat
type Device interface {
	Interface() string
	Addresses() []string
	Routes() []Route
	Bond() Bond
	Bridge() Bridge
	BridgePort() BridgePort
	Vlans() []Vlan
	MTU() int
	DHCP() bool
	Ignore() bool
	Dummy() bool
	DHCPOptions() DHCPOptions
	VIPConfig() VIPConfig
	WireguardConfig() WireguardConfig
	Selector() NetworkDeviceSelector
}

// DHCPOptions represents a set of DHCP options.
type DHCPOptions interface {
	RouteMetric() uint32
	IPv4() bool
	IPv6() bool
	DUIDv6() string
}

// VIPConfig contains settings for the Virtual (shared) IP setup.
type VIPConfig interface {
	IP() string
	EquinixMetal() VIPEquinixMetal
	HCloud() VIPHCloud
}

// VIPEquinixMetal contains Equinix Metal API VIP settings.
//
//nolint:iface
type VIPEquinixMetal interface {
	APIToken() string
}

// VIPHCloud contains Hetzner Cloud API VIP settings.
//
//nolint:iface
type VIPHCloud interface {
	APIToken() string
}

// WireguardConfig contains settings for configuring Wireguard network interface.
type WireguardConfig interface {
	PrivateKey() string
	ListenPort() int
	FirewallMark() int
	Peers() []WireguardPeer
}

// WireguardPeer a WireGuard device peer configuration.
type WireguardPeer interface {
	PublicKey() string
	Endpoint() string
	PersistentKeepaliveInterval() time.Duration
	AllowedIPs() []string
}

// Bond contains the various options for configuring a
// bonded interface.
//
//nolint:interfacebloat
type Bond interface {
	Interfaces() []string
	Selectors() []NetworkDeviceSelector
	ARPIPTarget() []string
	Mode() string
	HashPolicy() string
	LACPRate() string
	ADActorSystem() string
	ARPValidate() string
	ARPAllTargets() string
	Primary() string
	PrimaryReselect() string
	FailOverMac() string
	ADSelect() string
	MIIMon() uint32
	UpDelay() uint32
	DownDelay() uint32
	ARPInterval() uint32
	ResendIGMP() uint32
	MinLinks() uint32
	LPInterval() uint32
	PacketsPerSlave() uint32
	NumPeerNotif() uint8
	TLBDynamicLB() uint8
	AllSlavesActive() uint8
	UseCarrier() bool
	ADActorSysPrio() uint16
	ADUserPortKey() uint16
	PeerNotifyDelay() uint32
}

// STP contains the Spanning Tree Protocol settings for a bridge.
//
//nolint:iface
type STP interface {
	Enabled() bool
}

// BridgeVLAN contains the VLAN settings for a bridge.
type BridgeVLAN interface {
	FilteringEnabled() bool
}

// Bridge contains the options for configuring a bridged interface.
type Bridge interface {
	Interfaces() []string
	STP() STP
	VLAN() BridgeVLAN
}

// BridgePort contains the options for a bridge port.
type BridgePort interface {
	Master() string
}

// Vlan represents vlan settings for a device.
type Vlan interface {
	Addresses() []string
	Routes() []Route
	DHCP() bool
	ID() uint16
	MTU() uint32
	VIPConfig() VIPConfig
	DHCPOptions() DHCPOptions
}

// Route represents a network route.
type Route interface {
	Network() string
	Gateway() string
	Source() string
	Metric() uint32
	MTU() uint32
}

// KubeSpan configures KubeSpan feature.
type KubeSpan interface {
	Enabled() bool
	ForceRouting() bool
	AdvertiseKubernetesNetworks() bool
	HarvestExtraEndpoints() bool
	MTU() uint32
	Filters() KubeSpanFilters
}

// KubeSpanFilters configures KubeSpan filters.
type KubeSpanFilters interface {
	Endpoints() []string
}

// NetworkDeviceSelector defines the set of fields that can be used to pick network a device.
type NetworkDeviceSelector interface {
	Bus() string
	HardwareAddress() string
	PermanentAddress() string
	PCIID() string
	KernelDriver() string
	Physical() *bool
}

// Time defines the requirements for a config that pertains to time related
// options.
type Time interface {
	Disabled() bool
	Servers() []string
	BootTimeout() time.Duration
}

// Kubelet defines the requirements for a config that pertains to kubelet
// related options.
//
//nolint:interfacebloat
type Kubelet interface {
	Image() string
	ClusterDNS() []string
	ExtraArgs() map[string]string
	ExtraMounts() []specs.Mount
	ExtraConfig() map[string]any
	CredentialProviderConfig() map[string]any
	DefaultRuntimeSeccompProfileEnabled() bool
	RegisterWithFQDN() bool
	NodeIP() KubeletNodeIP
	SkipNodeRegistration() bool
	DisableManifestsDirectory() bool
}

// KubeletNodeIP defines the way node IPs are selected for the kubelet.
type KubeletNodeIP interface {
	ValidSubnets() []string
}

// Registries defines the configuration for image fetching.
type Registries interface {
	// Mirror config by registry host (first part of image reference).
	Mirrors() map[string]RegistryMirrorConfig
	// Registry config (auth, TLS) by hostname.
	Config() map[string]RegistryConfig
}

// RegistryMirrorConfig represents mirror configuration for a registry.
type RegistryMirrorConfig interface {
	Endpoints() []string
	OverridePath() bool
	SkipFallback() bool
}

// RegistryConfig specifies auth & TLS config per registry.
type RegistryConfig interface {
	TLS() RegistryTLSConfig
	Auth() RegistryAuthConfig
}

// RegistryAuthConfig specifies authentication configuration for a registry.
type RegistryAuthConfig interface {
	Username() string
	Password() string
	Auth() string
	IdentityToken() string
}

// RegistryTLSConfig specifies TLS config for HTTPS registries.
type RegistryTLSConfig interface {
	ClientIdentity() *x509.PEMEncodedCertificateAndKey
	CA() []byte
	InsecureSkipVerify() bool
	GetTLSConfig() (*tls.Config, error)
}

// EncryptionKey defines settings for the partition encryption key handling.
type EncryptionKey interface {
	Static() EncryptionKeyStatic
	NodeID() EncryptionKeyNodeID
	KMS() EncryptionKeyKMS
	Slot() int
	TPM() EncryptionKeyTPM
}

// EncryptionKeyStatic ephemeral encryption key.
type EncryptionKeyStatic interface {
	Key() []byte
	String() string
}

// EncryptionKeyKMS encryption key sealed by KMS.
type EncryptionKeyKMS interface {
	Endpoint() string
	String() string
}

// EncryptionKeyNodeID deterministically generated encryption key.
type EncryptionKeyNodeID interface {
	String() string
}

// EncryptionKeyTPM encryption key sealed by TPM.
type EncryptionKeyTPM interface {
	CheckSecurebootOnEnroll() bool
	String() string
}

// Encryption defines settings for the partition encryption.
type Encryption interface {
	Provider() string
	Cipher() string
	KeySize() uint
	BlockSize() uint64
	Options() []string
	Keys() []EncryptionKey
}

// SystemDiskEncryption accumulates settings for all system partitions encryption.
type SystemDiskEncryption interface {
	Get(label string) Encryption
}

// Features describe individual Talos features that can be switched on or off.
type Features interface {
	RBACEnabled() bool
	StableHostnameEnabled() bool
	KubernetesTalosAPIAccess() KubernetesTalosAPIAccess
	ApidCheckExtKeyUsageEnabled() bool
	DiskQuotaSupportEnabled() bool
	HostDNS() HostDNS
	KubePrism() KubePrism
	ImageCache() ImageCache
	NodeAddressSortAlgorithm() nethelpers.AddressSortAlgorithm
}

// KubernetesTalosAPIAccess describes the Kubernetes Talos API access features.
type KubernetesTalosAPIAccess interface {
	Enabled() bool
	AllowedRoles() []string
	AllowedKubernetesNamespaces() []string
}

// KubePrism describes the API Server load balancer features.
type KubePrism interface {
	Enabled() bool
	Port() int
}

// HostDNS describes the host DNS configuration.
type HostDNS interface {
	Enabled() bool
	ForwardKubeDNSToHost() bool
	ResolveMemberNames() bool
}

// ImageCache describes the image cache configuration.
type ImageCache interface {
	LocalEnabled() bool
}

// UdevConfig describes configuration for udev.
type UdevConfig interface {
	Rules() []string
}

// Logging describes logging configuration.
type Logging interface {
	Destinations() []LoggingDestination
}

// LoggingDestination describes logging destination.
type LoggingDestination interface {
	Endpoint() *url.URL
	ExtraTags() map[string]string
	Format() string
}

// Kernel describes Talos Linux kernel configuration.
type Kernel interface {
	Modules() []KernelModule
}

// KernelModule describes Linux module to load.
type KernelModule interface {
	Name() string
	Parameters() []string
}
