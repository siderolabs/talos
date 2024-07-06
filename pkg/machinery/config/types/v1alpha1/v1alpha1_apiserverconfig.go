// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1

import (
	"fmt"

	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/go-pointer"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// APIServerDefaultAuditPolicy is the default kube-apiserver audit policy.
var APIServerDefaultAuditPolicy = Unstructured{
	Object: map[string]any{
		"apiVersion": "audit.k8s.io/v1",
		"kind":       "Policy",
		"rules": []any{
			map[string]any{
				"level": "Metadata",
			},
		},
	},
}

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
	return xslices.Map(a.ExtraVolumesConfig, func(v VolumeMountConfig) config.VolumeMount { return v })
}

// Env implements the config.APIServer interface.
func (a *APIServerConfig) Env() Env {
	return a.EnvConfig
}

// DisablePodSecurityPolicy implements the config.APIServer interface.
func (a *APIServerConfig) DisablePodSecurityPolicy() bool {
	return pointer.SafeDeref(a.DisablePodSecurityPolicyConfig)
}

// AdmissionControl implements the config.APIServer interface.
func (a *APIServerConfig) AdmissionControl() []config.AdmissionPlugin {
	return xslices.Map(a.AdmissionControlConfig, func(c *AdmissionPluginConfig) config.AdmissionPlugin { return c })
}

// AuditPolicy implements the config.APIServer interface.
func (a *APIServerConfig) AuditPolicy() map[string]any {
	if len(a.AuditPolicyConfig.Object) == 0 {
		return APIServerDefaultAuditPolicy.DeepCopy().Object
	}

	return a.AuditPolicyConfig.Object
}

// Resources implements the config.Resources interface.
func (a *APIServerConfig) Resources() config.Resources {
	return a.ResourcesConfig
}

// Validate performs config validation.
func (a *APIServerConfig) Validate() error {
	if a == nil {
		return nil
	}

	if err := a.ResourcesConfig.Validate(); err != nil {
		return fmt.Errorf("apiserver resource validation failed: %w", err)
	}

	return nil
}
