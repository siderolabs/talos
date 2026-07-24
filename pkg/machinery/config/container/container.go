// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package container implements a wrapper which wraps all configuration documents into a single container.
package container

import (
	"bytes"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/siderolabs/gen/xslices"

	coreconfig "github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
)

// V1Alpha1ConflictValidator is the interface implemented by config documents which conflict with legacy v1alpha1 config.
type V1Alpha1ConflictValidator interface {
	V1Alpha1ConflictValidate(*v1alpha1.Config) error
}

// ControlplaneOnlyConfig is the interface implemented by config documents which are only applicable to controlplane nodes.
//
// Such documents will not be allowed for machines which do not have a machine type, or the machine type is not controlplane/init.
type ControlplaneOnlyConfig interface {
	config.Document
	ControlplaneOnlyDocument()
}

// Container wraps all configuration documents into a single container.
type Container struct {
	v1alpha1Config *v1alpha1.Config
	documents      []config.Document
	bytes          []byte
	readonly       bool
}

var _ coreconfig.Provider = &Container{}

// New creates a container out of the list of documents.
//
//nolint:gocyclo
func New(documents ...config.Document) (*Container, error) {
	container := &Container{
		documents: make([]config.Document, 0, len(documents)),
	}

	seenDocuments := make(map[string]struct{})
	conflictingDocuments := make(map[string]string)
	claimedNetworkLinks := make(map[string]string)

	for _, doc := range documents {
		switch d := doc.(type) {
		case *v1alpha1.Config:
			if container.v1alpha1Config != nil {
				return nil, errors.New("duplicate v1alpha1.Config")
			}

			container.v1alpha1Config = d
		default:
			if _, ok := d.(selector); !ok {
				documentID := d.Kind() + "/"

				if named, ok := d.(config.NamedDocument); ok {
					documentID += named.Name()
				}

				if _, alreadySeen := seenDocuments[documentID]; alreadySeen {
					return nil, fmt.Errorf("duplicate document: %s", documentID)
				}

				seenDocuments[documentID] = struct{}{}

				if conflictingID, isConflicting := conflictingDocuments[documentID]; isConflicting {
					return nil, fmt.Errorf("conflicting documents: %s and %s", conflictingID, documentID)
				}

				if conflicting, ok := d.(config.ConflictingDocument); ok {
					for _, kind := range conflicting.ConflictsWithKinds() {
						conflictingID := kind + "/"

						if named, ok := d.(config.NamedDocument); ok {
							conflictingID += named.Name()
						}

						if _, alreadySeen := seenDocuments[conflictingID]; alreadySeen {
							return nil, fmt.Errorf("conflicting documents: %s and %s", conflictingID, documentID)
						}

						conflictingDocuments[conflictingID] = documentID
					}
				}

				if linkConfig, ok := d.(config.NetworkCommonLinkConfig); ok {
					linkConfigs := []config.NetworkCommonLinkConfig{linkConfig}

					if additional, ok := d.(config.NetworkAdditionalLinkConfigs); ok {
						linkConfigs = append(linkConfigs, additional.AdditionalLinkConfigs()...)
					}

					for _, claimedLinkConfig := range linkConfigs {
						linkName := claimedLinkConfig.Name()

						if owner, exists := claimedNetworkLinks[linkName]; exists {
							return nil, fmt.Errorf(
								"conflicting link configurations: %s and %s both configure %q",
								owner,
								documentID,
								linkName,
							)
						}

						claimedNetworkLinks[linkName] = documentID
					}
				}
			}

			container.documents = append(container.documents, d)
		}
	}

	return container, nil
}

// NewReadonly creates a read-only container which preserves byte representation of contents.
func NewReadonly(bytes []byte, documents ...config.Document) (*Container, error) {
	c, err := New(documents...)
	if err != nil {
		return nil, err
	}

	c.bytes = bytes
	c.readonly = true

	return c, nil
}

// NewReadonlyUnvalidated creates a read-only container which does not validate the documents at all.
//
// Some methods of the provider don't work at all for such containers.
// This method is meant to be used only for loading config patches.
func NewReadonlyUnvalidated(bytes []byte, documents ...config.Document) *Container {
	return &Container{
		documents: slices.Clone(documents),
		bytes:     bytes,
		readonly:  true,
	}
}

// NewV1Alpha1 creates a container with (only) v1alpha1.Config document.
func NewV1Alpha1(config *v1alpha1.Config) *Container {
	return &Container{
		v1alpha1Config: config,
	}
}

// Clone the container.
//
// Cloned container is not readonly.
func (container *Container) Clone() coreconfig.Provider { return container.clone() }

func (container *Container) clone() *Container {
	return &Container{
		v1alpha1Config: container.v1alpha1Config.DeepCopy(),
		documents:      xslices.Map(container.documents, config.Document.Clone),
	}
}

// PatchV1Alpha1 patches the container's v1alpha1.Config while preserving other config documents.
func (container *Container) PatchV1Alpha1(patcher func(*v1alpha1.Config) error) (coreconfig.Provider, error) {
	cfg := container.RawV1Alpha1()
	if cfg == nil {
		return nil, fmt.Errorf("v1alpha1.Config is not present in the container")
	}

	return PatchDocument(container, func(c *v1alpha1.Config) error {
		return patcher(c)
	})
}

// Has checks if the container has a document of the given kind.
//
// This method only works for new multi-doc config documents, and does not check for v1alpha1.Config.
func (container *Container) Has(kind string) bool {
	return slices.ContainsFunc(container.documents, func(d config.Document) bool {
		if _, ok := d.(selector); ok {
			return false
		}

		return d.Kind() == kind
	})
}

// Readonly implements config.Container interface.
func (container *Container) Readonly() bool {
	return container.readonly
}

// Debug implements config.Config interface.
func (container *Container) Debug() bool {
	if container.v1alpha1Config == nil {
		return false
	}

	return container.v1alpha1Config.Debug()
}

// Machine implements config.Config interface.
func (container *Container) Machine() config.MachineConfig {
	if container.v1alpha1Config == nil {
		return nil
	}

	return container.v1alpha1Config.Machine()
}

// Cluster implements config.Config interface.
func (container *Container) Cluster() config.ClusterConfig {
	if container.v1alpha1Config == nil {
		return nil
	}

	return container.v1alpha1Config.Cluster()
}

func findMatchingDocs[T any](documents []config.Document) []T {
	var result []T

	for _, doc := range documents {
		if c, ok := doc.(T); ok {
			result = append(result, c)
		}
	}

	return result
}

// SideroLink implements config.Config interface.
func (container *Container) SideroLink() config.SideroLinkConfig {
	matching := findMatchingDocs[config.SideroLinkConfig](container.documents)
	if len(matching) == 0 {
		return nil
	}

	return matching[0]
}

// ExtensionServiceConfigs implements config.Config interface.
func (container *Container) ExtensionServiceConfigs() []config.ExtensionServiceConfig {
	return findMatchingDocs[config.ExtensionServiceConfig](container.documents)
}

// Runtime implements config.Config interface.
func (container *Container) Runtime() config.RuntimeConfig {
	return config.WrapRuntimeConfigList(findMatchingDocs[config.RuntimeConfig](container.documents)...)
}

// Environment implements config.Config interface.
func (container *Container) Environment() config.EnvironmentConfig {
	return config.WrapEnvironmentConfigList(findMatchingDocs[config.EnvironmentConfig](container.documents)...)
}

// EtcFileConfigs implements config.Config interface.
func (container *Container) EtcFileConfigs() []config.EtcFileConfig {
	return findMatchingDocs[config.EtcFileConfig](container.documents)
}

// CRICustomizationConfigs implements config.Config interface.
func (container *Container) CRICustomizationConfigs() []config.CRICustomizationConfig {
	matching := findMatchingDocs[config.CRICustomizationConfig](container.documents)

	if container.v1alpha1Config != nil {
		matching = append(matching, container.v1alpha1Config.CRICustomizationConfigs()...)
	}

	slices.SortStableFunc(matching, func(a, b config.CRICustomizationConfig) int {
		return strings.Compare(a.Name(), b.Name())
	})

	return matching
}

// CRIBaseRuntimeSpecConfig implements config.Config interface.
func (container *Container) CRIBaseRuntimeSpecConfig() config.CRIBaseRuntimeSpecConfig {
	matching := findMatchingDocs[config.CRIBaseRuntimeSpecConfig](container.documents)
	if len(matching) > 0 {
		return matching[0]
	}

	if container.v1alpha1Config != nil {
		return container.v1alpha1Config.CRIBaseRuntimeSpecConfig()
	}

	return nil
}

// UdevRulesConfig implements config.Config interface.
func (container *Container) UdevRulesConfig() config.UdevConfig {
	matching := findMatchingDocs[config.UdevConfig](container.documents)
	if len(matching) > 0 {
		return matching[0]
	}

	if container.v1alpha1Config != nil {
		return container.v1alpha1Config.Machine().Udev()
	}

	return nil
}

// SysctlConfig implements config.Config interface.
//
// The deprecated v1alpha1 values are merged with the multi-doc documents,
// with the multi-doc documents taking precedence on key conflicts.
func (container *Container) SysctlConfig() map[string]string {
	var configs []config.SysctlConfig

	// v1alpha1 has the lowest priority
	if container.v1alpha1Config != nil {
		configs = append(configs, container.v1alpha1Config.Machine())
	}

	// dedicated documents take precedence over v1alpha1
	configs = append(configs, findMatchingDocs[config.SysctlConfig](container.documents)...)

	// Config order matters, last one wins during merge.
	return config.WrapSysctlConfigList(configs...)
}

// SysfsConfig implements config.Config interface.
//
// The deprecated v1alpha1 values are merged with the multi-doc documents,
// with the multi-doc documents taking precedence on key conflicts.
func (container *Container) SysfsConfig() map[string]string {
	var configs []config.SysfsConfig

	// v1alpha1 has the lowest priority
	if container.v1alpha1Config != nil {
		configs = append(configs, container.v1alpha1Config.Machine())
	}

	// dedicated documents take precedence over v1alpha1
	configs = append(configs, findMatchingDocs[config.SysfsConfig](container.documents)...)

	// Config order matters, last one wins during merge.
	return config.WrapSysfsConfigList(configs...)
}

// KernelModuleConfigs implements config.Config interface.
//
// The deprecated v1alpha1 .machine.kernel.modules values are merged with the multi-doc documents,
// with the multi-doc documents taking precedence over the legacy config on module name conflicts
// (enforced by the KernelModuleConfigController, which writes one resource per module name).
func (container *Container) KernelModuleConfigs() []config.KernelModuleConfig {
	var modules []config.KernelModuleConfig

	// v1alpha1 has the lowest priority
	if container.v1alpha1Config != nil {
		modules = container.v1alpha1Config.KernelModuleConfigs()
	}

	// dedicated documents take precedence over v1alpha1
	modules = append(modules, findMatchingDocs[config.KernelModuleConfig](container.documents)...)

	return modules
}

// UnattendedInstallConfig implements config.Config interface.
func (container *Container) UnattendedInstallConfig() config.UnattendedInstallConfig {
	matching := findMatchingDocs[config.UnattendedInstallConfig](container.documents)
	if len(matching) == 0 {
		return nil
	}

	return matching[0]
}

// NetworkRules implements config.Config interface.
func (container *Container) NetworkRules() config.NetworkRuleConfig {
	return config.WrapNetworkRuleConfigList(findMatchingDocs[config.NetworkRuleConfigSignal](container.documents)...)
}

// TrustedRoots implements config.Config interface.
func (container *Container) TrustedRoots() config.TrustedRootsConfig {
	return config.WrapTrustedRootsConfig(findMatchingDocs[config.TrustedRootsConfig](container.documents)...)
}

// Volumes implements config.Config interface.
func (container *Container) Volumes() config.VolumesConfig {
	return config.WrapVolumesConfigList(findMatchingDocs[config.VolumeConfig](container.documents)...)
}

// KubespanConfig implements config.Config interface.
func (container *Container) KubespanConfig() config.KubespanConfig {
	return config.WrapKubespanConfig(findMatchingDocs[config.KubespanConfig](container.documents)...)
}

// DiscoveryServiceConfigs implements config.Config interface.
//
// Dedicated documents and the deprecated v1alpha1 discovery config are mutually exclusive
// (enforced by DiscoveryServiceConfigV1Alpha1.V1Alpha1ConflictValidate); the v1alpha1 config takes priority.
func (container *Container) DiscoveryServiceConfigs() []config.DiscoveryServiceConfig {
	// v1alpha1 discovery takes priority when it yields a config
	if container.v1alpha1Config != nil {
		if legacy := container.v1alpha1Config.DiscoveryServiceConfigs(); len(legacy) > 0 {
			return legacy
		}
	}

	// fallback to dedicated documents
	return findMatchingDocs[config.DiscoveryServiceConfig](container.documents)
}

// DiscoveryIdentityConfig implements config.Config interface.
//
// The dedicated document and the deprecated v1alpha1 cluster identity (.cluster.id/.cluster.secret) are
// mutually exclusive (enforced by DiscoveryIdentityConfigV1Alpha1.V1Alpha1ConflictValidate); the v1alpha1
// config takes priority.
func (container *Container) DiscoveryIdentityConfig() config.DiscoveryIdentityConfig {
	// v1alpha1 cluster identity takes priority when it yields a config
	if container.v1alpha1Config != nil {
		if legacy := container.v1alpha1Config.DiscoveryIdentityConfig(); legacy != nil {
			return legacy
		}
	}

	// fallback to dedicated multi-doc. Take first, since this doc is not named.
	if docs := findMatchingDocs[config.DiscoveryIdentityConfig](container.documents); len(docs) > 0 {
		return docs[0]
	}

	return nil
}

// PCIDriverRebindConfig implements config.Config interface.
func (container *Container) PCIDriverRebindConfig() config.PCIDriverRebindConfig {
	return config.WrapPCIDriverRebindConfig(findMatchingDocs[config.PCIDriverRebindConfig](container.documents)...)
}

// EthernetConfigs implements config.Config interface.
func (container *Container) EthernetConfigs() []config.EthernetConfig {
	return findMatchingDocs[config.EthernetConfig](container.documents)
}

// UserVolumeConfigs implements config.Config interface.
func (container *Container) UserVolumeConfigs() []config.UserVolumeConfig {
	return findMatchingDocs[config.UserVolumeConfig](container.documents)
}

// ExternalVolumeConfigs implements config.Config interface.
func (container *Container) ExternalVolumeConfigs() []config.ExternalVolumeConfig {
	return findMatchingDocs[config.ExternalVolumeConfig](container.documents)
}

// RawVolumeConfigs implements config.Config interface.
func (container *Container) RawVolumeConfigs() []config.RawVolumeConfig {
	return findMatchingDocs[config.RawVolumeConfig](container.documents)
}

// ExistingVolumeConfigs implements config.Config interface.
func (container *Container) ExistingVolumeConfigs() []config.ExistingVolumeConfig {
	return findMatchingDocs[config.ExistingVolumeConfig](container.documents)
}

// SwapVolumeConfigs implements config.Config interface.
func (container *Container) SwapVolumeConfigs() []config.SwapVolumeConfig {
	return findMatchingDocs[config.SwapVolumeConfig](container.documents)
}

// LVMVolumeGroupConfigs implements config.Config interface.
func (container *Container) LVMVolumeGroupConfigs() []config.LVMVolumeGroupConfig {
	return findMatchingDocs[config.LVMVolumeGroupConfig](container.documents)
}

// LVMLogicalVolumeConfigs implements config.Config interface.
func (container *Container) LVMLogicalVolumeConfigs() []config.LVMLogicalVolumeConfig {
	return findMatchingDocs[config.LVMLogicalVolumeConfig](container.documents)
}

// RAIDArrayConfigs implements config.Config interface.
func (container *Container) RAIDArrayConfigs() []config.RAIDArrayConfig {
	return findMatchingDocs[config.RAIDArrayConfig](container.documents)
}

// ZswapConfig implements config.Config interface.
func (container *Container) ZswapConfig() config.ZswapConfig {
	matching := findMatchingDocs[config.ZswapConfig](container.documents)
	if len(matching) == 0 {
		return nil
	}

	return matching[0]
}

// FilesystemTrimConfig implements config.Config interface.
func (container *Container) FilesystemTrimConfig() config.FilesystemTrimConfig {
	matching := findMatchingDocs[config.FilesystemTrimConfig](container.documents)
	if len(matching) == 0 {
		return nil
	}

	return matching[0]
}

// SecurityProfileConfig implements config.Config interface.
func (container *Container) SecurityProfileConfig() config.SecurityProfileConfig {
	matching := findMatchingDocs[config.SecurityProfileConfig](container.documents)
	if len(matching) == 0 {
		return nil
	}

	return matching[0]
}

// NetworkStaticHostConfig implements config.Config interface.
func (container *Container) NetworkStaticHostConfig() []config.NetworkStaticHostConfig {
	return slices.Concat(
		container.v1alpha1Config.NetworkStaticHostConfig(),
		findMatchingDocs[config.NetworkStaticHostConfig](container.documents),
	)
}

// NetworkHostnameConfig implements config.Config interface.
func (container *Container) NetworkHostnameConfig() config.NetworkHostnameConfig {
	// first check if we have a dedicated document
	matching := findMatchingDocs[config.NetworkHostnameConfig](container.documents)
	if len(matching) > 0 {
		return matching[0]
	}

	// fallback to v1alpha1
	if container.v1alpha1Config != nil {
		return container.v1alpha1Config
	}

	return nil
}

// NetworkResolverConfig implements config.Config interface.
func (container *Container) NetworkResolverConfig() config.NetworkResolverConfig {
	// first check if we have a dedicated document
	matching := findMatchingDocs[config.NetworkResolverConfig](container.documents)
	if len(matching) > 0 {
		return matching[0]
	}

	// fallback to v1alpha1
	if container.v1alpha1Config != nil {
		return container.v1alpha1Config
	}

	return nil
}

// NetworkHostDNSConfig implements config.Config interface.
func (container *Container) NetworkHostDNSConfig() config.NetworkHostDNSConfig {
	// first check if we have a dedicated document, and it is not empty
	// for backwards compatibility, we will fall back to v1alpha1 if the ResolverConfig document does not have hostDNS enabled
	matching := findMatchingDocs[config.NetworkHostDNSConfig](container.documents)
	if len(matching) > 0 && matching[0].HostDNSEnabled() {
		return matching[0]
	}

	// fallback to v1alpha1
	if container.v1alpha1Config != nil {
		return container.v1alpha1Config.NetworkHostDNSConfig()
	}

	return nil
}

// NetworkTimeSyncConfig implements config.Config interface.
func (container *Container) NetworkTimeSyncConfig() config.NetworkTimeSyncConfig {
	// first check if we have a dedicated document
	matching := findMatchingDocs[config.NetworkTimeSyncConfig](container.documents)
	if len(matching) > 0 {
		return matching[0]
	}

	// fallback to v1alpha1
	if container.v1alpha1Config != nil {
		return container.v1alpha1Config.NetworkTimeSyncConfig()
	}

	return nil
}

// NetworkKubeSpanConfig implements config.Config interface.
func (container *Container) NetworkKubeSpanConfig() config.NetworkKubeSpanConfig {
	// first check if we have a dedicated document
	matching := findMatchingDocs[config.NetworkKubeSpanConfig](container.documents)
	if len(matching) > 0 {
		return matching[0]
	}

	// fallback to v1alpha1
	if container.v1alpha1Config != nil {
		return container.v1alpha1Config.NetworkKubeSpanConfig()
	}

	return nil
}

// NetworkCommonLinkConfigs implements config.Config interface.
func (container *Container) NetworkCommonLinkConfigs() []config.NetworkCommonLinkConfig {
	result := findMatchingDocs[config.NetworkCommonLinkConfig](container.documents)

	for _, additional := range findMatchingDocs[config.NetworkAdditionalLinkConfigs](container.documents) {
		result = append(result, additional.AdditionalLinkConfigs()...)
	}

	return result
}

// NetworkLinkAliasConfigs implements config.Config interface.
func (container *Container) NetworkLinkAliasConfigs() []config.NetworkLinkAliasConfig {
	return findMatchingDocs[config.NetworkLinkAliasConfig](container.documents)
}

// NetworkDHCPConfigs implements config.Config interface.
func (container *Container) NetworkDHCPConfigs() []config.NetworkDHCPConfig {
	return findMatchingDocs[config.NetworkDHCPConfig](container.documents)
}

// NetworkBGPPeerConfig implements config.Config interface.
func (container *Container) NetworkBGPPeerConfig() config.NetworkBGPPeerConfig {
	matching := findMatchingDocs[config.NetworkBGPPeerConfig](container.documents)
	if len(matching) == 0 {
		return nil
	}

	return matching[0]
}

// NetworkDHCPv4Configs implements config.Config interface.
func (container *Container) NetworkDHCPv4Configs() []config.NetworkDHCPv4Config {
	return findMatchingDocs[config.NetworkDHCPv4Config](container.documents)
}

// NetworkDHCPv6Configs implements config.Config interface.
func (container *Container) NetworkDHCPv6Configs() []config.NetworkDHCPv6Config {
	return findMatchingDocs[config.NetworkDHCPv6Config](container.documents)
}

// NetworkVirtualIPConfigs implements config.Config interface.
func (container *Container) NetworkVirtualIPConfigs() []config.NetworkVirtualIPConfig {
	return findMatchingDocs[config.NetworkVirtualIPConfig](container.documents)
}

// NetworkProbeConfigs implements config.Config interface.
func (container *Container) NetworkProbeConfigs() []config.NetworkCommonProbeConfig {
	return findMatchingDocs[config.NetworkCommonProbeConfig](container.documents)
}

// NetworkBlackholeRouteConfigs implements config.Config interface.
func (container *Container) NetworkBlackholeRouteConfigs() []config.NetworkBlackholeRouteConfig {
	return findMatchingDocs[config.NetworkBlackholeRouteConfig](container.documents)
}

// NetworkRoutingRuleConfigs implements config.Config interface.
func (container *Container) NetworkRoutingRuleConfigs() []config.NetworkRoutingRuleConfig {
	return findMatchingDocs[config.NetworkRoutingRuleConfig](container.documents)
}

// RunDefaultDHCPOperators implements config.Config interface.
//
// The rules for this are:
//   - if there is a single new-style network config document for links,
//     we immediately stop running default DHCP operators (as user is taking full control)
func (container *Container) RunDefaultDHCPOperators() bool {
	return len(findMatchingDocs[config.NetworkCommonLinkConfig](container.documents)) == 0 &&
		len(findMatchingDocs[config.NetworkDHCPConfig](container.documents)) == 0
}

// K8sAdmissionControlPluginConfigs implements config.Config interface.
func (container *Container) K8sAdmissionControlPluginConfigs() []config.K8sAdmissionControlPluginConfig {
	docs := findMatchingDocs[config.K8sAdmissionControlPluginConfig](container.documents)
	if len(docs) > 0 {
		return docs
	}

	if container.v1alpha1Config != nil {
		return container.v1alpha1Config.K8sAdmissionControlPluginConfigs()
	}

	return nil
}

// K8sAPIServerCAConfig implements config.Config interface.
func (container *Container) K8sAPIServerCAConfig() config.K8sAPIServerCAConfig {
	matching := findMatchingDocs[config.K8sAPIServerCAConfig](container.documents)
	if len(matching) > 0 {
		return matching[0]
	}

	if container.v1alpha1Config != nil {
		return container.v1alpha1Config.K8sAPIServerCAConfig()
	}

	return nil
}

// K8sAggregatorCAConfig implements config.Config interface.
func (container *Container) K8sAggregatorCAConfig() config.K8sAggregatorCAConfig {
	matching := findMatchingDocs[config.K8sAggregatorCAConfig](container.documents)
	if len(matching) > 0 {
		return matching[0]
	}

	if container.v1alpha1Config != nil {
		return container.v1alpha1Config.K8sAggregatorCAConfig()
	}

	return nil
}

// K8sAuditPolicyConfig implements config.Config interface.
func (container *Container) K8sAuditPolicyConfig() config.K8sAuditPolicyConfig {
	matching := findMatchingDocs[config.K8sAuditPolicyConfig](container.documents)
	if len(matching) > 0 {
		return matching[0]
	}

	if container.v1alpha1Config != nil {
		return container.v1alpha1Config.K8sAuditPolicyConfig()
	}

	return nil
}

// K8sAuthenticationConfig implements config.Config interface.
func (container *Container) K8sAuthenticationConfig() config.K8sAuthenticationConfig {
	matching := findMatchingDocs[config.K8sAuthenticationConfig](container.documents)
	if len(matching) > 0 {
		return matching[0]
	}

	return nil
}

// K8sAuthorizerConfigs implements config.Config interface.
func (container *Container) K8sAuthorizerConfigs() []config.K8sAuthorizerConfig {
	docs := findMatchingDocs[config.K8sAuthorizerConfig](container.documents)
	if len(docs) > 0 {
		return docs
	}

	if container.v1alpha1Config != nil {
		return container.v1alpha1Config.K8sAuthorizerConfigs()
	}

	return nil
}

// K8sEtcdEncryptionConfig implements config.Config interface.
func (container *Container) K8sEtcdEncryptionConfig() config.K8sEtcdEncryptionConfig {
	matching := findMatchingDocs[config.K8sEtcdEncryptionConfig](container.documents)
	if len(matching) == 0 {
		return nil
	}

	return matching[0]
}

// K8sAPIServerConfig implements config.Config interface.
func (container *Container) K8sAPIServerConfig() config.K8sAPIServerConfig {
	matching := findMatchingDocs[config.K8sAPIServerConfig](container.documents)
	if len(matching) > 0 {
		return matching[0]
	}

	if container.v1alpha1Config != nil {
		return container.v1alpha1Config.K8sAPIServerConfig()
	}

	return nil
}

// K8sControllerManagerConfig implements config.Config interface.
func (container *Container) K8sControllerManagerConfig() config.K8sControllerManagerConfig {
	matching := findMatchingDocs[config.K8sControllerManagerConfig](container.documents)
	if len(matching) > 0 {
		return matching[0]
	}

	if container.v1alpha1Config != nil {
		return container.v1alpha1Config.K8sControllerManagerConfig()
	}

	return nil
}

// K8sSchedulerConfig implements config.Config interface.
func (container *Container) K8sSchedulerConfig() config.K8sSchedulerConfig {
	matching := findMatchingDocs[config.K8sSchedulerConfig](container.documents)
	if len(matching) > 0 {
		return matching[0]
	}

	if container.v1alpha1Config != nil {
		return container.v1alpha1Config.K8sSchedulerConfig()
	}

	return nil
}

// K8sProxyConfig implements config.Config interface.
func (container *Container) K8sProxyConfig() config.K8sProxyConfig {
	matching := findMatchingDocs[config.K8sProxyConfig](container.documents)
	if len(matching) > 0 {
		return matching[0]
	}

	if container.v1alpha1Config != nil {
		return container.v1alpha1Config.K8sProxyConfig()
	}

	return nil
}

// K8sClusterConfig implements config.Config interface.
func (container *Container) K8sClusterConfig() config.K8sClusterConfig {
	matching := findMatchingDocs[config.K8sClusterConfig](container.documents)
	if len(matching) > 0 {
		return matching[0]
	}

	if container.v1alpha1Config != nil {
		return container.v1alpha1Config.K8sClusterConfig()
	}

	return nil
}

// K8sNodeConfig implements config.Config interface.
func (container *Container) K8sNodeConfig() config.K8sNodeConfig {
	matching := findMatchingDocs[config.K8sNodeConfig](container.documents)
	if len(matching) > 0 {
		return matching[0]
	}

	if container.v1alpha1Config != nil {
		return container.v1alpha1Config.K8sNodeConfig()
	}

	return nil
}

// K8sNetworkConfig implements config.Config interface.
func (container *Container) K8sNetworkConfig() config.K8sNetworkConfig {
	matching := findMatchingDocs[config.K8sNetworkConfig](container.documents)
	if len(matching) > 0 {
		return matching[0]
	}

	if container.v1alpha1Config != nil {
		return container.v1alpha1Config.K8sNetworkConfig()
	}

	return nil
}

// K8sFlannelCNIConfig implements config.Config interface.
func (container *Container) K8sFlannelCNIConfig() config.K8sFlannelCNIConfig {
	matching := findMatchingDocs[config.K8sFlannelCNIConfig](container.documents)
	if len(matching) > 0 {
		return matching[0]
	}

	if container.v1alpha1Config != nil {
		return container.v1alpha1Config.K8sFlannelCNIConfig()
	}

	return nil
}

// K8sCoreDNSConfig implements config.Config interface.
func (container *Container) K8sCoreDNSConfig() config.K8sCoreDNSConfig {
	matching := findMatchingDocs[config.K8sCoreDNSConfig](container.documents)
	if len(matching) > 0 {
		return matching[0]
	}

	if container.v1alpha1Config != nil {
		return container.v1alpha1Config.K8sCoreDNSConfig()
	}

	return nil
}

// K8sServiceAccountConfig implements config.Config interface.
func (container *Container) K8sServiceAccountConfig() config.K8sServiceAccountConfig {
	matching := findMatchingDocs[config.K8sServiceAccountConfig](container.documents)
	if len(matching) > 0 {
		return matching[0]
	}

	if container.v1alpha1Config != nil {
		return container.v1alpha1Config.K8sServiceAccountConfig()
	}

	return nil
}

// K8sKubeletConfig implements config.Config interface.
func (container *Container) K8sKubeletConfig() config.K8sKubeletConfig {
	matching := findMatchingDocs[config.K8sKubeletConfig](container.documents)
	if len(matching) > 0 {
		return matching[0]
	}

	if container.v1alpha1Config != nil {
		return container.v1alpha1Config.K8sKubeletConfig()
	}

	return nil
}

// K8sCredentialProviderConfig implements config.Config interface.
func (container *Container) K8sCredentialProviderConfig() config.K8sCredentialProviderConfig {
	matching := findMatchingDocs[config.K8sCredentialProviderConfig](container.documents)
	if len(matching) > 0 {
		return matching[0]
	}

	if container.v1alpha1Config != nil {
		return container.v1alpha1Config.K8sCredentialProviderConfig()
	}

	return nil
}

// K8sStaticPodConfigs implements config.Config interface.
func (container *Container) K8sStaticPodConfigs() []config.K8sStaticPodConfig {
	matching := findMatchingDocs[config.K8sStaticPodConfig](container.documents)

	if container.v1alpha1Config != nil {
		matching = append(matching, container.v1alpha1Config.K8sStaticPodConfigs()...)
	}

	return matching
}

// K8sInlineManifestConfigs implements config.Config interface.
func (container *Container) K8sInlineManifestConfigs() []config.K8sInlineManifestConfig {
	matching := findMatchingDocs[config.K8sInlineManifestConfig](container.documents)

	if container.v1alpha1Config != nil {
		matching = append(matching, container.v1alpha1Config.K8sInlineManifestConfigs()...)
	}

	return matching
}

// K8sExternalManifestConfigs implements config.Config interface.
func (container *Container) K8sExternalManifestConfigs() []config.K8sExternalManifestConfig {
	matching := findMatchingDocs[config.K8sExternalManifestConfig](container.documents)

	if container.v1alpha1Config != nil {
		matching = append(matching, container.v1alpha1Config.K8sExternalManifestConfigs()...)
	}

	return matching
}

// K8sKubePrismConfig implements config.Config interface.
func (container *Container) K8sKubePrismConfig() config.K8sKubePrismConfig {
	matching := findMatchingDocs[config.K8sKubePrismConfig](container.documents)
	if len(matching) > 0 {
		return matching[0]
	}

	if container.v1alpha1Config != nil {
		return container.v1alpha1Config.K8sKubePrismConfig()
	}

	return nil
}

// K8sTalosAPIAccessConfig implements config.Config interface.
func (container *Container) K8sTalosAPIAccessConfig() config.K8sTalosAPIAccessConfig {
	matching := findMatchingDocs[config.K8sTalosAPIAccessConfig](container.documents)
	if len(matching) > 0 {
		return matching[0]
	}

	if container.v1alpha1Config != nil {
		return container.v1alpha1Config.K8sTalosAPIAccessConfig()
	}

	return nil
}

// OOMConfig implements config.Config interface.
func (container *Container) OOMConfig() config.OOMConfig {
	matching := findMatchingDocs[config.OOMConfig](container.documents)
	if len(matching) == 0 {
		return config.DefaultOOMConfig{}
	}

	return matching[0]
}

// RegistryMirrorConfigs implements config.Config interface.
func (container *Container) RegistryMirrorConfigs() map[string]config.RegistryMirrorConfig {
	var cfg map[string]config.RegistryMirrorConfig

	if container.v1alpha1Config != nil {
		cfg = container.v1alpha1Config.RegistryMirrorConfigs()
	}

	docs := findMatchingDocs[config.RegistryMirrorConfigDocument](container.documents)

	if cfg == nil {
		cfg = make(map[string]config.RegistryMirrorConfig, len(docs))
	}

	for _, doc := range docs {
		cfg[doc.Name()] = doc
	}

	return cfg
}

// RegistryAuthConfigs implements config.Config interface.
func (container *Container) RegistryAuthConfigs() map[string]config.RegistryAuthConfig {
	var cfg map[string]config.RegistryAuthConfig

	if container.v1alpha1Config != nil {
		cfg = container.v1alpha1Config.RegistryAuthConfigs()
	}

	docs := findMatchingDocs[config.RegistryAuthConfigDocument](container.documents)

	if cfg == nil {
		cfg = make(map[string]config.RegistryAuthConfig, len(docs))
	}

	for _, doc := range docs {
		cfg[doc.Name()] = doc
	}

	return cfg
}

// RegistryTLSConfigs implements config.Config interface.
func (container *Container) RegistryTLSConfigs() map[string]config.RegistryTLSConfig {
	var cfg map[string]config.RegistryTLSConfig

	if container.v1alpha1Config != nil {
		cfg = container.v1alpha1Config.RegistryTLSConfigs()
	}

	docs := findMatchingDocs[config.RegistryTLSConfigDocument](container.documents)

	if cfg == nil {
		cfg = make(map[string]config.RegistryTLSConfig, len(docs))
	}

	for _, doc := range docs {
		cfg[doc.Name()] = doc
	}

	return cfg
}

// ImageCacheConfig implements config.Config interface.
func (container *Container) ImageCacheConfig() config.ImageCacheConfig {
	// first check if we have a dedicated document
	matching := findMatchingDocs[config.ImageCacheConfig](container.documents)
	if len(matching) > 0 {
		return matching[0]
	}

	// fallback to v1alpha1
	if container.v1alpha1Config != nil {
		return container.v1alpha1Config.ImageCacheConfig()
	}

	return nil
}

// ImageVerificationConfig implements config.Config interface.
func (container *Container) ImageVerificationConfig() config.ImageVerificationConfig {
	docs := findMatchingDocs[config.ImageVerificationConfig](container.documents)
	if len(docs) == 0 {
		return nil
	}

	return docs[0]
}

// Bytes returns source YAML representation (if available) or does default encoding.
func (container *Container) Bytes() ([]byte, error) {
	if !container.readonly {
		return container.EncodeBytes()
	}

	if container.bytes == nil {
		panic("container.Bytes() called on a readonly container without bytes")
	}

	return container.bytes, nil
}

// EncodeString configuration to YAML using the provided options.
func (container *Container) EncodeString(encoderOptions ...encoder.Option) (string, error) {
	var buf strings.Builder

	err := container.encodeToBuf(&buf, encoderOptions...)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}

// EncodeBytes configuration to YAML using the provided options.
func (container *Container) EncodeBytes(encoderOptions ...encoder.Option) ([]byte, error) {
	var buf bytes.Buffer

	err := container.encodeToBuf(&buf, encoderOptions...)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

type buffer interface {
	Len() int
	Write(p []byte) (int, error)
	WriteString(s string) (int, error)
}

func (container *Container) encodeToBuf(buf buffer, encoderOptions ...encoder.Option) error {
	if container.v1alpha1Config != nil {
		b, err := encoder.NewEncoder(container.v1alpha1Config, encoderOptions...).Encode()
		if err != nil {
			return err
		}

		buf.Write(b) //nolint:errcheck
	}

	for _, doc := range container.documents {
		if buf.Len() > 0 {
			buf.WriteString("---\n") //nolint:errcheck
		}

		b, err := encoder.NewEncoder(doc, encoderOptions...).Encode()
		if err != nil {
			return err
		}

		buf.Write(b) //nolint:errcheck
	}

	return nil
}

func docID(doc config.Document) string {
	id := doc.Kind()

	if named, ok := doc.(config.NamedDocument); ok {
		id += "/" + named.Name()
	}

	return id
}

// RedactSecrets returns a copy of the Provider with all secrets replaced with the given string.
func (container *Container) RedactSecrets(replacement string) coreconfig.Provider {
	clone := container.clone()

	if clone.v1alpha1Config != nil {
		clone.v1alpha1Config.Redact(replacement)
	}

	for _, doc := range clone.documents {
		if secretDoc, ok := doc.(config.SecretDocument); ok {
			secretDoc.Redact(replacement)
		}
	}

	return clone
}

// RawV1Alpha1 returns internal config representation for v1alpha1.Config.
func (container *Container) RawV1Alpha1() *v1alpha1.Config {
	if container.readonly {
		return container.v1alpha1Config.DeepCopy()
	}

	return container.v1alpha1Config
}

// Documents returns all documents in the container.
//
// Documents should not be modified.
func (container *Container) Documents() []config.Document {
	result := make([]config.Document, 0, len(container.documents)+1)

	// first we take deletes for v1alpha1
	for _, doc := range container.documents {
		if _, ok := doc.(selector); ok && doc.Kind() == v1alpha1.Version {
			result = append(result, doc)
		}
	}

	// then we take the v1alpha1 config
	if container.v1alpha1Config != nil {
		result = append(result, container.v1alpha1Config)
	}

	// then we take the rest
	for _, doc := range container.documents {
		if _, ok := doc.(selector); ok && doc.Kind() == v1alpha1.Version {
			continue
		}

		result = append(result, doc)
	}

	return result
}

type selector interface{ ApplyTo(config.Document) error }

// CompleteForBoot return true if the machine config is enough to proceed with the boot process.
func (container *Container) CompleteForBoot() bool {
	// for now, v1alpha1 config is required
	return container.v1alpha1Config != nil
}
