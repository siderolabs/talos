// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1

import (
	"github.com/siderolabs/go-pointer"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
)

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
