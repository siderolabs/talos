// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/resources"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// MountStatusController transforms block.MountStatus resources into legacy v1alpha1.MountStatus.
//
// It only exists to provide backwards compatibility with legacy consumers.
type MountStatusController struct{}

// Name implements controller.Controller interface.
func (ctrl *MountStatusController) Name() string {
	return "runtime.MountStatusController"
}

// Inputs implements controller.Controller interface.
func (ctrl *MountStatusController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: resources.InMemoryNamespace,
			Type:      block.MountStatusType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: block.NamespaceName,
			Type:      block.VolumeStatusType,
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *MountStatusController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: runtime.MountStatusType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *MountStatusController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	for {
		select {
		case <-r.EventCh():
		case <-ctx.Done():
			return nil
		}

		mountStatuses, err := safe.ReaderListAll[*block.MountStatus](ctx, r)
		if err != nil {
			return fmt.Errorf("failed to read mount statuses: %w", err)
		}

		r.StartTrackingOutputs()

		for mountStatus := range mountStatuses.All() {
			volumeStatus, err := safe.ReaderGetByID[*block.VolumeStatus](ctx, r, mountStatus.TypedSpec().Spec.VolumeID)
			if err != nil && !state.IsNotFoundError(err) {
				return fmt.Errorf("failed to get volume status %q: %w", mountStatus.TypedSpec().Spec.VolumeID, err)
			}

			if volumeStatus.TypedSpec().Type != block.VolumeTypePartition && volumeStatus.TypedSpec().Type != block.VolumeTypeDisk {
				// legacy volume statuses shouldn't show up for non-partition/disk volumes
				continue
			}

			if err = safe.WriterModify(ctx, r, runtime.NewMountStatus(runtime.NamespaceName, volumeStatus.Metadata().ID()),
				func(res *runtime.MountStatus) error {
					res.TypedSpec().Source = mountStatus.TypedSpec().Source
					res.TypedSpec().Target = mountStatus.TypedSpec().Target
					res.TypedSpec().FilesystemType = volumeStatus.TypedSpec().Filesystem.String()
					res.TypedSpec().Encrypted = volumeStatus.TypedSpec().EncryptionProvider != block.EncryptionProviderNone
					res.TypedSpec().EncryptionProviders = volumeStatus.TypedSpec().ConfiguredEncryptionKeys

					return nil
				},
			); err != nil {
				return fmt.Errorf("failed to write mount status: %w", err)
			}
		}

		if err := safe.CleanupOutputs[*runtime.MountStatus](ctx, r); err != nil {
			return fmt.Errorf("failed to cleanup mount statuses: %w", err)
		}
	}
}
