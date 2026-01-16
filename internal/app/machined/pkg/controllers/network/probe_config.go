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

	talosconfig "github.com/siderolabs/talos/pkg/machinery/config"
	networkconfig "github.com/siderolabs/talos/pkg/machinery/config/types/network"
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

		cfgProvider, err := ctrl.getConfigProvider(ctx, r)
		if err != nil {
			return err
		}

		var specs []network.ProbeSpecSpec

		// parse machine configuration for probe config documents
		if cfgProvider != nil {
			configSpecs := ctrl.parseMachineConfiguration(cfgProvider, logger)
			specs = append(specs, configSpecs...)
		}

		ids, err := ctrl.apply(ctx, r, specs)
		if err != nil {
			return fmt.Errorf("error applying specs: %w", err)
		}

		touchedIDs := make(map[resource.ID]struct{}, len(ids))
		for _, id := range ids {
			touchedIDs[id] = struct{}{}
		}

		if err := ctrl.cleanup(ctx, r, touchedIDs); err != nil {
			return err
		}

		r.ResetRestartBackoff()
	}
}

func (ctrl *ProbeConfigController) getConfigProvider(ctx context.Context, r controller.Runtime) (talosconfig.Provider, error) {
	cfg, err := safe.ReaderGetByID[*config.MachineConfig](ctx, r, config.ActiveID)
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil, nil
		}

		return nil, fmt.Errorf("error getting config: %w", err)
	}

	// MachineConfig.Config() returns config.Config, but it's actually a Provider (container)
	if provider, ok := cfg.Config().(talosconfig.Provider); ok {
		return provider, nil
	}

	return nil, nil
}

func (ctrl *ProbeConfigController) cleanup(ctx context.Context, r controller.Runtime, touchedIDs map[resource.ID]struct{}) error {
	list, err := r.List(ctx, resource.NewMetadata(network.NamespaceName, network.ProbeSpecType, "", resource.VersionUndefined))
	if err != nil {
		return fmt.Errorf("error listing resources: %w", err)
	}

	for _, res := range list.Items {
		if res.Metadata().Owner() != ctrl.Name() {
			// skip specs created by other controllers
			continue
		}

		if _, ok := touchedIDs[res.Metadata().ID()]; !ok {
			if err = r.Destroy(ctx, res.Metadata()); err != nil {
				return fmt.Errorf("error cleaning up specs: %w", err)
			}
		}
	}

	return nil
}

func (ctrl *ProbeConfigController) apply(ctx context.Context, r controller.Runtime, specs []network.ProbeSpecSpec) ([]resource.ID, error) {
	ids := make([]string, 0, len(specs))

	for _, spec := range specs {
		id, err := spec.ID()
		if err != nil {
			return ids, fmt.Errorf("error getting probe spec ID: %w", err)
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
			return ids, err
		}

		ids = append(ids, id)
	}

	return ids, nil
}

func (ctrl *ProbeConfigController) parseMachineConfiguration(cfgProvider talosconfig.Provider, logger *zap.Logger) []network.ProbeSpecSpec {
	docs := cfgProvider.Documents()
	specs := make([]network.ProbeSpecSpec, 0, len(docs))

	for _, doc := range docs {
		if doc.Kind() != networkconfig.ProbeKind {
			continue
		}

		probeConfig, ok := doc.(*networkconfig.ProbeConfigV1Alpha1)
		if !ok {
			logger.Warn("unexpected probe config document type", zap.String("type", fmt.Sprintf("%T", doc)))

			continue
		}

		spec := network.ProbeSpecSpec{
			Interval:         probeConfig.ProbeInterval,
			FailureThreshold: probeConfig.FailureThreshold,
			ConfigLayer:      network.ConfigMachineConfiguration,
		}

		if probeConfig.TCP != nil {
			spec.TCP = network.TCPProbeSpec{
				Endpoint: probeConfig.TCP.Endpoint,
				Timeout:  probeConfig.TCP.Timeout,
			}
		}

		specs = append(specs, spec)
	}

	return specs
}
