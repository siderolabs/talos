// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubeaccess

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/go-pointer"
	"go.uber.org/zap"

	"github.com/talos-systems/talos/pkg/machinery/resources/config"
	"github.com/talos-systems/talos/pkg/machinery/resources/kubeaccess"
)

// ConfigController watches v1alpha1.Config, updates Talos API access config.
type ConfigController struct{}

// Name implements controller.Controller interface.
func (ctrl *ConfigController) Name() string {
	return "kubeaccess.ConfigController"
}

// Inputs implements controller.Controller interface.
func (ctrl *ConfigController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: config.NamespaceName,
			Type:      config.MachineTypeType,
			ID:        pointer.To(config.MachineTypeID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: config.NamespaceName,
			Type:      config.MachineConfigType,
			ID:        pointer.To(config.V1Alpha1ID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *ConfigController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: kubeaccess.ConfigType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *ConfigController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		machineType, err := r.Get(ctx, resource.NewMetadata(config.NamespaceName, config.MachineTypeType, config.MachineTypeID, resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return fmt.Errorf("error getting machine type: %w", err)
		}

		if !machineType.(*config.MachineType).MachineType().IsControlPlane() {
			if err = r.Destroy(ctx, kubeaccess.NewConfig(config.NamespaceName, kubeaccess.ConfigID).Metadata()); err != nil {
				if !state.IsNotFoundError(err) {
					return fmt.Errorf("error destroying kubeaccess config: %w", err)
				}
			}

			// not a control plane node, nothing to do
			continue
		}

		cfg, err := r.Get(ctx, resource.NewMetadata(config.NamespaceName, config.MachineConfigType, config.V1Alpha1ID, resource.VersionUndefined))
		if err != nil {
			if !state.IsNotFoundError(err) {
				return fmt.Errorf("error getting config: %w", err)
			}
		}

		if err = r.Modify(ctx, kubeaccess.NewConfig(config.NamespaceName, kubeaccess.ConfigID), func(res resource.Resource) error {
			spec := res.(*kubeaccess.Config).TypedSpec()

			*spec = kubeaccess.ConfigSpec{}

			if cfg != nil {
				c := cfg.(*config.MachineConfig).Config()

				spec.Enabled = c.Machine().Features().KubernetesTalosAPIAccess().Enabled()
				spec.AllowedAPIRoles = c.Machine().Features().KubernetesTalosAPIAccess().AllowedRoles()
				spec.AllowedKubernetesNamespaces = c.Machine().Features().KubernetesTalosAPIAccess().AllowedKubernetesNamespaces()
			}

			return nil
		}); err != nil {
			return err
		}
	}
}
