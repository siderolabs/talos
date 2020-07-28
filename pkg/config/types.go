// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config

import (
	"crypto/tls"
	stdx509 "crypto/x509"
	"fmt"
	"os"

	"github.com/talos-systems/talos/pkg/crypto/x509"
)

// Env represents a set of environment variables.
type Env = map[string]string

// File represents a file to write to disk.
type File struct {
	Content     string      `yaml:"content"`
	Permissions os.FileMode `yaml:"permissions"`
	Path        string      `yaml:"path"`
	Op          string      `yaml:"op"`
}

// ExtraHost represents a host entry in /etc/hosts.
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
	Dummy     bool    `yaml:"dummy"`
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

// Vlan represents vlan settings for a device.
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
