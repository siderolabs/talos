// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package machine

import (
	"os"

	specs "github.com/opencontainers/runtime-spec/specs-go"

	"github.com/talos-systems/talos/pkg/crypto/x509"
)

// Type represents the node type.
type Type int

const (
	// Bootstrap represents a bootstrap node.
	Bootstrap Type = iota
	// ControlPlane represents a control plane node.
	ControlPlane
	// Worker represents a worker node.
	Worker
)

// Machine defines the requirements for a config that pertains to machine
// related options.
type Machine interface {
	Install() Install
	Security() Security
	Network() Network
	Disks() []Disk
	Time() Time
	Env() Env
	Files() []File
	Type() Type
	Kubelet() Kubelet
	Sysctls() map[string]string
}

// Env represents a set of environment variables.
type Env = map[string]string

// File represents a file to write to disk.
type File struct {
	Contents    string      `yaml:"contents"`
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
	ExtraArgs() map[string]string
	ExtraMounts() []specs.Mount
}
