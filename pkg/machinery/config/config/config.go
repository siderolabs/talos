// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package config provides interfaces to consume machine configuration values.
package config

// Config defines the interface to access contents of the machine configuration.
type Config interface { //nolint:interfacebloat
	// old v1alpha1 interface (to be decomposed as we move to multi-doc)
	Debug() bool
	Machine() MachineConfig
	Cluster() ClusterConfig

	// new multi-doc interfaces:
	//  - network
	SideroLink() SideroLinkConfig
	NetworkRules() NetworkRuleConfig
	KubespanConfig() KubespanConfig
	EthernetConfigs() []EthernetConfig
	RunDefaultDHCPOperators() bool
	NetworkStaticHostConfig() []NetworkStaticHostConfig
	NetworkHostnameConfig() NetworkHostnameConfig
	NetworkResolverConfig() NetworkResolverConfig
	NetworkHostDNSConfig() NetworkHostDNSConfig
	NetworkTimeSyncConfig() NetworkTimeSyncConfig
	NetworkKubeSpanConfig() NetworkKubeSpanConfig
	NetworkCommonLinkConfigs() []NetworkCommonLinkConfig
	NetworkLinkAliasConfigs() []NetworkLinkAliasConfig
	NetworkDHCPConfigs() []NetworkDHCPConfig
	NetworkDHCPv4Configs() []NetworkDHCPv4Config
	NetworkDHCPv6Configs() []NetworkDHCPv6Config
	NetworkVirtualIPConfigs() []NetworkVirtualIPConfig
	NetworkProbeConfigs() []NetworkCommonProbeConfig
	NetworkBlackholeRouteConfigs() []NetworkBlackholeRouteConfig
	NetworkRoutingRuleConfigs() []NetworkRoutingRuleConfig

	// - cluster
	DiscoveryServiceConfigs() []DiscoveryServiceConfig
	DiscoveryIdentityConfig() DiscoveryIdentityConfig

	// - k8s:
	K8sAdmissionControlPluginConfigs() []K8sAdmissionControlPluginConfig
	K8sAuditPolicyConfig() K8sAuditPolicyConfig
	K8sAuthenticationConfig() K8sAuthenticationConfig
	K8sAuthorizerConfigs() []K8sAuthorizerConfig
	K8sEtcdEncryptionConfig() K8sEtcdEncryptionConfig
	K8sAPIServerConfig() K8sAPIServerConfig
	K8sControllerManagerConfig() K8sControllerManagerConfig
	K8sSchedulerConfig() K8sSchedulerConfig
	K8sProxyConfig() K8sProxyConfig
	K8sNetworkConfig() K8sNetworkConfig
	K8sFlannelCNIConfig() K8sFlannelCNIConfig
	K8sCoreDNSConfig() K8sCoreDNSConfig

	// - block devices/storage:
	Volumes() VolumesConfig
	UserVolumeConfigs() []UserVolumeConfig
	RawVolumeConfigs() []RawVolumeConfig
	ExistingVolumeConfigs() []ExistingVolumeConfig
	ExternalVolumeConfigs() []ExternalVolumeConfig
	SwapVolumeConfigs() []SwapVolumeConfig
	ZswapConfig() ZswapConfig
	FilesystemTrimConfig() FilesystemTrimConfig
	LVMVolumeGroupConfigs() []LVMVolumeGroupConfig
	LVMLogicalVolumeConfigs() []LVMLogicalVolumeConfig
	RAIDArrayConfigs() []RAIDArrayConfig

	// - cri:
	RegistryMirrorConfigs() map[string]RegistryMirrorConfig
	RegistryAuthConfigs() map[string]RegistryAuthConfig
	RegistryTLSConfigs() map[string]RegistryTLSConfig
	ImageCacheConfig() ImageCacheConfig

	// - misc:
	ExtensionServiceConfigs() []ExtensionServiceConfig
	Runtime() RuntimeConfig
	Environment() EnvironmentConfig
	EtcFileConfigs() []EtcFileConfig
	UdevRulesConfig() UdevConfig
	TrustedRoots() TrustedRootsConfig
	PCIDriverRebindConfig() PCIDriverRebindConfig
	OOMConfig() OOMConfig
	ImageVerificationConfig() ImageVerificationConfig
	SysctlConfig() map[string]string
	SysfsConfig() map[string]string
	UnattendedInstallConfig() UnattendedInstallConfig
}
