// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1

import (
	"fmt"

	"github.com/talos-systems/talos/pkg/machinery/config"
	"github.com/talos-systems/talos/pkg/machinery/constants"
)

// Image implements the config.APIServer interface.
func (a *APIServerConfig) Image() string {
	image := a.ContainerImage

	if image == "" {
		image = fmt.Sprintf("%s:v%s", constants.KubernetesAPIServerImage, constants.DefaultKubernetesVersion)
	}

	return image
}

// ExtraArgs implements the config.APIServer interface.
func (a *APIServerConfig) ExtraArgs() map[string]string {
	return a.ExtraArgsConfig
}

// ExtraVolumes implements the config.APIServer interface.
func (a *APIServerConfig) ExtraVolumes() []config.VolumeMount {
	volumes := make([]config.VolumeMount, 0, len(a.ExtraVolumesConfig))

	for _, volume := range a.ExtraVolumesConfig {
		volumes = append(volumes, volume)
	}

	return volumes
}
