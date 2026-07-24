// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config

import (
	"net/netip"
	"net/url"

	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/siderolabs/crypto/x509"
	"github.com/siderolabs/gen/optional"
)

// K8sAPIServerConfig defines kube-apiserver configuration options.
//
//nolint:interfacebloat
type K8sAPIServerConfig interface {
	K8sAPIServerConfigSignal()
	Image() string
	ExtraArgs() map[string][]string
	ExtraVolumes() []VolumeMount
	Env() Env
	Resources() Resources
	StartupProbesEnabled() bool
	UseAuthenticationConfig() bool
	InjectDefaultAuthorizers() bool
	CertSANs() []string
	APIPort() int
}

// K8sAPIServerCAConfig defines kube-apiserver CA configuration options.
type K8sAPIServerCAConfig interface {
	K8sAPIServerCAConfigSignal()
	// IssuingCA returns the key pair used to issue certificates for the kube-apiserver.
	//
	// This method only returns non-nil value on the controlplane.
	IssuingCA() *x509.PEMEncodedCertificateAndKey
	// AcceptedCAs returns the list of CA certificates that the kube-apiserver trusts.
	//
	// If the IssuingCA is not nil, the returned list will include the issuing CA as the first element.
	AcceptedCAs() []*x509.PEMEncodedCertificate
}

// K8sAggregatorCAConfig defines kube-apiserver aggregator CA configuration.
type K8sAggregatorCAConfig interface {
	K8sAggregatorCAConfigSignal()
	// IssuingCA returns the key pair used to issue certificates for the kube-apiserver.
	//
	// This method only returns non-nil value on the controlplane.
	IssuingCA() *x509.PEMEncodedCertificateAndKey
	// AcceptedCAs returns the list of CA certificates that the kube-apiserver trusts.
	//
	// If the IssuingCA is not nil, the returned list will include the issuing CA as the first element.
	AcceptedCAs() []*x509.PEMEncodedCertificate
}

// K8sControllerManagerConfig defines configuration options for the kube-controller-manager static pod.
type K8sControllerManagerConfig interface {
	K8sControllerManagerConfigSignal()
	Enabled() bool
	Image() string
	ExtraArgs() map[string][]string
	ExtraVolumes() []VolumeMount
	Env() Env
	Resources() Resources
}

// K8sSchedulerConfig defines configuration options for the kube-scheduler static pod.
type K8sSchedulerConfig interface {
	K8sSchedulerConfigSignal()
	Enabled() bool
	Image() string
	ExtraArgs() map[string][]string
	ExtraVolumes() []VolumeMount
	Env() Env
	Resources() Resources
	Config() map[string]any
}

// K8sProxyConfig defines the configuration options for the kube-proxy.
type K8sProxyConfig interface {
	K8sProxyConfigSignal()
	Enabled() bool
	Image() string
	Mode() string
	ExtraArgs() map[string][]string
	Resources() Resources
	Config() map[string]any
	UseConfigFile() bool
}

// K8sEtcdEncryptionConfig defines the interface to access Kubernetes API server encryption of secret data at rest configuration.
type K8sEtcdEncryptionConfig interface {
	// EtcdEncryptionConfig returns the exact contents of the configuration file, excluding the apiVersion and kind fields.
	EtcdEncryptionConfig() map[string]any
}

// K8sNetworkConfig defines Kubernetes network configuration options.
type K8sNetworkConfig interface {
	PodCIDRs() []netip.Prefix
	ServiceCIDRs() []netip.Prefix
	DNSDomain() string
	NodeCIDRMaskSizeIPv4() int
	NodeCIDRMaskSizeIPv6() int
}

// K8sFlannelCNIConfig defines the configuration options for the Flannel CNI in Kubernetes.
type K8sFlannelCNIConfig interface {
	BackendType() string
	BackendPort() optional.Optional[uint16]
	BackendMTU() optional.Optional[uint32]
	BackendExtraConfig() map[string]any
	Resources() Resources
	ExtraArgs() []string
	KubeNetworkPoliciesEnabled() bool
}

// K8sAdmissionControlPluginConfig defines the configuration options for kube-apiserver admission control plugins.
type K8sAdmissionControlPluginConfig interface {
	K8sAdmissionControlPluginConfigSignal()
	NamedDocument
	Configuration() map[string]any
}

// K8sAuditPolicyConfig defines the configuration options for kube-apiserver audit policy.
type K8sAuditPolicyConfig interface {
	K8sAuditPolicyConfigSignal()
	Configuration() map[string]any
}

// K8sAuthenticationConfig defines the configuration options for kube-apiserver authentication.
type K8sAuthenticationConfig interface {
	K8sAuthenticationConfigSignal()
	Configuration() map[string]any
}

// K8sAuthorizerConfig defines the API server authorization Authorizer configuration.
type K8sAuthorizerConfig interface {
	K8sAuthorizerConfigSignal()
	Type() string
	Name() string
	Webhook() map[string]any
}

// K8sCoreDNSConfig defines the configuration options for CoreDNS.
type K8sCoreDNSConfig interface {
	K8sCoreDNSConfigSignal()
	Enabled() bool
	Image() string
}

// K8sServiceAccountConfig defines the configuration options for Kubernetes service accounts.
type K8sServiceAccountConfig interface {
	K8sServiceAccountConfigSignal()
	IssuingKey() *x509.PEMEncodedKey
	AcceptedKeys() []*x509.PEMEncodedKey
	IssuerURL() string
	AcceptedIssuers() []string
	APIAudiences() []string
}

// K8sClusterConfig defines cluster-wide configuration options for Kubernetes.
type K8sClusterConfig interface {
	ClusterName() string
	ClusterEndpoint() *url.URL
}

// K8sNodeIPConfig defines the way node IPs are selected for the kubelet.
type K8sNodeIPConfig interface {
	ValidSubnets() []string
}

// K8sNodeConfig defines configuration options for the Kubernetes node.
type K8sNodeConfig interface {
	SkipNodeRegistration() bool
	RegisterWithFQDN() bool
	NodeIP() K8sNodeIPConfig
	Labels() map[string]string
	Taints() map[string]string
	Annotations() map[string]string
}

// K8sKubeletConfig defines the kubelet configuration options for the Kubernetes node.
type K8sKubeletConfig interface {
	K8sKubeletConfigSignal()
	Image() string
	ClusterDNS() []string
	ExtraArgs() map[string][]string
	ExtraMounts() []specs.Mount
	ExtraConfig() map[string]any
	DefaultRuntimeSeccompProfileEnabled() bool
	DisableManifestsDirectory() bool
}

// K8sCredentialProviderConfig defines the configuration options for the Kubernetes credential provider.
type K8sCredentialProviderConfig interface {
	K8sCredentialProviderConfigSignal()
	Configuration() map[string]any
}

// K8sStaticPodConfig defines the configuration options for a Kubernetes static pod.
type K8sStaticPodConfig interface {
	K8sStaticPodConfigSignal()
	NamedDocument
	Pod() map[string]any
}

// K8sInlineManifestConfig describes inline manifest for the cluster bootstrap.
type K8sInlineManifestConfig interface {
	K8sInlineManifestConfigSignal()
	NamedDocument
	Contents() string
}

// K8sExternalManifestConfig describes external manifest for the cluster bootstrap.
type K8sExternalManifestConfig interface {
	K8sExternalManifestConfigSignal()
	NamedDocument
	Headers() map[string]string
	URL() string
}

// K8sKubePrismConfig describes the API Server load balancer features.
type K8sKubePrismConfig interface {
	K8sKubePrismConfigSignal()
	Port() int
	TLSServerName() string
}

// K8sTalosAPIAccessConfig describes the Kubernetes Talos API access features.
type K8sTalosAPIAccessConfig interface {
	AllowedRoles() []string
	AllowedKubernetesNamespaces() []string
}
