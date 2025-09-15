// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block

import (
	"fmt"

	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/xfs"
	"github.com/siderolabs/talos/pkg/xfs/opentree"
)

// VolumeMountStatus adapter provides conversion from MountStatus.
//
//nolint:revive,golint
func VolumeMountStatus(r *block.VolumeMountStatus) volumeMountStatus {
	return volumeMountStatus{
		VolumeMountStatus: r,
	}
}

type volumeMountStatus struct {
	VolumeMountStatus *block.VolumeMountStatus
}

// WithRoot adapts VolumeMountStatus to xfs.Root and calls the provided callback with it.
func (a volumeMountStatus) WithRoot(logger *zap.Logger, callback func(root xfs.Root) error) error {
	var root xfs.Root

	root, ok := a.VolumeMountStatus.TypedSpec().Root().(xfs.Root)

	if !ok || root == nil || !a.VolumeMountStatus.TypedSpec().Detached {
		root = &xfs.UnixRoot{
			FS: opentree.NewFromPath(a.VolumeMountStatus.TypedSpec().Target),
		}
		if err := root.OpenFS(); err != nil {
			return fmt.Errorf("error opening filesystem: %w", err)
		}

		defer func() {
			if err := root.Close(); err != nil {
				logger.Error("error closing filesystem", zap.Error(err))
			}
		}()
	}

	return callback(root)
}
