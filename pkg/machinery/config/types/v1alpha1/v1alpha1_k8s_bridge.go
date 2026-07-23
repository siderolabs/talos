// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1

import (
	"maps"
	"net/url"
	"slices"

	"github.com/siderolabs/crypto/x509"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/go-pointer"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// K8sAPIServerCAConfig implements the config.K8sAPIServerCAConfig interface.
func (c *Config) K8sAPIServerCAConfig() config.K8sAPIServerCAConfig {
	clusterConfig := c.ClusterConfig
	if clusterConfig == nil {
		return nil
	}

	if clusterConfig.ClusterCA == nil && clusterConfig.ClusterAcceptedCAs == nil {
		return nil
	}

	return apiServerCAConfigShim{
		clusterConfig,
	}
}

type apiServerCAConfigShim struct {
	c *ClusterConfig
}

// K8sAPIServerCAConfigSignal implements the config.K8sAPIServerCAConfig interface.
func (s apiServerCAConfigShim) K8sAPIServerCAConfigSignal() {}

// IssuingCA implements the config.K8sAPIServerCAConfig interface.
func (s apiServerCAConfigShim) IssuingCA() *x509.PEMEncodedCertificateAndKey {
	if s.c.ClusterCA == nil || len(s.c.ClusterCA.Key) == 0 {
		return nil
	}

	return s.c.ClusterCA
}

// AcceptedCAs implements the config.K8sAPIServerCAConfig interface.
func (s apiServerCAConfigShim) AcceptedCAs() []*x509.PEMEncodedCertificate {
	acceptedCAs := slices.Clone(s.c.ClusterAcceptedCAs)

	if s.c.ClusterCA != nil && s.c.ClusterCA.Crt != nil {
		acceptedCAs = slices.Insert(acceptedCAs, 0, &x509.PEMEncodedCertificate{
			Crt: s.c.ClusterCA.Crt,
		})
	}

	return acceptedCAs
}

// K8sAggregatorCAConfig implements the config.K8sAggregatorCAConfig interface.
func (c *Config) K8sAggregatorCAConfig() config.K8sAggregatorCAConfig {
	clusterConfig := c.ClusterConfig
	if clusterConfig == nil {
		return nil
	}

	if clusterConfig.ClusterAggregatorCA == nil {
		return nil
	}

	return aggregatorCAConfigShim{
		c: clusterConfig,
	}
}

type aggregatorCAConfigShim struct {
	c *ClusterConfig
}

// K8sAggregatorCAConfigSignal implements the config.K8sAggregatorCAConfig interface.
func (s aggregatorCAConfigShim) K8sAggregatorCAConfigSignal() {}

// IssuingCA implements the config.K8sAggregatorCAConfig interface.
func (s aggregatorCAConfigShim) IssuingCA() *x509.PEMEncodedCertificateAndKey {
	return s.c.ClusterAggregatorCA
}

// AcceptedCAs implements the config.K8sAggregatorCAConfig interface.
func (s aggregatorCAConfigShim) AcceptedCAs() []*x509.PEMEncodedCertificate {
	return []*x509.PEMEncodedCertificate{
		{
			Crt: s.c.ClusterAggregatorCA.Crt,
		},
	}
}

// K8sAPIServerConfig implements the config.Config interface.
func (c *Config) K8sAPIServerConfig() config.K8sAPIServerConfig {
	clusterConfig := c.ClusterConfig
	if clusterConfig == nil {
		clusterConfig = &ClusterConfig{}
	}

	return struct {
		*APIServerConfig
		apiServerConfigShim
	}{
		APIServerConfig:     clusterConfig.APIServer(),
		apiServerConfigShim: apiServerConfigShim{c: clusterConfig},
	}
}

type apiServerConfigShim struct {
	c *ClusterConfig
}

// K8sAPIServerConfigSignal implements the config.K8sAPIServerConfig interface.
func (s apiServerConfigShim) K8sAPIServerConfigSignal() {}

// APIPort implements the config.K8sAPIServerConfig interface.
func (s apiServerConfigShim) APIPort() int {
	return s.c.LocalAPIServerPort()
}

// CertSANs implements the config.K8sAPIServerConfig interface.
func (s apiServerConfigShim) CertSANs() []string {
	return s.c.CertSANs()
}

// StartupProbesEnabled implements the config.K8sAPIServerConfig interface.
func (s apiServerConfigShim) StartupProbesEnabled() bool {
	return false
}

// UseAuthenticationConfig implements the config.K8sAPIServerConfig interface.
func (s apiServerConfigShim) UseAuthenticationConfig() bool {
	return false
}

// InjectDefaultAuthorizers implements the config.K8sAPIServerConfig interface.
func (s apiServerConfigShim) InjectDefaultAuthorizers() bool {
	return true
}

// K8sSchedulerConfig implements the config.Config interface.
func (c *Config) K8sSchedulerConfig() config.K8sSchedulerConfig {
	clusterConfig := c.ClusterConfig
	if clusterConfig == nil {
		clusterConfig = &ClusterConfig{}
	}

	return struct {
		*SchedulerConfig
		schedulerConfigShim
	}{
		SchedulerConfig:     clusterConfig.Scheduler(),
		schedulerConfigShim: schedulerConfigShim{c: c},
	}
}

type schedulerConfigShim struct {
	c *Config
}

// K8sSchedulerConfigSignal implements the config.K8sSchedulerConfig interface.
func (s schedulerConfigShim) K8sSchedulerConfigSignal() {}

// Enabled implements the config.K8sSchedulerConfig interface.
func (s schedulerConfigShim) Enabled() bool {
	if s.c.MachineConfig == nil || s.c.MachineConfig.MachineControlPlane == nil {
		return true
	}

	mcp := s.c.MachineConfig.MachineControlPlane

	if mcp.MachineScheduler != nil {
		return !pointer.SafeDeref(mcp.MachineScheduler.MachineSchedulerDisabled)
	}

	return true
}

// K8sControllerManagerConfig implements the config.Config interface.
func (c *Config) K8sControllerManagerConfig() config.K8sControllerManagerConfig {
	clusterConfig := c.ClusterConfig
	if clusterConfig == nil {
		clusterConfig = &ClusterConfig{}
	}

	return struct {
		*ControllerManagerConfig
		controllerManagerConfigShim
	}{
		ControllerManagerConfig:     clusterConfig.ControllerManager(),
		controllerManagerConfigShim: controllerManagerConfigShim{c: c},
	}
}

type controllerManagerConfigShim struct {
	c *Config
}

// K8sControllerManagerConfigSignal implements the config.K8sControllerManagerConfig interface.
func (s controllerManagerConfigShim) K8sControllerManagerConfigSignal() {}

// Enabled implements the config.K8sControllerManagerConfig interface.
func (s controllerManagerConfigShim) Enabled() bool {
	if s.c.MachineConfig == nil || s.c.MachineConfig.MachineControlPlane == nil {
		return true
	}

	mcp := s.c.MachineConfig.MachineControlPlane

	if mcp.MachineControllerManager != nil {
		return !pointer.SafeDeref(mcp.MachineControllerManager.MachineControllerManagerDisabled)
	}

	return true
}

// K8sProxyConfig implements the config.Config interface.
func (c *Config) K8sProxyConfig() config.K8sProxyConfig {
	clusterConfig := c.ClusterConfig
	if clusterConfig == nil {
		clusterConfig = &ClusterConfig{}
	}

	return clusterConfig.Proxy()
}

// K8sNetworkConfig implements the config.Config interface.
func (c *Config) K8sNetworkConfig() config.K8sNetworkConfig {
	// if the section is missing, assume it's not set (multi-doc should provide it)
	if c.ClusterConfig == nil || c.ClusterConfig.ClusterNetwork == nil {
		return nil
	}

	return c.ClusterConfig
}

// K8sFlannelCNIConfig implements the config.Config interface.
func (c *Config) K8sFlannelCNIConfig() config.K8sFlannelCNIConfig {
	// if the section is missing, assume it's not set (multi-doc should provide it)
	if c.ClusterConfig == nil || c.ClusterConfig.ClusterNetwork == nil {
		return nil
	}

	cniConfig := c.ClusterConfig.CNI()

	// if CNI is not Flannel, assume it is disabled
	if cniConfig.CNIName != constants.FlannelCNI {
		return nil
	}

	return cniConfig.Flannel()
}

// K8sAdmissionControlPluginConfigs implements the config.Config interface.
func (c *Config) K8sAdmissionControlPluginConfigs() []config.K8sAdmissionControlPluginConfig {
	if c.ClusterConfig == nil || c.ClusterConfig.APIServerConfig == nil {
		return nil
	}

	return xslices.Map(
		c.ClusterConfig.APIServerConfig.AdmissionControlConfig,
		func(pluginConfig *AdmissionPluginConfig) config.K8sAdmissionControlPluginConfig {
			return pluginConfig
		},
	)
}

// K8sAuditPolicyConfig implements the config.Config interface.
func (c *Config) K8sAuditPolicyConfig() config.K8sAuditPolicyConfig {
	if c.ClusterConfig == nil || c.ClusterConfig.APIServerConfig == nil {
		return auditPolicyConfigShim{APIServerConfig: &APIServerConfig{}}
	}

	return auditPolicyConfigShim{APIServerConfig: c.ClusterConfig.APIServerConfig}
}

type auditPolicyConfigShim struct {
	*APIServerConfig
}

// K8sAuditPolicyConfigSignal implements the config.K8sAuditPolicyConfig interface.
func (s auditPolicyConfigShim) K8sAuditPolicyConfigSignal() {}

// Configuration implements the config.K8sAuditPolicyConfig interface.
func (s auditPolicyConfigShim) Configuration() map[string]any {
	return s.AuditPolicy()
}

// K8sAuthorizerConfigs implements the config.APIServer interface.
func (c *Config) K8sAuthorizerConfigs() []config.K8sAuthorizerConfig {
	var apiServerConfig *APIServerConfig

	if c.ClusterConfig == nil || c.ClusterConfig.APIServerConfig == nil {
		apiServerConfig = &APIServerConfig{}
	} else {
		apiServerConfig = c.ClusterConfig.APIServerConfig
	}

	return xslices.Map(
		apiServerConfig.AuthorizationConfigConfig,
		func(c *AuthorizationConfigAuthorizerConfig) config.K8sAuthorizerConfig { return c },
	)
}

// K8sCoreDNSConfig implements the config.K8sCoreDNSConfig interface.
func (c *Config) K8sCoreDNSConfig() config.K8sCoreDNSConfig {
	if c.ClusterConfig == nil || c.ClusterConfig.CoreDNSConfig == nil {
		return &CoreDNS{}
	}

	return c.ClusterConfig.CoreDNSConfig
}

// K8sServiceAccountConfig implements the config.K8sServiceAccountConfig interface.
func (c *Config) K8sServiceAccountConfig() config.K8sServiceAccountConfig {
	if c.ClusterConfig == nil || c.ClusterConfig.ClusterServiceAccount == nil {
		return nil
	}

	return serviceAccountShim{
		endpoint: c.ClusterConfig.Endpoint(),
		key:      c.ClusterConfig.ClusterServiceAccount,
	}
}

type serviceAccountShim struct {
	endpoint *url.URL
	key      *x509.PEMEncodedKey
}

// K8sServiceAccountConfigSignal implements the config.K8sServiceAccountConfig interface.
func (s serviceAccountShim) K8sServiceAccountConfigSignal() {}

// IssuingKey implements the config.K8sServiceAccountConfig interface.
func (s serviceAccountShim) IssuingKey() *x509.PEMEncodedKey {
	return s.key
}

// AcceptedKeys implements the config.K8sServiceAccountConfig interface.
func (s serviceAccountShim) AcceptedKeys() []*x509.PEMEncodedKey {
	issuingKey, err := s.IssuingKey().GetKey()
	if err != nil {
		// legacy config doesn't fully validate the key, and the actual failure
		// was previously deferred to the controllers, so here return a broken
		// config
		//
		// no actually working config can have a broken service account key
		return nil
	}

	return []*x509.PEMEncodedKey{
		{
			Key: issuingKey.GetPublicKeyPEM(),
		},
	}
}

// IssuerURL implements the config.K8sServiceAccountConfig interface.
func (s serviceAccountShim) IssuerURL() string {
	return s.endpoint.String()
}

// AcceptedIssuers implements the config.K8sServiceAccountConfig interface.
func (s serviceAccountShim) AcceptedIssuers() []string {
	return nil
}

// APIAudiences implements the config.K8sServiceAccountConfig interface.
func (s serviceAccountShim) APIAudiences() []string {
	return []string{s.endpoint.String()}
}

// K8sClusterConfig implements the config.Config interface.
func (c *Config) K8sClusterConfig() config.K8sClusterConfig {
	if c.ClusterConfig == nil || c.ClusterConfig.ControlPlane == nil {
		return nil
	}

	return k8sClusterConfigShim{
		c: c.ClusterConfig,
	}
}

type k8sClusterConfigShim struct {
	c *ClusterConfig
}

// ClusterName implements the config.K8sClusterConfig interface.
func (s k8sClusterConfigShim) ClusterName() string {
	return s.c.Name()
}

// ClusterEndpoint implements the config.K8sClusterConfig interface.
func (s k8sClusterConfigShim) ClusterEndpoint() *url.URL {
	return s.c.Endpoint()
}

// K8sNodeConfig implements the config.Config interface.
func (c *Config) K8sNodeConfig() config.K8sNodeConfig {
	if c.MachineConfig == nil && c.ClusterConfig == nil {
		return nil
	}

	var (
		machineConfig *MachineConfig
		kubeletConfig *KubeletConfig
		clusterConfig *ClusterConfig
	)

	if c.MachineConfig != nil {
		machineConfig = c.MachineConfig
	} else {
		machineConfig = &MachineConfig{}
	}

	if machineConfig.MachineKubelet != nil {
		kubeletConfig = machineConfig.MachineKubelet
	} else {
		kubeletConfig = &KubeletConfig{}
	}

	if c.ClusterConfig != nil {
		clusterConfig = c.ClusterConfig
	} else {
		clusterConfig = &ClusterConfig{}
	}

	return k8sNodeConfigShim{
		machineConfig: machineConfig,
		kubeletConfig: kubeletConfig,
		clusterConfig: clusterConfig,
	}
}

type k8sNodeConfigShim struct {
	machineConfig *MachineConfig
	kubeletConfig *KubeletConfig
	clusterConfig *ClusterConfig
}

// SkipNodeRegistration implements the config.K8sNodeConfig interface.
func (s k8sNodeConfigShim) SkipNodeRegistration() bool {
	return pointer.SafeDeref(s.kubeletConfig.KubeletSkipNodeRegistration)
}

// RegisterWithFQDN implements the config.K8sNodeConfig interface.
func (s k8sNodeConfigShim) RegisterWithFQDN() bool {
	return pointer.SafeDeref(s.kubeletConfig.KubeletRegisterWithFQDN)
}

// NodeIP implements the config.K8sNodeConfig interface.
func (s k8sNodeConfigShim) NodeIP() config.K8sNodeIPConfig {
	return pointer.SafeDeref(s.kubeletConfig.KubeletNodeIP)
}

// ValidSubnets implements the config.K8sNodeIPConfig interface.
func (c KubeletNodeIPConfig) ValidSubnets() []string {
	return c.KubeletNodeIPValidSubnets
}

// Labels implements the config.K8sNodeConfig interface.
func (s k8sNodeConfigShim) Labels() map[string]string {
	l := s.machineConfig.MachineNodeLabels

	if s.machineConfig.Type().IsControlPlane() {
		if l == nil {
			l = map[string]string{}
		} else {
			l = maps.Clone(l)
		}

		l[constants.LabelNodeRoleControlPlane] = ""
	}

	return l
}

// Taints implements the config.K8sNodeConfig interface.
func (s k8sNodeConfigShim) Taints() map[string]string {
	t := s.machineConfig.MachineNodeTaints

	if s.machineConfig.Type().IsControlPlane() {
		if !s.clusterConfig.ScheduleOnControlPlanes() {
			if t == nil {
				t = map[string]string{}
			} else {
				t = maps.Clone(t)
			}

			t[constants.LabelNodeRoleControlPlane] = constants.TaintEffectNoSchedule
		}
	}

	return t
}

// Annotations implements the config.K8sNodeConfig interface.
func (s k8sNodeConfigShim) Annotations() map[string]string {
	return s.machineConfig.MachineNodeAnnotations
}

// K8sKubeletConfig implements the config.Config interface.
func (c *Config) K8sKubeletConfig() config.K8sKubeletConfig {
	if c.MachineConfig == nil {
		return nil
	}

	if c.MachineConfig.MachineKubelet == nil {
		return &KubeletConfig{}
	}

	return c.MachineConfig.MachineKubelet
}

// K8sCredentialProviderConfig implements the config.Config interface.
func (c *Config) K8sCredentialProviderConfig() config.K8sCredentialProviderConfig {
	if c.MachineConfig == nil || c.MachineConfig.MachineKubelet == nil {
		return nil
	}

	return k8sCredentialProviderConfigShim{
		kubeletConfig: c.MachineConfig.MachineKubelet,
	}
}

type k8sCredentialProviderConfigShim struct {
	kubeletConfig *KubeletConfig
}

// K8sCredentialProviderConfigSignal implements config.K8sCredentialProviderConfig interface.
func (s k8sCredentialProviderConfigShim) K8sCredentialProviderConfigSignal() {}

// Configuration implements config.K8sCredentialProviderConfig interface.
func (s k8sCredentialProviderConfigShim) Configuration() map[string]any {
	return s.kubeletConfig.CredentialProviderConfig()
}

// K8sStaticPodConfigs implements the config.Config interface.
func (c *Config) K8sStaticPodConfigs() []config.K8sStaticPodConfig {
	if c.MachineConfig == nil {
		return nil
	}

	return xslices.Map(
		c.MachineConfig.MachinePods,
		func(u meta.Unstructured) config.K8sStaticPodConfig {
			return kubeStaticPodShim{obj: u.Object}
		},
	)
}

type kubeStaticPodShim struct {
	obj map[string]any
}

func (s kubeStaticPodShim) K8sStaticPodConfigSignal() {}

func (s kubeStaticPodShim) Name() string {
	// the v1alpha1 config doesn't have a name, so we try to synthesize the name
	// of out pod's namespace and name
	//
	// if we fail to do so, we replace each component with 'default' as best effort
	name := "default"
	namespace := "default"

	if metadata, ok := s.obj["metadata"].(map[string]any); ok {
		if n, ok := metadata["name"].(string); ok {
			name = n
		}

		if ns, ok := metadata["namespace"].(string); ok {
			namespace = ns
		}
	}

	return namespace + "-" + name
}

func (s kubeStaticPodShim) Pod() map[string]any {
	return s.obj
}

// K8sInlineManifestConfigs implements the config.Config interface.
func (c *Config) K8sInlineManifestConfigs() []config.K8sInlineManifestConfig {
	if c.ClusterConfig == nil {
		return nil
	}

	return xslices.Map(
		c.ClusterConfig.ClusterInlineManifests,
		func(m ClusterInlineManifest) config.K8sInlineManifestConfig { return m },
	)
}

// K8sExternalManifestConfigs implements the config.Config interface.
func (c *Config) K8sExternalManifestConfigs() []config.K8sExternalManifestConfig {
	if c.ClusterConfig == nil {
		return nil
	}

	return xslices.Map(
		c.ClusterConfig.ExtraManifestURLs(),
		func(u string) config.K8sExternalManifestConfig {
			return kubeExternalManifestShim{
				url:     u,
				headers: c.ClusterConfig.ExtraManifestHeaderMap(),
			}
		},
	)
}

type kubeExternalManifestShim struct {
	url     string
	headers map[string]string
}

func (s kubeExternalManifestShim) K8sExternalManifestConfigSignal() {}

func (s kubeExternalManifestShim) URL() string {
	return s.url
}

func (s kubeExternalManifestShim) Headers() map[string]string {
	return s.headers
}

func (s kubeExternalManifestShim) Name() string {
	return s.url
}

// K8sKubePrismConfig implements the config.Config interface.
func (c *Config) K8sKubePrismConfig() config.K8sKubePrismConfig {
	if c.MachineConfig == nil || c.MachineConfig.MachineFeatures == nil {
		return nil
	}

	kubePrismConfig := c.MachineConfig.MachineFeatures.KubePrismSupport
	if kubePrismConfig == nil || !kubePrismConfig.Enabled() {
		return nil
	}

	return kubePrismConfig
}

// K8sTalosAPIAccessConfig implements the config.Config interface.
func (c *Config) K8sTalosAPIAccessConfig() config.K8sTalosAPIAccessConfig {
	if c.MachineConfig == nil || c.MachineConfig.MachineFeatures == nil {
		return nil
	}

	talosAPIAccessConfig := c.MachineConfig.MachineFeatures.KubernetesTalosAPIAccessConfig
	if talosAPIAccessConfig == nil || !talosAPIAccessConfig.Enabled() {
		return nil
	}

	return talosAPIAccessConfig
}
