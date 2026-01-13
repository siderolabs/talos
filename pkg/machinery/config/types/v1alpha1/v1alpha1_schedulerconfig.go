// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1

import (
	"fmt"

	"github.com/siderolabs/gen/xslices"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// Image implements the config.Scheduler interface.
func (s *SchedulerConfig) Image() string {
	image := s.ContainerImage

	if image == "" {
		image = fmt.Sprintf("%s:v%s", constants.KubernetesSchedulerImage, constants.DefaultKubernetesVersion)
	}

	return image
}

// ExtraArgs implements the config.Scheduler interface.
func (s *SchedulerConfig) ExtraArgs() map[string][]string {
	return s.ExtraArgsConfig.ToMap()
}

// ExtraVolumes implements the config.Scheduler interface.
func (s *SchedulerConfig) ExtraVolumes() []config.VolumeMount {
	return xslices.Map(s.ExtraVolumesConfig, func(v VolumeMountConfig) config.VolumeMount { return v })
}

// Env implements the config.Scheduler interface.
func (s *SchedulerConfig) Env() Env {
	return s.EnvConfig
}

// Resources implements the config.Resources interface.
func (s *SchedulerConfig) Resources() config.Resources {
	return s.ResourcesConfig
}

// Config implements the config.Scheduler interface.
func (s *SchedulerConfig) Config() map[string]any {
	return s.SchedulerConfig.Object
}

// Validate performs config validation.
func (s *SchedulerConfig) Validate() error {
	if s == nil {
		return nil
	}

	if err := s.ResourcesConfig.Validate(); err != nil {
		return fmt.Errorf("scheduler resource validation failed: %w", err)
	}

	return nil
}
