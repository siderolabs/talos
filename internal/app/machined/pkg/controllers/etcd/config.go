// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package etcd

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/go-pointer"
	"go.uber.org/zap"

	"github.com/talos-systems/talos/pkg/machinery/resources/config"
	"github.com/talos-systems/talos/pkg/machinery/resources/etcd"
)

// ConfigController renders manifests based on templates and config/secrets.
type ConfigController struct{}

// Name implements controller.Controller interface.
func (ctrl *ConfigController) Name() string {
	return "etcd.ConfigController"
}

// Inputs implements controller.Controller interface.
func (ctrl *ConfigController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: config.NamespaceName,
			Type:      config.MachineConfigType,
			ID:        pointer.To(config.V1Alpha1ID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: config.NamespaceName,
			Type:      config.MachineTypeType,
			ID:        pointer.To(config.MachineTypeID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *ConfigController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: etcd.ConfigType,
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

		machineType, err := safe.ReaderGet[*config.MachineType](ctx, r, resource.NewMetadata(config.NamespaceName, config.MachineTypeType, config.MachineTypeID, resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return fmt.Errorf("error getting machine config: %w", err)
		}

		if !machineType.MachineType().IsControlPlane() {
			if err = ctrl.teardownAll(ctx, r); err != nil {
				return err
			}

			continue
		}

		machineConfig, err := safe.ReaderGet[*config.MachineConfig](ctx, r, resource.NewMetadata(config.NamespaceName, config.MachineConfigType, config.V1Alpha1ID, resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return fmt.Errorf("error getting machine config: %w", err)
		}

		if err = safe.WriterModify(ctx, r, etcd.NewConfig(etcd.NamespaceName, etcd.ConfigID), func(status *etcd.Config) error {
			status.TypedSpec().AdvertiseValidSubnets = machineConfig.Config().Cluster().Etcd().AdvertisedSubnets()
			status.TypedSpec().ListenValidSubnets = machineConfig.Config().Cluster().Etcd().ListenSubnets()

			status.TypedSpec().Image = machineConfig.Config().Cluster().Etcd().Image()
			status.TypedSpec().ExtraArgs = machineConfig.Config().Cluster().Etcd().ExtraArgs()

			return nil
		}); err != nil {
			return fmt.Errorf("error updating Config status: %w", err)
		}
	}
}

func (ctrl *ConfigController) teardownAll(ctx context.Context, r controller.Runtime) error {
	list, err := r.List(ctx, resource.NewMetadata(etcd.NamespaceName, etcd.ConfigType, "", resource.VersionUndefined))
	if err != nil {
		return err
	}

	for _, res := range list.Items {
		if err = r.Destroy(ctx, res.Metadata()); err != nil {
			return err
		}
	}

	return nil
}
