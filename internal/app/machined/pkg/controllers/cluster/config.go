// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cluster

import (
	"context"
	"fmt"

	"github.com/AlekSi/pointer"
	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"

	"github.com/talos-systems/talos/pkg/resources/cluster"
	"github.com/talos-systems/talos/pkg/resources/config"
)

// ConfigController watches v1alpha1.Config, updates discovery config.
type ConfigController struct{}

// Name implements controller.Controller interface.
func (ctrl *ConfigController) Name() string {
	return "cluster.ConfigController"
}

// Inputs implements controller.Controller interface.
func (ctrl *ConfigController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: config.NamespaceName,
			Type:      config.MachineConfigType,
			ID:        pointer.ToString(config.V1Alpha1ID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *ConfigController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: cluster.ConfigType,
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
			cfg, err := r.Get(ctx, resource.NewMetadata(config.NamespaceName, config.MachineConfigType, config.V1Alpha1ID, resource.VersionUndefined))
			if err != nil {
				if !state.IsNotFoundError(err) {
					return fmt.Errorf("error getting config: %w", err)
				}
			}

			touchedIDs := make(map[resource.ID]struct{})

			if cfg != nil {
				c := cfg.(*config.MachineConfig).Config()

				if err = r.Modify(ctx, cluster.NewConfig(config.NamespaceName, cluster.ConfigID), func(res resource.Resource) error {
					res.(*cluster.Config).TypedSpec().DiscoveryEnabled = c.Cluster().Discovery().Enabled()

					return nil
				}); err != nil {
					return err
				}

				touchedIDs[cluster.ConfigID] = struct{}{}
			}

			// list keys for cleanup
			list, err := r.List(ctx, resource.NewMetadata(config.NamespaceName, cluster.ConfigType, "", resource.VersionUndefined))
			if err != nil {
				return fmt.Errorf("error listing resources: %w", err)
			}

			for _, res := range list.Items {
				if res.Metadata().Owner() != ctrl.Name() {
					continue
				}

				if _, ok := touchedIDs[res.Metadata().ID()]; !ok {
					if err = r.Destroy(ctx, res.Metadata()); err != nil {
						return fmt.Errorf("error cleaning up specs: %w", err)
					}
				}
			}
		}
	}
}
