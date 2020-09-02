// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package install

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/talos-systems/go-retry/retry"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/pkg/blockdevice"
	"github.com/talos-systems/talos/pkg/blockdevice/filesystem/vfat"
	"github.com/talos-systems/talos/pkg/blockdevice/filesystem/xfs"
	"github.com/talos-systems/talos/pkg/blockdevice/table"
	"github.com/talos-systems/talos/pkg/blockdevice/table/gpt/partition"
	"github.com/talos-systems/talos/pkg/blockdevice/util"
	"github.com/talos-systems/talos/pkg/machinery/constants"
)

// Manifest represents the instructions for preparing all block devices
// for an installation.
type Manifest struct {
	Targets map[string][]*Target
}

// Target represents an installation partition.
type Target struct {
	Label          string
	Device         string
	FileSystemType string
	PartitionName  string
	Size           uint
	Force          bool
	Test           bool
	Assets         []*Asset
	BlockDevice    *blockdevice.BlockDevice
}

// Asset represents a file required by a target.
type Asset struct {
	Source      string
	Destination string
}

// NewManifest initializes and returns a Manifest.
func NewManifest(label string, sequence runtime.Sequence, opts *Options) (manifest *Manifest, err error) {
	if label == "" {
		return nil, fmt.Errorf("a label is required, got \"\"")
	}

	manifest = &Manifest{
		Targets: map[string][]*Target{},
	}

	// Verify that the target device(s) can satisify the requested options.

	if sequence != runtime.SequenceUpgrade {
		if err = VerifyEphemeralPartition(opts); err != nil {
			return nil, fmt.Errorf("failed to prepare ephemeral partition: %w", err)
		}

		if err = VerifyBootPartition(opts); err != nil {
			return nil, fmt.Errorf("failed to prepare boot partition: %w", err)
		}
	}

	// Initialize any slices we need. Note that a boot paritition is not
	// required.

	if manifest.Targets[opts.Disk] == nil {
		manifest.Targets[opts.Disk] = []*Target{}
	}

	efiTarget := &Target{
		Device: opts.Disk,
		Label:  constants.EFIPartitionLabel,
		Size:   100 * 1024 * 1024,
		Force:  true,
		Test:   false,
	}

	biosTarget := &Target{
		Device: opts.Disk,
		Label:  constants.BIOSGrubPartitionLabel,
		Size:   1 * 1024 * 1024,
		Force:  true,
		Test:   false,
	}

	var bootTarget *Target

	if opts.Bootloader {
		bootTarget = &Target{
			Device: opts.Disk,
			Label:  constants.BootPartitionLabel,
			Size:   300 * 1024 * 1024,
			Force:  true,
			Test:   false,
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
		}
	}

	metaTarget := &Target{
		Device: opts.Disk,
		Label:  constants.MetaPartitionLabel,
		Size:   1 * 1024 * 1024,
		Force:  true,
		Test:   false,
	}

	stateTarget := &Target{
		Device: opts.Disk,
		Label:  constants.StatePartitionLabel,
		Size:   100 * 1024 * 1024,
		Force:  true,
		Test:   false,
	}

	ephemeralTarget := &Target{
		Device: opts.Disk,
		Label:  constants.EphemeralPartitionLabel,
		Size:   0,
		Force:  true,
		Test:   false,
	}

	for _, target := range []*Target{efiTarget, biosTarget, bootTarget, metaTarget, stateTarget, ephemeralTarget} {
		if target == nil {
			continue
		}

		manifest.Targets[target.Device] = append(manifest.Targets[target.Device], target)
	}

	return manifest, nil
}

// ExecuteManifest partitions and formats all disks in a manifest.
func (m *Manifest) ExecuteManifest() (err error) {
	for dev, targets := range m.Targets {
		var bd *blockdevice.BlockDevice

		if bd, err = blockdevice.Open(dev, blockdevice.WithNewGPT(true)); err != nil {
			return err
		}

		// nolint: errcheck
		defer bd.Close()

		for _, target := range targets {
			if err = target.Partition(bd); err != nil {
				return fmt.Errorf("failed to partition device: %w", err)
			}
		}

		if err = bd.RereadPartitionTable(); err != nil {
			log.Printf("failed to re-read partition table on %q: %s, ignoring error...", dev, err)
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
	}

	return nil
}

// Partition creates a new partition on the specified device.
// nolint: dupl, gocyclo
func (t *Target) Partition(bd *blockdevice.BlockDevice) (err error) {
	log.Printf("partitioning %s - %s\n", t.Device, t.Label)

	var pt table.PartitionTable

	if pt, err = bd.PartitionTable(); err != nil {
		return err
	}

	opts := []interface{}{}

	const (
		EFISystemPartition  = "C12A7328-F81F-11D2-BA4B-00A0C93EC93B"
		BIOSBootPartition   = "21686148-6449-6E6F-744E-656564454649"
		LinuxFilesystemData = "0FC63DAF-8483-4772-8E79-3D69D8477DE4"
	)

	switch t.Label {
	case constants.EFIPartitionLabel:
		opts = append(opts, partition.WithPartitionType(EFISystemPartition), partition.WithPartitionName(t.Label))
	case constants.BIOSGrubPartitionLabel:
		opts = append(opts, partition.WithPartitionType(BIOSBootPartition), partition.WithPartitionName(t.Label), partition.WithLegacyBIOSBootableAttribute(true))
	case constants.BootPartitionLabel:
		opts = append(opts, partition.WithPartitionType(LinuxFilesystemData), partition.WithPartitionName(t.Label))
	case constants.MetaPartitionLabel:
		opts = append(opts, partition.WithPartitionType(LinuxFilesystemData), partition.WithPartitionName(t.Label))
	case constants.StatePartitionLabel:
		opts = append(opts, partition.WithPartitionType(LinuxFilesystemData), partition.WithPartitionName(t.Label))
	case constants.EphemeralPartitionLabel:
		opts = append(opts, partition.WithPartitionType(LinuxFilesystemData), partition.WithPartitionName(t.Label), partition.WithMaximumSize(true))
	default:
		opts = append(opts, partition.WithPartitionType(LinuxFilesystemData))

		if t.Size == 0 {
			opts = append(opts, partition.WithMaximumSize(true))
		}
	}

	part, err := pt.Add(uint64(t.Size), opts...)
	if err != nil {
		return err
	}

	if err = pt.Write(); err != nil {
		return err
	}

	t.PartitionName, err = util.PartPath(t.Device, int(part.No()))
	if err != nil {
		return err
	}

	return nil
}

// Format creates a filesystem on the device/partition.
//
//nolint: gocyclo
func (t *Target) Format() error {
	switch t.Label {
	case constants.EFIPartitionLabel:
		log.Printf("formatting partition %q as %q with label %q\n", t.PartitionName, "fat", t.Label)
		return vfat.MakeFS(t.PartitionName, vfat.WithLabel(t.Label))
	case constants.BIOSGrubPartitionLabel:
		return nil
	case constants.BootPartitionLabel:
		log.Printf("formatting partition %q as %q with label %q\n", t.PartitionName, "xfs", t.Label)
		opts := []xfs.Option{xfs.WithForce(t.Force)}

		if t.Label != "" {
			opts = append(opts, xfs.WithLabel(t.Label))
		}

		return xfs.MakeFS(t.PartitionName, opts...)
	case constants.MetaPartitionLabel:
		return nil
	case constants.StatePartitionLabel:
		log.Printf("formatting partition %q as %q with label %q\n", t.PartitionName, "xfs", t.Label)
		opts := []xfs.Option{xfs.WithForce(t.Force)}

		if t.Label != "" {
			opts = append(opts, xfs.WithLabel(t.Label))
		}

		return xfs.MakeFS(t.PartitionName, opts...)
	case constants.EphemeralPartitionLabel:
		log.Printf("formatting partition %q as %q with label %q\n", t.PartitionName, "xfs", t.Label)
		opts := []xfs.Option{xfs.WithForce(t.Force)}

		if t.Label != "" {
			opts = append(opts, xfs.WithLabel(t.Label))
		}

		return xfs.MakeFS(t.PartitionName, opts...)
	default:
		return nil
	}
}

// Save copies the assets to the bootloader partition.
func (t *Target) Save() (err error) {
	for _, asset := range t.Assets {
		var (
			sourceFile *os.File
			destFile   *os.File
		)

		if sourceFile, err = os.Open(asset.Source); err != nil {
			return err
		}
		// nolint: errcheck
		defer sourceFile.Close()

		if err = os.MkdirAll(filepath.Dir(asset.Destination), os.ModeDir); err != nil {
			return err
		}

		if destFile, err = os.Create(asset.Destination); err != nil {
			return err
		}

		// nolint: errcheck
		defer destFile.Close()

		log.Printf("copying %s to %s\n", sourceFile.Name(), destFile.Name())

		if _, err = io.Copy(destFile, sourceFile); err != nil {
			log.Printf("failed to copy %s to %s\n", sourceFile.Name(), destFile.Name())
			return err
		}

		if err = destFile.Close(); err != nil {
			log.Printf("failed to close %s", destFile.Name())
			return err
		}

		if err = sourceFile.Close(); err != nil {
			log.Printf("failed to close %s", sourceFile.Name())
			return err
		}
	}

	return nil
}
