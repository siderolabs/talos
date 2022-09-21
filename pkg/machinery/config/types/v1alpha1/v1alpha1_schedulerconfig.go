// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1

import (
	"fmt"

	"github.com/siderolabs/gen/slices"

	"github.com/talos-systems/talos/pkg/machinery/config"
	"github.com/talos-systems/talos/pkg/machinery/constants"
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
func (s *SchedulerConfig) ExtraArgs() map[string]string {
	return s.ExtraArgsConfig
}

// ExtraVolumes implements the config.Scheduler interface.
func (s *SchedulerConfig) ExtraVolumes() []config.VolumeMount {
	return slices.Map(s.ExtraVolumesConfig, func(v VolumeMountConfig) config.VolumeMount { return v })
}

// Env implements the config.Scheduler interface.
func (s *SchedulerConfig) Env() Env {
	return s.EnvConfig
}
