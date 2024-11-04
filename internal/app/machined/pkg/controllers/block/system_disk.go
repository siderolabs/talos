// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

// SystemDiskController provides a detailed view of blockdevices of type 'disk'.
type SystemDiskController struct{}

// Name implements controller.Controller interface.
func (ctrl *SystemDiskController) Name() string {
	return "block.SystemDiskController"
}

// Inputs implements controller.Controller interface.
func (ctrl *SystemDiskController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: block.NamespaceName,
			Type:      block.DiscoveredVolumeType,
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *SystemDiskController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: block.SystemDiskType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *SystemDiskController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	for {
		select {
		case <-r.EventCh():
		case <-ctx.Done():
			return nil
		}

		discoveredVolumes, err := safe.ReaderListAll[*block.DiscoveredVolume](ctx, r)
		if err != nil {
			return fmt.Errorf("failed to list discovered volumes: %w", err)
		}

		var (
			systemDiskID   string
			systemDiskPath string
		)

		for volume := range discoveredVolumes.All() {
			if volume.TypedSpec().PartitionLabel == constants.MetaPartitionLabel {
				systemDiskID = volume.TypedSpec().Parent
				systemDiskPath = volume.TypedSpec().ParentDevPath

				break
			}
		}

		if systemDiskID != "" {
			if err = safe.WriterModify(ctx, r, block.NewSystemDisk(block.NamespaceName, block.SystemDiskID), func(d *block.SystemDisk) error {
				d.TypedSpec().DiskID = systemDiskID
				d.TypedSpec().DevPath = systemDiskPath

				return nil
			}); err != nil {
				return fmt.Errorf("failed to write system disk: %w", err)
			}
		} else {
			if err = r.Destroy(ctx, block.NewSystemDisk(block.NamespaceName, block.SystemDiskID).Metadata()); err != nil && !state.IsNotFoundError(err) {
				return fmt.Errorf("failed to destroy system disk: %w", err)
			}
		}
	}
}
