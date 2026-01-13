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

// Image implements the config.ControllerManager interface.
func (c *ControllerManagerConfig) Image() string {
	image := c.ContainerImage

	if image == "" {
		image = fmt.Sprintf("%s:v%s", constants.KubernetesControllerManagerImage, constants.DefaultKubernetesVersion)
	}

	return image
}

// ExtraArgs implements the config.ControllerManager interface.
func (c *ControllerManagerConfig) ExtraArgs() map[string][]string {
	return c.ExtraArgsConfig.ToMap()
}

// ExtraVolumes implements the config.ControllerManager interface.
func (c *ControllerManagerConfig) ExtraVolumes() []config.VolumeMount {
	return xslices.Map(c.ExtraVolumesConfig, func(v VolumeMountConfig) config.VolumeMount { return v })
}

// Env implements the config.ControllerManager interface.
func (c *ControllerManagerConfig) Env() Env {
	return c.EnvConfig
}

// Resources implements the config.Resources interface.
func (c *ControllerManagerConfig) Resources() config.Resources {
	return c.ResourcesConfig
}

// Validate performs config validation.
func (c *ControllerManagerConfig) Validate() error {
	if c == nil {
		return nil
	}

	if err := c.ResourcesConfig.Validate(); err != nil {
		return fmt.Errorf("controller-manager resource validation failed: %w", err)
	}

	return nil
}
