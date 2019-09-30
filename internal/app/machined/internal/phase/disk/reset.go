/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package disk

import (
	"github.com/pkg/errors"

	"github.com/talos-systems/talos/internal/app/machined/internal/phase"
	"github.com/talos-systems/talos/internal/pkg/runtime"
	"github.com/talos-systems/talos/pkg/blockdevice"
	"github.com/talos-systems/talos/pkg/blockdevice/table"
)

// ResetDisk represents the task for stop all containerd tasks in the
// k8s.io namespace.
type ResetDisk struct {
	devname string
}

// NewResetDiskTask initializes and returns an Services task.
func NewResetDiskTask(devname string) phase.Task {
	return &ResetDisk{
		devname: devname,
	}
}

// RuntimeFunc returns the runtime function.
func (task *ResetDisk) RuntimeFunc(mode runtime.Mode) phase.RuntimeFunc {
	return func(args *phase.RuntimeArgs) error {
		return task.standard()
	}
}

func (task *ResetDisk) standard() (err error) {
	var bd *blockdevice.BlockDevice
	if bd, err = blockdevice.Open(task.devname); err != nil {
		return err
	}
	// nolint: errcheck
	defer bd.Close()

	var pt table.PartitionTable
	if pt, err = bd.PartitionTable(true); err != nil {
		return err
	}
	for _, p := range pt.Partitions() {
		if err = pt.Delete(p); err != nil {
			return errors.Wrap(err, "failed to delete partition")
		}
	}

	if err = pt.Write(); err != nil {
		return err
	}

	return nil
}
