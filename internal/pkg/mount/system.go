// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package mount

import (
	"fmt"
	"os"
	"sync"

	"github.com/talos-systems/go-blockdevice/blockdevice"
	"github.com/talos-systems/go-blockdevice/blockdevice/filesystem"
	"golang.org/x/sys/unix"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/disk"
	"github.com/talos-systems/talos/internal/pkg/encryption"
	"github.com/talos-systems/talos/internal/pkg/partition"
	"github.com/talos-systems/talos/pkg/machinery/config"
	"github.com/talos-systems/talos/pkg/machinery/constants"
)

var (
	mountpoints      = map[string]*Point{}
	mountpointsMutex sync.RWMutex
)

// SystemMountPointsForDevice returns the mountpoints required to boot the system.
// This function is called exclusively during installations ( both image
// creation and bare metall installs ). This is why we want to look up
// device by specified disk as well as why we don't want to grow any
// filesystems.
func SystemMountPointsForDevice(devpath string, opts ...Option) (mountpoints *Points, err error) {
	mountpoints = NewMountPoints()

	bd, err := blockdevice.Open(devpath)
	if err != nil {
		return nil, err
	}

	defer bd.Close() //nolint:errcheck

	for _, name := range []string{constants.EphemeralPartitionLabel, constants.BootPartitionLabel, constants.EFIPartitionLabel, constants.StatePartitionLabel} {
		mountpoint, err := SystemMountPointForLabel(bd, name, opts...)
		if err != nil {
			return nil, err
		}

		mountpoints.Set(name, mountpoint)
	}

	return mountpoints, nil
}

// SystemMountPointForLabel returns a mount point for the specified device and label.
//nolint:gocyclo
func SystemMountPointForLabel(device *blockdevice.BlockDevice, label string, opts ...Option) (mountpoint *Point, err error) {
	var target string

	switch label {
	case constants.EphemeralPartitionLabel:
		target = constants.EphemeralMountPoint
	case constants.BootPartitionLabel:
		target = constants.BootMountPoint
	case constants.EFIPartitionLabel:
		target = constants.EFIMountPoint
	case constants.StatePartitionLabel:
		target = constants.StateMountPoint
	default:
		return nil, fmt.Errorf("unknown label: %q", label)
	}

	part, err := device.GetPartition(label)
	if err != nil && err != os.ErrNotExist {
		return nil, err
	}

	if part == nil {
		// A boot partitition is not required.
		if label == constants.BootPartitionLabel {
			return nil, nil
		}

		return nil, fmt.Errorf("failed to find device with label %s: %w", label, err)
	}

	fsType, err := part.Filesystem()
	if err != nil {
		return nil, err
	}

	partPath, err := part.Path()
	if err != nil {
		return nil, err
	}

	o := NewDefaultOptions(opts...)

	preMountHooks := []Hook{}

	if o.Encryption != nil {
		encryptionHandler, err := encryption.NewHandler(
			device,
			part,
			o.Encryption,
		)
		if err != nil {
			return nil, err
		}

		preMountHooks = append(preMountHooks,
			func(p *Point) error {
				var (
					err  error
					path string
				)

				if path, err = encryptionHandler.Open(); err != nil {
					return err
				}

				p.source = path

				return nil
			},
		)

		opts = append(opts,
			WithPostUnmountHooks(
				func(p *Point) error {
					return encryptionHandler.Close()
				},
			),
		)
	}

	// Format the partition if it does not have any filesystem
	preMountHooks = append(preMountHooks, func(p *Point) error {
		sb, err := filesystem.Probe(p.source)
		if err != nil {
			return err
		}

		p.fstype = ""

		// skip formatting the partition if filesystem is detected
		// and assign proper fs type to the mountpoint
		if sb != nil && sb.Type() != filesystem.Unknown {
			p.fstype = sb.Type()

			return nil
		}

		opts := partition.NewFormatOptions(part.Name)
		if opts == nil {
			return fmt.Errorf("failed to determine format options for partition label %s", part.Name)
		}

		p.fstype = opts.FileSystemType

		return partition.Format(p.source, opts)
	})

	opts = append(opts, WithPreMountHooks(preMountHooks...))

	mountpoint = NewMountPoint(partPath, target, fsType, unix.MS_NOATIME, "", opts...)

	return mountpoint, nil
}

// SystemPartitionMount mounts a system partition by the label.
func SystemPartitionMount(r runtime.Runtime, label string, opts ...Option) (err error) {
	device := r.State().Machine().Disk(disk.WithPartitionLabel(label))
	if device == nil {
		return fmt.Errorf("failed to find device with partition labeled %s", label)
	}

	var encryptionConfig config.Encryption

	if r.Config() != nil && r.Config().Machine() != nil {
		encryptionConfig = r.Config().Machine().SystemDiskEncryption().Get(label)
	}

	if encryptionConfig != nil {
		opts = append(opts, WithEncryptionConfig(encryptionConfig))
	}

	mountpoint, err := SystemMountPointForLabel(device.BlockDevice, label, opts...)
	if err != nil {
		return err
	}

	if mountpoint == nil {
		return fmt.Errorf("no mountpoints for label %q", label)
	}

	if err = mountMountpoint(mountpoint); err != nil {
		return err
	}

	mountpointsMutex.Lock()
	defer mountpointsMutex.Unlock()

	mountpoints[label] = mountpoint

	return nil
}

// SystemPartitionUnmount unmounts a system partition by the label.
func SystemPartitionUnmount(r runtime.Runtime, label string) (err error) {
	mountpointsMutex.RLock()
	mountpoint, ok := mountpoints[label]
	mountpointsMutex.RUnlock()

	if !ok {
		return nil
	}

	err = mountpoint.Unmount()
	if err != nil {
		return err
	}

	mountpointsMutex.Lock()
	delete(mountpoints, label)
	mountpointsMutex.Unlock()

	return nil
}
