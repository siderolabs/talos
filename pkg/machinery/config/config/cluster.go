// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config

import (
	"net/netip"
	"net/url"
	"time"

	"github.com/siderolabs/crypto/x509"
)

// ClusterConfig defines the requirements for a config that pertains to cluster
// related options.
//
//nolint:interfacebloat
type ClusterConfig interface {
	ID() string
	Name() string
	Secret() string
	APIServer() APIServer
	ControllerManager() ControllerManager
	Proxy() Proxy
	Scheduler() Scheduler
	Endpoint() *url.URL
	Token() Token
	CertSANs() []string
	IssuingCA() *x509.PEMEncodedCertificateAndKey
	AcceptedCAs() []*x509.PEMEncodedCertificate
	AggregatorCA() *x509.PEMEncodedCertificateAndKey
	ServiceAccount() *x509.PEMEncodedKey
	AESCBCEncryptionSecret() string
	SecretboxEncryptionSecret() string
	Etcd() Etcd
	Network() ClusterNetwork
	LocalAPIServerPort() int
	CoreDNS() CoreDNS
	// ExternalCloudProvider returns external cloud provider settings.
	ExternalCloudProvider() ExternalCloudProvider
	ExtraManifestURLs() []string
	ExtraManifestHeaderMap() map[string]string
	InlineManifests() []InlineManifest
	AdminKubeconfig() AdminKubeconfig
	ScheduleOnControlPlanes() bool
	Discovery() Discovery
}

// ClusterNetwork defines the requirements for a config that pertains to cluster
// network options.
type ClusterNetwork interface {
	CNI() CNI
	PodCIDRs() []string
	ServiceCIDRs() []string
	DNSDomain() string
	// APIServerIPs returns kube-apiserver IPs in the ServiceCIDR.
	APIServerIPs() ([]netip.Addr, error)
	// DNSServiceIPs returns DNS service IPs in the ServiceCIDR.
	DNSServiceIPs() ([]netip.Addr, error)
}

// CNI defines the requirements for a config that pertains to Kubernetes
// cni.
type CNI interface {
	Name() string
	URLs() []string
	Flannel() FlannelCNI
}

// FlannelCNI defines the requirements for a config that pertains to configure Flannel.
type FlannelCNI interface {
	ExtraArgs() []string
}

// APIServer defines the requirements for a config that pertains to apiserver related
// options.
type APIServer interface {
	Image() string
	ExtraArgs() map[string]string
	ExtraVolumes() []VolumeMount
	Env() Env
	DisablePodSecurityPolicy() bool
	AdmissionControl() []AdmissionPlugin
	AuditPolicy() map[string]interface{}
	Resources() Resources
}

// AdmissionPlugin defines the API server Admission Plugin configuration.
type AdmissionPlugin interface {
	Name() string
	Configuration() map[string]interface{}
}

// ControllerManager defines the requirements for a config that pertains to controller manager related
// options.
type ControllerManager interface {
	Image() string
	ExtraArgs() map[string]string
	ExtraVolumes() []VolumeMount
	Env() Env
	Resources() Resources
}

// Proxy defines the requirements for a config that pertains to the kube-proxy
// options.
type Proxy interface {
	Enabled() bool

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
	ExtraVolumes() []VolumeMount
	Env() Env
	Resources() Resources
	Config() map[string]any
}

// Etcd defines the requirements for a config that pertains to etcd related
// options.
type Etcd interface {
	Image() string
	CA() *x509.PEMEncodedCertificateAndKey
	ExtraArgs() map[string]string
	AdvertisedSubnets() []string
	ListenSubnets() []string
}

// Token defines the requirements for a config that pertains to Kubernetes
// bootstrap token.
type Token interface {
	ID() string
	Secret() string
}

// CoreDNS defines the requirements for a config that pertains to CoreDNS
// coredns options.
type CoreDNS interface {
	Enabled() bool
	Image() string
}

// ExternalCloudProvider defines settings for external cloud provider.
type ExternalCloudProvider interface {
	// Enabled returns true if external cloud provider is enabled.
	Enabled() bool
	// ManifestURLs returns external cloud provider manifest URLs if it is enabled.
	ManifestURLs() []string
}

// AdminKubeconfig defines settings for admin kubeconfig.
type AdminKubeconfig interface {
	CommonName() string
	CertOrganization() string
	CertLifetime() time.Duration
}

// VolumeMount describes extra volume mount for the static pods.
type VolumeMount interface {
	Name() string
	HostPath() string
	MountPath() string
	ReadOnly() bool
}

// Resources describes memory/cpu requests/limits for static pods.
type Resources interface {
	CPURequests() string
	MemoryRequests() string
	CPULimits() string
	MemoryLimits() string
}

// InlineManifest describes inline manifest for the cluster boostrap.
type InlineManifest interface {
	Name() string
	Contents() string
}

// Discovery describes cluster membership discovery.
type Discovery interface {
	Enabled() bool
	Registries() DiscoveryRegistries
}

// DiscoveryRegistries describes discovery methods.
type DiscoveryRegistries interface {
	Kubernetes() KubernetesRegistry
	Service() ServiceRegistry
}

// KubernetesRegistry describes Kubernetes discovery registry.
type KubernetesRegistry interface {
	Enabled() bool
}

// ServiceRegistry describes external service discovery registry.
type ServiceRegistry interface {
	Enabled() bool
	Endpoint() string
}
