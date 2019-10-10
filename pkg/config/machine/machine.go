/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

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
	Time() Time
	Env() Env
	Files() []File
	Type() Type
	Kubelet() Kubelet
}

// Env represents a set of environment variables.
type Env = map[string]string

// File represents a file to write to disk.
type File struct {
	Contents    string      `yaml:"contents"`
	Permissions os.FileMode `yaml:"permissions"`
	Path        string      `yaml:"path"`
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
	Mode       string   `yaml:"mode"`
	HashPolicy string   `yaml:"hashpolicy"`
	LACPRate   string   `yaml:"lacprate"`
	Interfaces []string `yaml:"interfaces"`
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
	ExtraDisks() []Disk
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
	Server() string
}

// Kubelet defines the requirements for a config that pertains to kubelet
// related options.
type Kubelet interface {
	ExtraMounts() []specs.Mount
}
