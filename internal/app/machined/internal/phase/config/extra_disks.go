// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config

import (
	"log"

	"golang.org/x/sys/unix"

	"github.com/talos-systems/talos/cmd/installer/pkg/manifest"
	"github.com/talos-systems/talos/internal/app/machined/internal/phase"
	"github.com/talos-systems/talos/internal/pkg/mount"
	"github.com/talos-systems/talos/internal/pkg/mount/manager"
	"github.com/talos-systems/talos/internal/pkg/runtime"
	"github.com/talos-systems/talos/pkg/blockdevice/probe"
	"github.com/talos-systems/talos/pkg/blockdevice/util"
)

// ExtraDisks represents the ExtraDisks task.
type ExtraDisks struct{}

// NewExtraDisksTask initializes and returns an ExtraDisks task.
func NewExtraDisksTask() phase.Task {
	return &ExtraDisks{}
}

// TaskFunc returns the runtime function.
func (task *ExtraDisks) TaskFunc(mode runtime.Mode) phase.TaskFunc {
	return task.runtime
}

func (task *ExtraDisks) runtime(r runtime.Runtime) (err error) {
	if err = partitionAndFormatDisks(r); err != nil {
		return err
	}

	return mountDisks(r)
}

func partitionAndFormatDisks(r runtime.Runtime) (err error) {
	m := &manifest.Manifest{
		Targets: map[string][]*manifest.Target{},
	}

	for _, disk := range r.Config().Machine().Disks() {
		if m.Targets[disk.Device] == nil {
			m.Targets[disk.Device] = []*manifest.Target{}
		}

		for _, part := range disk.Partitions {
			extraTarget := &manifest.Target{
				Device: disk.Device,
				Size:   part.Size,
				Force:  true,
				Test:   false,
			}

			m.Targets[disk.Device] = append(m.Targets[disk.Device], extraTarget)
		}
	}

	probed, err := probe.All()
	if err != nil {
		return err
	}

	// TODO(andrewrynhard): This is disgusting, but it works. We should revisit
	// this at a later time.
	for _, p := range probed {
		for _, disk := range r.Config().Machine().Disks() {
			for i := range disk.Partitions {
				partname := util.PartPath(disk.Device, i+1)
				if p.Path == partname {
					log.Printf(("found existing partitions for %q"), disk.Device)
					return nil
				}
			}
		}
	}

	if err = m.ExecuteManifest(); err != nil {
		return err
	}

	return nil
}

func mountDisks(r runtime.Runtime) (err error) {
	mountpoints := mount.NewMountPoints()

	for _, extra := range r.Config().Machine().Disks() {
		for i, part := range extra.Partitions {
			partname := util.PartPath(extra.Device, i+1)
			mountpoints.Set(partname, mount.NewMountPoint(partname, part.MountPoint, "xfs", unix.MS_NOATIME, ""))
		}
	}

	extras := manager.NewManager(mountpoints)
	if err = extras.MountAll(); err != nil {
		return err
	}

	return nil
}
