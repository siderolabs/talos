// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package files

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/containerd/containerd/v2/core/containers"
	"github.com/containerd/containerd/v2/pkg/namespaces"
	"github.com/containerd/containerd/v2/pkg/oci"
	"github.com/containerd/platforms"
	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/config/merge"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/files"
)

// CRIBaseRuntimeSpecController generates parts of the CRI config for base OCI runtime configuration.
type CRIBaseRuntimeSpecController struct{}

// Name implements controller.Controller interface.
func (ctrl *CRIBaseRuntimeSpecController) Name() string {
	return "files.CRIBaseRuntimeSpecController"
}

// Inputs implements controller.Controller interface.
func (ctrl *CRIBaseRuntimeSpecController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: config.NamespaceName,
			Type:      config.MachineConfigType,
			ID:        optional.Some(config.V1Alpha1ID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *CRIBaseRuntimeSpecController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: files.EtcFileSpecType,
			Kind: controller.OutputShared,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *CRIBaseRuntimeSpecController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		cfg, err := safe.ReaderGetByID[*config.MachineConfig](ctx, r, config.V1Alpha1ID)
		if err != nil {
			if state.IsNotFoundError(err) {
				// wait for machine config to be available
				continue
			}

			return fmt.Errorf("error getting machine config: %w", err)
		}

		if cfg.Config().Machine() == nil {
			// wait for machine config to be available
			continue
		}

		platform := platforms.DefaultString()

		defaultSpec, err := oci.GenerateSpecWithPlatform(
			namespaces.WithNamespace(ctx, constants.K8sContainerdNamespace),
			nil,
			platform,
			&containers.Container{},
		)
		if err != nil {
			return fmt.Errorf("error generating default spec: %w", err)
		}

		// compatibility with CRI defaults:
		// * remove default rlimits (See https://github.com/containerd/cri/issues/515)
		defaultSpec.Process.Rlimits = nil

		if len(cfg.Config().Machine().BaseRuntimeSpecOverrides()) > 0 {
			var overrides oci.Spec

			jsonOverrides, err := json.Marshal(cfg.Config().Machine().BaseRuntimeSpecOverrides())
			if err != nil {
				return fmt.Errorf("error marshaling runtime spec overrides: %w", err)
			}

			if err := json.Unmarshal(jsonOverrides, &overrides); err != nil {
				return fmt.Errorf("error unmarshaling runtime spec overrides: %w", err)
			}

			if err := merge.Merge(defaultSpec, &overrides); err != nil {
				return fmt.Errorf("error merging runtime spec overrides: %w", err)
			}
		}

		contents, err := json.Marshal(defaultSpec)
		if err != nil {
			return fmt.Errorf("error marshaling runtime spec: %w", err)
		}

		if err := safe.WriterModify(ctx, r, files.NewEtcFileSpec(files.NamespaceName, constants.CRIBaseRuntimeSpec),
			func(r *files.EtcFileSpec) error {
				spec := r.TypedSpec()

				spec.Contents = contents
				spec.Mode = 0o600
				spec.SelinuxLabel = constants.EtcSelinuxLabel

				return nil
			}); err != nil {
			return fmt.Errorf("error modifying resource: %w", err)
		}

		r.ResetRestartBackoff()
	}
}
