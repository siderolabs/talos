// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	configconfig "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// ProbeConfigController manages network.ProbeSpec based on ProbeConfig documents in machine configuration.
type ProbeConfigController struct{}

// Name implements controller.Controller interface.
func (ctrl *ProbeConfigController) Name() string {
	return "network.ProbeConfigController"
}

// Inputs implements controller.Controller interface.
func (ctrl *ProbeConfigController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: config.NamespaceName,
			Type:      config.MachineConfigType,
			ID:        optional.Some(config.ActiveID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *ProbeConfigController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: network.ProbeSpecType,
			Kind: controller.OutputShared,
		},
	}
}

// Run implements controller.Controller interface.
func (ctrl *ProbeConfigController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		r.StartTrackingOutputs()

		cfg, err := safe.ReaderGetByID[*config.MachineConfig](ctx, r, config.ActiveID)
		if err != nil {
			if !state.IsNotFoundError(err) {
				return fmt.Errorf("error getting config: %w", err)
			}
		}

		var specs []network.ProbeSpecSpec

		// parse machine configuration for probe config documents
		if cfg != nil {
			configSpecs := ctrl.parseMachineConfiguration(cfg)
			specs = append(specs, configSpecs...)
		}

		if err = ctrl.apply(ctx, r, specs); err != nil {
			return fmt.Errorf("error applying specs: %w", err)
		}

		if err = r.CleanupOutputs(ctx,
			resource.NewMetadata(network.NamespaceName, network.ProbeSpecType, "", resource.VersionUndefined),
		); err != nil {
			return fmt.Errorf("error cleaning up outputs: %w", err)
		}

		r.ResetRestartBackoff()
	}
}

func (ctrl *ProbeConfigController) apply(ctx context.Context, r controller.Runtime, specs []network.ProbeSpecSpec) error {
	for _, spec := range specs {
		id, err := spec.ID()
		if err != nil {
			return fmt.Errorf("error getting probe spec ID: %w", err)
		}

		if err := safe.WriterModify(
			ctx,
			r,
			network.NewProbeSpec(network.NamespaceName, id),
			func(r *network.ProbeSpec) error {
				*r.TypedSpec() = spec

				return nil
			},
		); err != nil {
			return fmt.Errorf("error modifying probe spec: %w", err)
		}
	}

	return nil
}

func (ctrl *ProbeConfigController) parseMachineConfiguration(cfg *config.MachineConfig) []network.ProbeSpecSpec {
	probeConfigs := cfg.Config().NetworkProbeConfigs()
	specs := make([]network.ProbeSpecSpec, 0, len(probeConfigs))

	for _, probeConfig := range probeConfigs {
		spec := network.ProbeSpecSpec{
			Interval:         probeConfig.Interval(),
			FailureThreshold: probeConfig.FailureThreshold(),
			ConfigLayer:      network.ConfigMachineConfiguration,
		}

		switch probeConfig := probeConfig.(type) {
		case configconfig.NetworkTCPProbeConfig:
			spec.TCP = network.TCPProbeSpec{
				Endpoint: probeConfig.Endpoint(),
				Timeout:  probeConfig.Timeout(),
			}
		default:
			panic(fmt.Sprintf("unsupported probe config type: %T", probeConfig))
		}

		specs = append(specs, spec)
	}

	return specs
}
