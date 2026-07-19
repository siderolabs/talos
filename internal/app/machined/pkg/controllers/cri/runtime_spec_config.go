// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cri

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/containerd/containerd/v2/core/containers"
	"github.com/containerd/containerd/v2/pkg/namespaces"
	"github.com/containerd/containerd/v2/pkg/oci"
	"github.com/containerd/platforms"
	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/cri"
)

// RuntimeSpecConfigController projects OCI runtime spec configuration sources.
type RuntimeSpecConfigController struct{}

// Name implements controller.Controller interface.
func (ctrl *RuntimeSpecConfigController) Name() string {
	return "cri.RuntimeSpecConfigController"
}

// Inputs implements controller.Controller interface.
func (ctrl *RuntimeSpecConfigController) Inputs() []controller.Input {
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
func (ctrl *RuntimeSpecConfigController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: cri.BaseRuntimeSpecConfigType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
func (ctrl *RuntimeSpecConfigController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		r.StartTrackingOutputs()

		if err := ctrl.reconcile(ctx, r); err != nil {
			return err
		}

		if err := safe.CleanupOutputs[*cri.BaseRuntimeSpecConfig](ctx, r); err != nil {
			return fmt.Errorf("failed to clean up outputs: %w", err)
		}
	}
}

func (ctrl *RuntimeSpecConfigController) reconcile(ctx context.Context, r controller.Runtime) error {
	cfg, err := safe.ReaderGetByID[*config.MachineConfig](ctx, r, config.ActiveID)
	if err != nil && !state.IsNotFoundError(err) {
		return fmt.Errorf("failed to get machine config: %w", err)
	}

	defaultSpec, err := generateDefaultRuntimeSpec(ctx)
	if err != nil {
		return err
	}

	if err = writeRuntimeSpecConfig(ctx, r, cri.BaseRuntimeSpecDefaultID, defaultSpec); err != nil {
		return err
	}

	if configuredSpec := configuredRuntimeSpec(cfg); len(configuredSpec) > 0 {
		if err = writeRuntimeSpecConfig(ctx, r, cri.BaseRuntimeSpecOverridesID, configuredSpec); err != nil {
			return err
		}
	}

	return nil
}

func configuredRuntimeSpec(cfg *config.MachineConfig) map[string]any {
	if cfg == nil {
		return nil
	}

	if document := cfg.Config().CRIBaseRuntimeSpecConfig(); document != nil && len(document.Overrides()) > 0 {
		return document.Overrides()
	}

	return nil
}

func generateDefaultRuntimeSpec(ctx context.Context) (map[string]any, error) {
	defaultSpec, err := oci.GenerateSpecWithPlatform(
		namespaces.WithNamespace(ctx, constants.K8sContainerdNamespace),
		nil,
		platforms.DefaultString(),
		&containers.Container{},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to generate default runtime spec: %w", err)
	}

	// compatibility with CRI defaults:
	// * remove default rlimits (See https://github.com/containerd/cri/issues/515)
	defaultSpec.Process.Rlimits = nil

	data, err := json.Marshal(defaultSpec)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal default runtime spec: %w", err)
	}

	result := map[string]any{}

	if err = json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal default runtime spec: %w", err)
	}

	return result, nil
}

func writeRuntimeSpecConfig(ctx context.Context, r controller.Runtime, id resource.ID, spec map[string]any) error {
	if err := safe.WriterModify(ctx, r, cri.NewBaseRuntimeSpecConfig(id), func(res *cri.BaseRuntimeSpecConfig) error {
		res.TypedSpec().Object = spec

		return nil
	}); err != nil {
		return fmt.Errorf("failed to write runtime spec config %q: %w", id, err)
	}

	return nil
}
