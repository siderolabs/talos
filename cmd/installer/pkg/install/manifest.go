// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package install

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/siderolabs/go-blockdevice/blockdevice"
	"github.com/siderolabs/go-blockdevice/blockdevice/partition/gpt"
	"github.com/siderolabs/go-retry/retry"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/board"
	"github.com/siderolabs/talos/internal/pkg/mount"
	"github.com/siderolabs/talos/internal/pkg/partition"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/imager/quirks"
)

// Manifest represents the instructions for preparing all block devices
// for an installation.
type Manifest struct {
	PartitionOptions  *runtime.PartitionOptions
	Devices           map[string]Device
	Targets           map[string][]*Target
	LegacyBIOSSupport bool

	Printf func(string, ...any)
}

// Device represents device options.
type Device struct {
	Device string

	ResetPartitionTable bool
	Zero                bool

	SkipOverlayMountsCheck bool
}

// NewManifest initializes and returns a Manifest.
//
//nolint:gocyclo,cyclop
func NewManifest(mode Mode, uefiOnlyBoot bool, bootLoaderPresent bool, opts *Options) (manifest *Manifest, err error) {
	manifest = &Manifest{
		Devices:           map[string]Device{},
		Targets:           map[string][]*Target{},
		LegacyBIOSSupport: opts.LegacyBIOSSupport,

		Printf: opts.Printf,
	}

	if opts.Board != constants.BoardNone && !quirks.New(opts.Version).SupportsOverlay() {
		if uefiOnlyBoot {
			return nil, errors.New("board option can't be used with uefi-only-boot")
		}

		var b runtime.Board

		b, err = board.NewBoard(opts.Board)
		if err != nil {
			return nil, err
		}

		manifest.PartitionOptions = b.PartitionOptions()
	}

	if opts.OverlayInstaller != nil {
		overlayOpts, getOptsErr := opts.OverlayInstaller.GetOptions(opts.ExtraOptions)
		if getOptsErr != nil {
			return nil, fmt.Errorf("failed to get overlay installer options: %w", getOptsErr)
		}

		if overlayOpts.PartitionOptions.Offset != 0 {
			manifest.PartitionOptions = &runtime.PartitionOptions{
				PartitionsOffset: overlayOpts.PartitionOptions.Offset,
			}
		}
	}

	// TODO: legacy, to support old Talos initramfs, assume force if boot partition not found
	if !bootLoaderPresent {
		opts.Force = true
	}

	if !opts.Force && opts.Zero {
		return nil, errors.New("zero option can't be used without force")
	}

	if !opts.Force && !bootLoaderPresent {
		return nil, errors.New("install with preserve is not supported if existing boot partition was not found")
	}

	// Verify that the target device(s) can satisfy the requested options.

	if mode == ModeInstall {
		if err = VerifyEphemeralPartition(opts); err != nil {
			return nil, fmt.Errorf("failed to prepare ephemeral partition: %w", err)
		}
	}

	skipOverlayMountsCheck, err := shouldSkipOverlayMountsCheck(mode)
	if err != nil {
		return nil, err
	}

	manifest.Devices[opts.Disk] = Device{
		Device: opts.Disk,

		ResetPartitionTable: opts.Force,
		Zero:                opts.Zero,

		SkipOverlayMountsCheck: skipOverlayMountsCheck,
	}

	// Initialize any slices we need. Note that a boot partition is not
	// required.

	if manifest.Targets[opts.Disk] == nil {
		manifest.Targets[opts.Disk] = []*Target{}
	}

	targets := []*Target{}

	// create GRUB BIOS+UEFI partitions, or only one big EFI partition if not using GRUB
	if !uefiOnlyBoot {
		targets = append(targets,
			EFITarget(opts.Disk, nil),
			BIOSTarget(opts.Disk, nil),
			BootTarget(opts.Disk, &Target{
				PreserveContents: bootLoaderPresent,
			}),
		)
	} else {
		targets = append(targets,
			EFITargetUKI(opts.Disk, &Target{
				PreserveContents: bootLoaderPresent,
			}),
		)
	}

	targets = append(targets,
		MetaTarget(opts.Disk, &Target{
			PreserveContents: bootLoaderPresent,
		}),
		StateTarget(opts.Disk, &Target{
			PreserveContents: bootLoaderPresent,
			FormatOptions: &partition.FormatOptions{
				FileSystemType: partition.FilesystemTypeNone,
			},
		}),
		EphemeralTarget(opts.Disk, NoFilesystem),
	)

	if !opts.Force {
		for _, target := range targets {
			target.Force = false
			target.Skip = true
		}
	}

	for _, target := range targets {
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
//
//nolint:gocyclo
func (m *Manifest) checkMounts(device Device) error {
	matches, err := filepath.Glob("/proc/*/mountinfo")
	if err != nil {
		return err
	}

	for _, path := range matches {
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

				if !device.SkipOverlayMountsCheck && fields[len(fields)-2] == "overlay" {
					//nolint:dupword
					// parsing options (last column) in the overlay mount line which looks like:
					// 163 70 0:52 /apid / ro,relatime - overlay overlay rw,lowerdir=/opt,upperdir=/var/system/overlays/opt-diff,workdir=/var/system/overlays/opt-workdir

					options := strings.Split(fields[len(fields)-1], ",")

					for _, option := range options {
						parts := strings.SplitN(option, "=", 2)
						if len(parts) == 2 {
							if strings.HasPrefix(parts[1], "/var/") {
								return fmt.Errorf("found overlay mount in %q: %s", path, scanner.Text())
							}
						}
					}
				}

				if strings.HasPrefix(fields[len(fields)-2], device.Device) {
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

//nolint:gocyclo,cyclop
func (m *Manifest) executeOnDevice(device Device, targets []*Target) (err error) {
	if err = m.checkMounts(device); err != nil {
		return err
	}

	if err = m.preserveContents(device, targets); err != nil {
		return err
	}

	if device.Zero {
		if err = partition.Format(device.Device, &partition.FormatOptions{
			FileSystemType: partition.FilesystemTypeNone,
		}, m.Printf); err != nil {
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

		m.Printf("creating new partition table on %s", device.Device)

		gptOpts := []gpt.Option{
			gpt.WithMarkMBRBootable(m.LegacyBIOSSupport),
		}

		if m.PartitionOptions != nil {
			gptOpts = append(gptOpts, gpt.WithPartitionEntriesStartLBA(m.PartitionOptions.PartitionsOffset))
		}

		pt, err = gpt.New(bd.Device(), gptOpts...)
		if err != nil {
			return err
		}

		m.Printf("logical/physical block size: %d/%d", pt.Header().LBA.LogicalBlockSize, pt.Header().LBA.PhysicalBlockSize)
		m.Printf("minimum/optimal I/O size: %d/%d", pt.Header().LBA.MinimalIOSize, pt.Header().LBA.OptimalIOSize)

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
			m.Printf("resetting partition table on %s", device.Device)

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
					m.Printf("deleting partition %s", part.Name)

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
		if err = target.partition(pt, i, m.Printf); err != nil {
			return fmt.Errorf("failed to partition device: %w", err)
		}
	}

	if err = pt.Write(); err != nil {
		return err
	}

	for _, target := range targets {
		err = retry.Constant(time.Minute, retry.WithUnits(100*time.Millisecond)).Retry(func() error {
			e := target.Format(m.Printf)
			if e != nil {
				if strings.Contains(e.Error(), "No such file or directory") {
					// workaround problem with partition device not being visible immediately after partitioning
					return retry.ExpectedError(e)
				}

				return e
			}

			return nil
		})
		if err != nil {
			return fmt.Errorf("failed to format device: %w", err)
		}
	}

	return m.restoreContents(targets)
}

//nolint:gocyclo
func (m *Manifest) preserveContents(device Device, targets []*Target) (err error) {
	anyPreserveContents := false

	for _, target := range targets {
		if target.Skip {
			continue
		}

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
		m.Printf("warning: skipping preserve contents on %q as block device failed: %s", device.Device, err)

		return nil
	}

	//nolint:errcheck
	defer bd.Close()

	pt, err := bd.PartitionTable()
	if err != nil {
		m.Printf("warning: skipping preserve contents on %q as partition table failed: %s", device.Device, err)

		return nil
	}

	for _, target := range targets {
		if target.Skip {
			continue
		}

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
			m.Printf("warning: failed to preserve contents of %q on %q, as source partition wasn't found", target.Label, device.Device)

			continue
		}

		if err = target.SaveContents(device, sourcePart, fileSystemType, fnmatchFilters); err != nil {
			m.Printf("warning: failed to preserve contents of %q on %q: %s", target.Label, device.Device, err)
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
func (m *Manifest) SystemMountpoints(ctx context.Context, opts ...mount.Option) (*mount.Points, error) {
	mountpoints := mount.NewMountPoints()

	for dev := range m.Targets {
		mp, err := mount.SystemMountPointsForDevice(ctx, dev, opts...)
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

func shouldSkipOverlayMountsCheck(mode Mode) (bool, error) {
	var skipOverlayMountsCheck bool

	_, err := os.Stat("/.dockerenv")

	switch {
	case err == nil:
		skipOverlayMountsCheck = true
	case os.IsNotExist(err):
		skipOverlayMountsCheck = mode.IsImage()
	default:
		return false, fmt.Errorf("cannot determine if /.dockerenv exists: %w", err)
	}

	return skipOverlayMountsCheck, nil
}
