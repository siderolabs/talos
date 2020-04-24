// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"crypto/tls"
	"fmt"
	"net/url"
	"os"
	"time"

	stdx509 "crypto/x509"

	"github.com/opencontainers/runtime-spec/specs-go"

	"github.com/talos-systems/talos/pkg/blockdevice/probe"
	"github.com/talos-systems/talos/pkg/client/config"
	"github.com/talos-systems/talos/pkg/crypto/x509"
)

// Configurator defines the configuration interface.
type Configurator interface {
	Version() string
	Debug() bool
	Persist() bool
	Machine() MachineConfig
	Cluster() ClusterConfig
	Validate(Mode) error
	String() (string, error)
	Bytes() ([]byte, error)
}

// ConfiguratorBundle defines the configuration bundle interface.
type ConfiguratorBundle interface {
	Init() Configurator
	ControlPlane() Configurator
	Join() Configurator
	TalosConfig() *config.Config
}

// Machine defines the runtime parameters.
type Machine interface {
	State() MachineState
	Config() MachineConfig
}

// MachineState defines the machined state.
type MachineState interface {
	Disk() *probe.ProbedBlockDevice
	Close() error
	Installed() bool
}

// MachineType represents a machine type.
type MachineType int

const (
	// MachineTypeInit represents a bootstrap node.
	MachineTypeInit MachineType = iota
	// MachineTypeControlPlane represents a control plane node.
	MachineTypeControlPlane
	// MachineTypeJoin represents a worker node.
	MachineTypeJoin
)

const (
	machineTypeInit         = "init"
	machineTypeControlPlane = "controlplane"
	machineTypeJoin         = "join"
)

// String returns the string representation of Type.
func (t MachineType) String() string {
	return [...]string{machineTypeInit, machineTypeControlPlane, machineTypeJoin}[t]
}

// ParseMachineType parses string constant as Type
func ParseMachineType(t string) (MachineType, error) {
	switch t {
	case machineTypeInit:
		return MachineTypeInit, nil
	case machineTypeControlPlane:
		return MachineTypeControlPlane, nil
	case machineTypeJoin:
		return MachineTypeJoin, nil
	default:
		return 0, fmt.Errorf("unknown machine type: %q", t)
	}
}

// MachineConfig defines the requirements for a config that pertains to machine
// related options.
type MachineConfig interface {
	Install() Install
	Security() Security
	Network() MachineNetwork
	Disks() []Disk
	Time() Time
	Env() Env
	Files() ([]File, error)
	Type() MachineType
	Kubelet() Kubelet
	Sysctls() map[string]string
	Registries() Registries
}

// Env represents a set of environment variables.
type Env = map[string]string

// File represents a file to write to disk.
type File struct {
	Content     string      `yaml:"content"`
	Permissions os.FileMode `yaml:"permissions"`
	Path        string      `yaml:"path"`
	Op          string      `yaml:"op"`
}

// Security defines the requirements for a config that pertains to security
// related options.
type Security interface {
	CA() *x509.PEMEncodedCertificateAndKey
	Token() string
	CertSANs() []string
	SetCertSANs([]string)
}

// MachineNetwork defines the requirements for a config that pertains to network
// related options.
type MachineNetwork interface {
	Hostname() string
	SetHostname(string)
	Resolvers() []string
	Devices() []Device
	ExtraHosts() []ExtraHost
}

// ExtraHost represents a host entry in /etc/hosts
type ExtraHost struct {
	IP      string   `yaml:"ip"`
	Aliases []string `yaml:"aliases"`
}

// Device represents a network interface.
type Device struct {
	Interface string  `yaml:"interface"`
	CIDR      string  `yaml:"cidr"`
	Routes    []Route `yaml:"routes"`
	Bond      *Bond   `yaml:"bond"`
	Vlans     []*Vlan `yaml:"vlans"`
	MTU       int     `yaml:"mtu"`
	DHCP      bool    `yaml:"dhcp"`
	Ignore    bool    `yaml:"ignore"`
}

// Bond contains the various options for configuring a
// bonded interface.
type Bond struct {
	Interfaces      []string `yaml:"interfaces"`
	ARPIPTarget     []string `yaml:"arpIPTarget"`
	Mode            string   `yaml:"mode"`
	HashPolicy      string   `yaml:"xmitHashPolicy"`
	LACPRate        string   `yaml:"lacpRate"`
	ADActorSystem   string   `yaml:"adActorSystem"`
	ARPValidate     string   `yaml:"arpValidate"`
	ARPAllTargets   string   `yaml:"arpAllTargets"`
	Primary         string   `yaml:"primary"`
	PrimaryReselect string   `yaml:"primaryReselect"`
	FailOverMac     string   `yaml:"failOverMac"`
	ADSelect        string   `yaml:"adSelect"`
	MIIMon          uint32   `yaml:"miimon"`
	UpDelay         uint32   `yaml:"updelay"`
	DownDelay       uint32   `yaml:"downdelay"`
	ARPInterval     uint32   `yaml:"arpInterval"`
	ResendIGMP      uint32   `yaml:"resendIgmp"`
	MinLinks        uint32   `yaml:"minLinks"`
	LPInterval      uint32   `yaml:"lpInterval"`
	PacketsPerSlave uint32   `yaml:"packetsPerSlave"`
	NumPeerNotif    uint8    `yaml:"numPeerNotif"`
	TLBDynamicLB    uint8    `yaml:"tlbDynamicLb"`
	AllSlavesActive uint8    `yaml:"allSlavesActive"`
	UseCarrier      bool     `yaml:"useCarrier"`
	ADActorSysPrio  uint16   `yaml:"adActorSysPrio"`
	ADUserPortKey   uint16   `yaml:"adUserPortKey"`
	PeerNotifyDelay uint32   `yaml:"peerNotifyDelay"`
}

// Vlan represents vlan settings for a device
type Vlan struct {
	CIDR   string  `ỳaml:"cidr"`
	Routes []Route `yaml:"routes"`
	DHCP   bool    `yaml:"dhcp"`
	ID     uint16  `yaml:"vlanId"`
}

// Route represents a network route.
type Route struct {
	Network string `yaml:"network"`
	Gateway string `yaml:"gateway"`
}

// Install defines the requirements for a config that pertains to install
// related options.
type Install interface {
	Image() string
	Disk() string
	ExtraKernelArgs() []string
	Zero() bool
	Force() bool
	WithBootloader() bool
}

// Disk represents the options available for partitioning, formatting, and
// mounting extra disks.
type Disk struct {
	Device     string      `yaml:"device,omitempty"`
	Partitions []Partition `yaml:"partitions,omitempty"`
}

// Partition represents the options for a device partition.
type Partition struct {
	Size       uint   `yaml:"size,omitempty"`
	MountPoint string `yaml:"mountpoint,omitempty"`
}

// Time defines the requirements for a config that pertains to time related
// options.
type Time interface {
	Servers() []string
}

// Kubelet defines the requirements for a config that pertains to kubelet
// related options.
type Kubelet interface {
	Image() string
	ExtraArgs() map[string]string
	ExtraMounts() []specs.Mount
}

// RegistryMirrorConfig represents mirror configuration for a registry.
type RegistryMirrorConfig struct {
	//   description: |
	//     List of endpoints (URLs) for registry mirrors to use.
	//     Endpoint configures HTTP/HTTPS access mode, host name,
	//     port and path (if path is not set, it defaults to `/v2`).
	Endpoints []string `yaml:"endpoints"`
}

// RegistryConfig specifies auth & TLS config per registry.
type RegistryConfig struct {
	TLS  *RegistryTLSConfig  `yaml:"tls,omitempty"`
	Auth *RegistryAuthConfig `yaml:"auth,omitempty"`
}

// RegistryAuthConfig specifies authentication configuration for a registry.
type RegistryAuthConfig struct {
	//   description: |
	//     Optional registry authentication.
	//     The meaning of each field is the same with the corresponding field in .docker/config.json.
	Username string `yaml:"username"`
	//   description: |
	//     Optional registry authentication.
	//     The meaning of each field is the same with the corresponding field in .docker/config.json.
	Password string `yaml:"password"`
	//   description: |
	//     Optional registry authentication.
	//     The meaning of each field is the same with the corresponding field in .docker/config.json.
	Auth string `yaml:"auth"`
	//   description: |
	//     Optional registry authentication.
	//     The meaning of each field is the same with the corresponding field in .docker/config.json.
	IdentityToken string `yaml:"identityToken"`
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
	ClientIdentity *x509.PEMEncodedCertificateAndKey `yaml:"clientIdentity,omitempty"`
	//   description: |
	//     CA registry certificate to add the list of trusted certificates.
	//     Certificate should be base64-encoded.
	CA []byte `yaml:"ca,omitempty"`
	//   description: |
	//     Skip TLS server certificate verification (not recommended).
	InsecureSkipVerify bool `yaml:"insecureSkipVerify,omitempty"`
}

// GetTLSConfig prepares TLS configuration for connection.
func (cfg *RegistryTLSConfig) GetTLSConfig() (*tls.Config, error) {
	tlsConfig := &tls.Config{}

	if cfg.ClientIdentity != nil {
		cert, err := tls.X509KeyPair(cfg.ClientIdentity.Crt, cfg.ClientIdentity.Key)
		if err != nil {
			return nil, fmt.Errorf("error parsing client identity: %w", err)
		}

		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	if cfg.CA != nil {
		tlsConfig.RootCAs = stdx509.NewCertPool()
		tlsConfig.RootCAs.AppendCertsFromPEM(cfg.CA)
	}

	if cfg.InsecureSkipVerify {
		tlsConfig.InsecureSkipVerify = true
	}

	return tlsConfig, nil
}

// Registries defines the configuration for image fetching.
type Registries interface {
	// Mirror config by registry host (first part of image reference).
	Mirrors() map[string]RegistryMirrorConfig
	// Registry config (auth, TLS) by hostname.
	Config() map[string]RegistryConfig
	// ExtraFiles generates TOML config for containerd CRI plugin.
	ExtraFiles() ([]File, error)
}

// ClusterState defines the cluster state.
type ClusterState interface{}

// ClusterConfig defines the requirements for a config that pertains to cluster
// related options.
type ClusterConfig interface {
	Name() string
	APIServer() APIServer
	ControllerManager() ControllerManager
	Scheduler() Scheduler
	Endpoint() *url.URL
	Token() Token
	CertSANs() []string
	SetCertSANs([]string)
	CA() *x509.PEMEncodedCertificateAndKey
	AESCBCEncryptionSecret() string
	Config(MachineType) (string, error)
	Etcd() Etcd
	Network() ClusterNetwork
	LocalAPIServerPort() int
	PodCheckpointer() PodCheckpointer
	CoreDNS() CoreDNS
	ExtraManifestURLs() []string
	ExtraManifestHeaderMap() map[string]string
	AdminKubeconfig() AdminKubeconfig
}

// ClusterNetwork defines the requirements for a config that pertains to cluster
// network options.
type ClusterNetwork interface {
	CNI() CNI
	PodCIDR() string
	ServiceCIDR() string
	DNSDomain() string
}

// CNI defines the requirements for a config that pertains to Kubernetes
// cni.
type CNI interface {
	Name() string
	URLs() []string
}

// APIServer defines the requirements for a config that pertains to apiserver related
// options.
type APIServer interface {
	ExtraArgs() map[string]string
}

// ControllerManager defines the requirements for a config that pertains to controller manager related
// options.
type ControllerManager interface {
	ExtraArgs() map[string]string
}

// Scheduler defines the requirements for a config that pertains to scheduler related
// options.
type Scheduler interface {
	ExtraArgs() map[string]string
}

// Etcd defines the requirements for a config that pertains to etcd related
// options.
type Etcd interface {
	Image() string
	CA() *x509.PEMEncodedCertificateAndKey
	ExtraArgs() map[string]string
}

// Token defines the requirements for a config that pertains to Kubernetes
// bootstrap token.
type Token interface {
	ID() string
	Secret() string
}

// PodCheckpointer defines the requirements for a config that pertains to bootkube
// pod-checkpointer options.
type PodCheckpointer interface {
	Image() string
}

// CoreDNS defines the requirements for a config that pertains to bootkube
// coredns options.
type CoreDNS interface {
	Image() string
}

// AdminKubeconfig defines settings for admin kubeconfig.
type AdminKubeconfig interface {
	CertLifetime() time.Duration
}
