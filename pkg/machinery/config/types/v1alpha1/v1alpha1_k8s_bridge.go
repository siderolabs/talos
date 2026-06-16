// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1

import (
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/go-pointer"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

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
