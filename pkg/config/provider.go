// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config

import (
	"net/url"
	"time"

	"github.com/opencontainers/runtime-spec/specs-go"

	"github.com/talos-systems/talos/pkg/config/types/v1alpha1/machine"
	"github.com/talos-systems/talos/pkg/crypto/x509"
)

// Provider defines the configuration consumption interface.
type Provider interface {
	Version() string
	Debug() bool
	Persist() bool
	Machine() MachineConfig
	Cluster() ClusterConfig
	Validate(RuntimeMode) error
	String() (string, error)
	Bytes() ([]byte, error)
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
	Type() machine.Type
	Kubelet() Kubelet
	Sysctls() map[string]string
	Registries() Registries
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

// ClusterConfig defines the requirements for a config that pertains to cluster
// related options.
type ClusterConfig interface {
	Name() string
	APIServer() APIServer
	ControllerManager() ControllerManager
	Proxy() Proxy
	Scheduler() Scheduler
	Endpoint() *url.URL
	Token() Token
	CertSANs() []string
	SetCertSANs([]string)
	CA() *x509.PEMEncodedCertificateAndKey
	AESCBCEncryptionSecret() string
	Config(machine.Type) (string, error)
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
	Image() string
	ExtraArgs() map[string]string
}

// ControllerManager defines the requirements for a config that pertains to controller manager related
// options.
type ControllerManager interface {
	Image() string
	ExtraArgs() map[string]string
}

// Proxy defines the requirements for a config that pertains to the kube-proxy
// options.
type Proxy interface {
	Image() string

	// Mode indicates the proxy mode for kube-proxy.  By default, this is `iptables`.  Other options include `ipvs`.
	Mode() string

	// ExtraArgs describe an additional set of arguments to be supplied to the execution of `kube-proxy`
	ExtraArgs() map[string]string
}

// Scheduler defines the requirements for a config that pertains to scheduler related
// options.
type Scheduler interface {
	Image() string
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
