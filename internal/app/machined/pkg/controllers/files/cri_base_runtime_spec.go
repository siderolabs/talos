// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package files

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/containerd/containerd/v2/pkg/oci"
	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/config/merge"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/cri"
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
			Namespace: cri.NamespaceName,
			Type:      cri.BaseRuntimeSpecConfigType,
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

		configs, err := readRuntimeSpecConfigs(ctx, r)
		if err != nil {
			return err
		}

		if configs == nil {
			// wait for the generated defaults to be available
			continue
		}

		defaultSpec, err := mergeRuntimeSpecConfigs(configs)
		if err != nil {
			return err
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

func readRuntimeSpecConfigs(ctx context.Context, r controller.Reader) ([]*cri.BaseRuntimeSpecConfig, error) {
	configList, err := safe.ReaderListAll[*cri.BaseRuntimeSpecConfig](ctx, r)
	if err != nil {
		return nil, fmt.Errorf("error listing runtime spec configs: %w", err)
	}

	configs := safe.ToSlice(configList, func(cfg *cri.BaseRuntimeSpecConfig) *cri.BaseRuntimeSpecConfig { return cfg })

	if !slices.ContainsFunc(configs, func(cfg *cri.BaseRuntimeSpecConfig) bool {
		return cfg.Metadata().ID() == cri.BaseRuntimeSpecDefaultID
	}) {
		return nil, nil
	}

	slices.SortFunc(configs, func(left, right *cri.BaseRuntimeSpecConfig) int {
		return cmp.Compare(left.Metadata().ID(), right.Metadata().ID())
	})

	return configs, nil
}

func mergeRuntimeSpecConfigs(configs []*cri.BaseRuntimeSpecConfig) (*oci.Spec, error) {
	result := &oci.Spec{}

	for _, cfg := range configs {
		data, err := json.Marshal(cfg.TypedSpec().Object)
		if err != nil {
			return nil, fmt.Errorf("error marshaling runtime spec config %q: %w", cfg.Metadata().ID(), err)
		}

		var source oci.Spec

		if err = json.Unmarshal(data, &source); err != nil {
			return nil, fmt.Errorf("error unmarshaling runtime spec config %q: %w", cfg.Metadata().ID(), err)
		}

		if err = merge.Merge(result, &source); err != nil {
			return nil, fmt.Errorf("error merging runtime spec config %q: %w", cfg.Metadata().ID(), err)
		}
	}

	return result, nil
}
