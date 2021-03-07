// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package install

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/talos-systems/go-blockdevice/blockdevice"
	"github.com/talos-systems/go-blockdevice/blockdevice/partition/gpt"
	"github.com/talos-systems/go-blockdevice/blockdevice/util"
	"github.com/talos-systems/go-retry/retry"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/board"
	"github.com/talos-systems/talos/internal/pkg/mount"
	"github.com/talos-systems/talos/internal/pkg/partition"
	"github.com/talos-systems/talos/pkg/machinery/constants"
)

// Manifest represents the instructions for preparing all block devices
// for an installation.
type Manifest struct {
	PartitionOptions *runtime.PartitionOptions
	Devices          map[string]Device
	Targets          map[string][]*Target
}

// Device represents device options.
type Device struct {
	Device string

	ResetPartitionTable bool
	Zero                bool
}

// NewManifest initializes and returns a Manifest.
//
//nolint:gocyclo
func NewManifest(label string, sequence runtime.Sequence, bootPartitionFound bool, opts *Options) (manifest *Manifest, err error) {
	if label == "" {
		return nil, fmt.Errorf("a label is required, got \"\"")
	}

	manifest = &Manifest{
		Devices: map[string]Device{},
		Targets: map[string][]*Target{},
	}

	if opts.Board != constants.BoardNone {
		var b runtime.Board

		b, err = board.NewBoard(opts.Board)
		if err != nil {
			return nil, err
		}

		manifest.PartitionOptions = b.PartitionOptions()
	}

	// TODO: legacy, to support old Talos initramfs, assume force if boot partition not found
	if !bootPartitionFound {
		opts.Force = true
	}

	if !opts.Force && opts.Zero {
		return nil, fmt.Errorf("zero option can't be used without force")
	}

	if !opts.Force && !bootPartitionFound {
		return nil, fmt.Errorf("install with preserve is not supported if existing boot partition was not found")
	}

	// Verify that the target device(s) can satisfy the requested options.

	if sequence != runtime.SequenceUpgrade {
		if err = VerifyEphemeralPartition(opts); err != nil {
			return nil, fmt.Errorf("failed to prepare ephemeral partition: %w", err)
		}

		if err = VerifyBootPartition(opts); err != nil {
			return nil, fmt.Errorf("failed to prepare boot partition: %w", err)
		}
	}

	manifest.Devices[opts.Disk] = Device{
		Device: opts.Disk,

		ResetPartitionTable: opts.Force,
		Zero:                opts.Zero,
	}

	// Initialize any slices we need. Note that a boot partition is not
	// required.

	if manifest.Targets[opts.Disk] == nil {
		manifest.Targets[opts.Disk] = []*Target{}
	}

	efiTarget := EFITarget(opts.Disk, nil)
	biosTarget := BIOSTarget(opts.Disk, nil)

	var bootTarget *Target

	if opts.Bootloader {
		bootTarget = BootTarget(opts.Disk, &Target{
			PreserveContents: bootPartitionFound,
			Assets: []*Asset{
				{
					Source:      constants.KernelAssetPath,
					Destination: filepath.Join(constants.BootMountPoint, label, constants.KernelAsset),
				},
				{
					Source:      constants.InitramfsAssetPath,
					Destination: filepath.Join(constants.BootMountPoint, label, constants.InitramfsAsset),
				},
			},
		})
	}

	metaTarget := MetaTarget(opts.Disk, &Target{
		PreserveContents: bootPartitionFound,
	})

	stateTarget := StateTarget(opts.Disk, &Target{
		PreserveContents: bootPartitionFound,
		FormatOptions: &partition.FormatOptions{
			FileSystemType: partition.FilesystemTypeNone,
		},
	})

	ephemeralTarget := EphemeralTarget(opts.Disk, NoFilesystem)

	if opts.Force {
		ephemeralTarget.Force = true
	} else {
		ephemeralTarget.Force = false
		ephemeralTarget.Skip = true
		stateTarget.Size = 0 // expand previous partition to cover whatever space is available
	}

	for _, target := range []*Target{efiTarget, biosTarget, bootTarget, metaTarget, stateTarget, ephemeralTarget} {
		if target == nil {
			continue
		}

		manifest.Targets[target.Device] = append(manifest.Targets[target.Device], target)
	}

	return manifest, nil
}

// Execute partitions and formats all disks in a manifest.
func (m *Manifest) Execute() (err error) {
	for dev, targets := range m.Targets {
		if err = m.executeOnDevice(m.Devices[dev], targets); err != nil {
			return err
		}
	}

	return nil
}

// checkMounts verifies that no active mounts in any mount namespace exist for the device.
func (m *Manifest) checkMounts(device Device) error {
	matches, err := filepath.Glob("/proc/*/mountinfo")
	if err != nil {
		return err
	}

	for _, path := range matches {
		path := path

		if err = func() error {
			var f *os.File

			f, err = os.Open(path)
			if err != nil {
				// ignore error in case process got removed
				return nil
			}

			defer f.Close() //nolint:errcheck

			scanner := bufio.NewScanner(f)
			for scanner.Scan() {
				fields := strings.Fields(scanner.Text())

				if len(fields) < 2 {
					continue
				}

				if fields[len(fields)-2] == device.Device {
					return fmt.Errorf("found active mount in %q for %q: %s", path, device.Device, scanner.Text())
				}
			}

			return f.Close()
		}(); err != nil {
			return err
		}
	}

	return nil
}

//nolint:gocyclo
func (m *Manifest) executeOnDevice(device Device, targets []*Target) (err error) {
	if err = m.checkMounts(device); err != nil {
		return err
	}

	if err = m.preserveContents(device, targets); err != nil {
		return err
	}

	if device.Zero {
		if err = m.zeroDevice(device); err != nil {
			return err
		}
	}

	var bd *blockdevice.BlockDevice

	if bd, err = blockdevice.Open(device.Device, blockdevice.WithExclusiveLock(true)); err != nil {
		return err
	}

	//nolint:errcheck
	defer bd.Close()

	var pt *gpt.GPT

	created := false

	pt, err = bd.PartitionTable()
	if err != nil {
		if !errors.Is(err, blockdevice.ErrMissingPartitionTable) {
			return err
		}

		log.Printf("creating new partition table on %s", device.Device)

		gptOpts := []gpt.Option{}

		if m.PartitionOptions != nil {
			gptOpts = append(gptOpts, gpt.WithPartitionEntriesStartLBA(m.PartitionOptions.PartitionsOffset))
		}

		pt, err = gpt.New(bd.Device(), gptOpts...)
		if err != nil {
			return err
		}

		if err = pt.Write(); err != nil {
			return err
		}

		if err = bd.Close(); err != nil {
			return err
		}

		if bd, err = blockdevice.Open(device.Device, blockdevice.WithExclusiveLock(true)); err != nil {
			return err
		}

		defer bd.Close() //nolint:errcheck

		created = true
	}

	if !created {
		if device.ResetPartitionTable {
			log.Printf("resetting partition table on %s", device.Device)

			// TODO: how should it work with zero option above?
			if err = bd.Reset(); err != nil {
				return err
			}
		} else {
			// clean up partitions which are going to be recreated
			keepPartitions := map[string]struct{}{}

			for _, target := range targets {
				if target.Skip {
					keepPartitions[target.Label] = struct{}{}
				}
			}

			// make sure all partitions to be skipped already exist
			missingPartitions := map[string]struct{}{}

			for label := range keepPartitions {
				missingPartitions[label] = struct{}{}
			}

			for _, part := range pt.Partitions().Items() {
				delete(missingPartitions, part.Name)
			}

			if len(missingPartitions) > 0 {
				return fmt.Errorf("some partitions to be skipped are missing: %v", missingPartitions)
			}

			// delete all partitions which are not skipped
			for _, part := range pt.Partitions().Items() {
				if _, ok := keepPartitions[part.Name]; !ok {
					log.Printf("deleting partition %s", part.Name)

					if err = pt.Delete(part); err != nil {
						return err
					}
				}
			}

			if err = pt.Write(); err != nil {
				return err
			}
		}
	}

	pt, err = bd.PartitionTable()
	if err != nil {
		return err
	}

	for i, target := range targets {
		if err = target.Partition(pt, i, bd); err != nil {
			return fmt.Errorf("failed to partition device: %w", err)
		}
	}

	if err = pt.Write(); err != nil {
		return err
	}

	for _, target := range targets {
		target := target

		err = retry.Constant(time.Minute, retry.WithUnits(100*time.Millisecond)).Retry(func() error {
			e := target.Format()
			if e != nil {
				if strings.Contains(e.Error(), "No such file or directory") {
					// workaround problem with partition device not being visible immediately after partitioning
					return retry.ExpectedError(e)
				}

				return retry.UnexpectedError(e)
			}

			return nil
		})

		if err != nil {
			return fmt.Errorf("failed to format device: %w", err)
		}
	}

	if err = m.restoreContents(targets); err != nil {
		return err
	}

	return nil
}

//nolint:gocyclo
func (m *Manifest) preserveContents(device Device, targets []*Target) (err error) {
	anyPreserveContents := false

	for _, target := range targets {
		if target.PreserveContents {
			anyPreserveContents = true

			break
		}
	}

	if !anyPreserveContents {
		// no target to preserve contents, exit early
		return nil
	}

	var bd *blockdevice.BlockDevice

	if bd, err = blockdevice.Open(device.Device); err != nil {
		// failed to open the block device, probably it's damaged?
		log.Printf("warning: skipping preserve contents on %q as block device failed: %s", device.Device, err)

		return nil
	}

	//nolint:errcheck
	defer bd.Close()

	pt, err := bd.PartitionTable()
	if err != nil {
		log.Printf("warning: skipping preserve contents on %q as partition table failed: %s", device.Device, err)

		return nil
	}

	for _, target := range targets {
		if !target.PreserveContents {
			continue
		}

		var (
			sourcePart     *gpt.Partition
			fileSystemType partition.FileSystemType
			fnmatchFilters []string
		)

		sources := append([]PreserveSource{
			{
				Label:          target.Label,
				FileSystemType: target.FileSystemType,
			},
		}, target.ExtraPreserveSources...)

		for _, source := range sources {
			// find matching existing partition table entry
			for _, part := range pt.Partitions().Items() {
				if part.Name == source.Label {
					sourcePart = part
					fileSystemType = source.FileSystemType
					fnmatchFilters = source.FnmatchFilters

					break
				}
			}
		}

		if sourcePart == nil {
			log.Printf("warning: failed to preserve contents of %q on %q, as source partition wasn't found", target.Label, device.Device)

			continue
		}

		if err = target.SaveContents(device, sourcePart, fileSystemType, fnmatchFilters); err != nil {
			log.Printf("warning: failed to preserve contents of %q on %q: %s", target.Label, device.Device, err)
		}
	}

	return nil
}

func (m *Manifest) restoreContents(targets []*Target) error {
	for _, target := range targets {
		if err := target.RestoreContents(); err != nil {
			return fmt.Errorf("error restoring contents for %q: %w", target.Label, err)
		}
	}

	return nil
}

// SystemMountpoints returns list of system mountpoints for the manifest.
func (m *Manifest) SystemMountpoints(opts ...mount.Option) (*mount.Points, error) {
	mountpoints := mount.NewMountPoints()

	for dev := range m.Targets {
		mp, err := mount.SystemMountPointsForDevice(dev, opts...)
		if err != nil {
			return nil, err
		}

		iter := mp.Iter()
		for iter.Next() {
			mountpoints.Set(iter.Key(), iter.Value())
		}
	}

	return mountpoints, nil
}

// zeroDevice fills the device with zeroes.
func (m *Manifest) zeroDevice(device Device) (err error) {
	var bd *blockdevice.BlockDevice

	log.Printf("wiping %q", device.Device)

	if bd, err = blockdevice.Open(device.Device, blockdevice.WithExclusiveLock(true)); err != nil {
		return err
	}

	defer bd.Close() //nolint:errcheck

	var method string

	if method, err = bd.Wipe(); err != nil {
		return err
	}

	log.Printf("wiped %q with %q", device.Device, method)

	return bd.Close()
}

// Partition creates a new partition on the specified device.
//nolint:dupl,gocyclo
func (t *Target) Partition(pt *gpt.GPT, pos int, bd *blockdevice.BlockDevice) (err error) {
	if t.Skip {
		part := pt.Partitions().FindByName(t.Label)
		if part != nil {
			log.Printf("skipped %s (%s) size %d blocks", t.PartitionName, t.Label, part.Length())
		}

		return nil
	}

	log.Printf("partitioning %s - %s %q\n", t.Device, t.Label, humanize.Bytes(t.Size))

	opts := []gpt.PartitionOption{
		gpt.WithPartitionType(t.PartitionType),
		gpt.WithPartitionName(t.Label),
	}

	if t.Size == 0 {
		opts = append(opts, gpt.WithMaximumSize(true))
	}

	if t.LegacyBIOSBootable {
		opts = append(opts, gpt.WithLegacyBIOSBootableAttribute(true))
	}

	part, err := pt.InsertAt(pos, t.Size, opts...)
	if err != nil {
		return err
	}

	t.PartitionName, err = util.PartPath(t.Device, int(part.Number))
	if err != nil {
		return err
	}

	log.Printf("created %s (%s) size %d blocks", t.PartitionName, t.Label, part.Length())

	return nil
}
