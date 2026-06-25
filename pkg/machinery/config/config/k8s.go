// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config

import (
	"net/netip"

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
