// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package machine

import (
	"crypto/tls"
	stdx509 "crypto/x509"
	"fmt"
	"os"

	specs "github.com/opencontainers/runtime-spec/specs-go"

	"github.com/talos-systems/talos/pkg/crypto/x509"
)

// Type represents a machine type.
type Type int

const (
	// TypeInit represents an init node.
	TypeInit Type = iota
	// TypeControlPlane represents a control plane node.
	TypeControlPlane
	// TypeWorker represents a worker node.
	TypeWorker
)

// String returns the string representation of Type.
func (t Type) String() string {
	return [...]string{"Init", "ControlPlane", "Join"}[t]
}

// ParseType parses string constant as Type
func ParseType(t string) (Type, error) {
	switch t {
	case "Init":
		return TypeInit, nil
	case "ControlPlane":
		return TypeControlPlane, nil
	case "Join":
		return TypeWorker, nil
	default:
		return 0, fmt.Errorf("unknown type %q", t)
	}
}

// Machine defines the requirements for a config that pertains to machine
// related options.
type Machine interface {
	Install() Install
	Security() Security
	Network() Network
	Disks() []Disk
	Time() Time
	Env() Env
	Files() ([]File, error)
	Type() Type
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

// Network defines the requirements for a config that pertains to network
// related options.
type Network interface {
	Hostname() string
	SetHostname(string)
	Resolvers() []string
	Devices() []Device
}

// Device represents a network interface.
type Device struct {
	Interface string  `yaml:"interface"`
	CIDR      string  `yaml:"cidr"`
	Routes    []Route `yaml:"routes"`
	Bond      *Bond   `yaml:"bond"`
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
