// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config

import (
	"time"

	"github.com/siderolabs/crypto/x509"
)

// ClusterConfig defines the requirements for a config that pertains to cluster
// related options.
//
//nolint:interfacebloat
type ClusterConfig interface {
	Token() Token
	AESCBCEncryptionSecret() string
	SecretboxEncryptionSecret() string
	Etcd() Etcd
	// ExternalCloudProvider returns external cloud provider settings.
	ExternalCloudProvider() ExternalCloudProvider
	AdminKubeconfig() AdminKubeconfig
	Discovery() Discovery
}

// DiscoveryIdentityConfig provides the cluster identity (ID and shared secret) used by the
// discovery service and KubeSpan.
type DiscoveryIdentityConfig interface {
	ClusterID() string
	ClusterSecret() string
}

// Etcd defines the requirements for a config that pertains to etcd related
// options.
type Etcd interface {
	Image() string
	CA() *x509.PEMEncodedCertificateAndKey
	ExtraArgs() map[string][]string
	AdvertisedSubnets() []string
	ListenSubnets() []string
}

// Token defines the requirements for a config that pertains to Kubernetes
// bootstrap token.
type Token interface {
	ID() string
	Secret() string
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

// Discovery describes cluster membership discovery.
type Discovery interface {
	Enabled() bool
	Registries() DiscoveryRegistries
}

// DiscoveryRegistries describes discovery methods.
type DiscoveryRegistries interface {
	Kubernetes() KubernetesRegistry
}

// KubernetesRegistry describes Kubernetes discovery registry.
//
//nolint:iface
type KubernetesRegistry interface {
	Enabled() bool
}
