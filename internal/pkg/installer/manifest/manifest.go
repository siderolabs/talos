/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package manifest

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pkg/errors"

	"github.com/talos-systems/talos/pkg/blockdevice"
	"github.com/talos-systems/talos/pkg/blockdevice/filesystem/vfat"
	"github.com/talos-systems/talos/pkg/blockdevice/filesystem/xfs"
	"github.com/talos-systems/talos/pkg/blockdevice/table"
	"github.com/talos-systems/talos/pkg/blockdevice/table/gpt/partition"
	"github.com/talos-systems/talos/pkg/config/machine"
	"github.com/talos-systems/talos/pkg/constants"
)

const (
	// DefaultSizeBootDevice is the default size of the boot partition.
	DefaultSizeBootDevice = 512 * 1000 * 1000
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
func NewManifest(install machine.Install) (manifest *Manifest, err error) {
	manifest = &Manifest{
		Targets: map[string][]*Target{},
	}

	// Verify that the target device(s) can satisify the requested options.

	if err = VerifyDataDevice(install); err != nil {
		return nil, errors.Wrap(err, "failed to prepare ephemeral partition")
	}

	if err = VerifyBootDevice(install); err != nil {
		return nil, errors.Wrap(err, "failed to prepare boot partition")
	}

	// Initialize any slices we need. Note that a boot paritition is not
	// required.

	if manifest.Targets[install.Disk()] == nil {
		manifest.Targets[install.Disk()] = []*Target{}
	}

	var bootTarget *Target
	if install.WithBootloader() {
		bootTarget = &Target{
			Device: install.Disk(),
			Label:  constants.BootPartitionLabel,
			Size:   512 * 1024 * 1024,
			Force:  true,
			Test:   false,
			Assets: []*Asset{
				{
					Source:      constants.KernelAssetPath,
					Destination: filepath.Join(constants.BootMountPoint, "default", constants.KernelAsset),
				},
				{
					Source:      constants.InitramfsAssetPath,
					Destination: filepath.Join(constants.BootMountPoint, "default", constants.InitramfsAsset),
				},
			},
		}
	}

	ephemeralTarget := &Target{
		Device: install.Disk(),
		Label:  constants.EphemeralPartitionLabel,
		Size:   16 * 1024 * 1024,
		Force:  true,
		Test:   false,
	}

	for _, target := range []*Target{bootTarget, ephemeralTarget} {
		if target == nil {
			continue
		}

		manifest.Targets[target.Device] = append(manifest.Targets[target.Device], target)
	}

	for _, extra := range install.ExtraDisks() {
		if manifest.Targets[extra.Device] == nil {
			manifest.Targets[extra.Device] = []*Target{}
		}

		for _, part := range extra.Partitions {
			extraTarget := &Target{
				Device: extra.Device,
				Size:   part.Size,
				Force:  true,
				Test:   false,
			}

			manifest.Targets[extra.Device] = append(manifest.Targets[extra.Device], extraTarget)
		}
	}

	return manifest, nil
}

// ExecuteManifest partitions and formats all disks in a manifest.
func (m *Manifest) ExecuteManifest(manifest *Manifest) (err error) {
	for dev, targets := range manifest.Targets {
		var bd *blockdevice.BlockDevice

		if bd, err = blockdevice.Open(dev, blockdevice.WithNewGPT(true)); err != nil {
			return err
		}

		// nolint: errcheck
		defer bd.Close()

		for _, target := range targets {
			if err = target.Partition(bd); err != nil {
				return errors.Wrap(err, "failed to partition device")
			}
		}

		for _, target := range targets {
			if err = target.Format(); err != nil {
				return errors.Wrap(err, "failed to format device")
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

	if pt, err = bd.PartitionTable(true); err != nil {
		return err
	}

	opts := []interface{}{}

	switch t.Label {
	case constants.BootPartitionLabel:
		// EFI System Partition
		typeID := "C12A7328-F81F-11D2-BA4B-00A0C93EC93B"
		opts = append(opts, partition.WithPartitionType(typeID), partition.WithPartitionName(t.Label), partition.WithLegacyBIOSBootableAttribute(true))
	case constants.EphemeralPartitionLabel:
		// Ephemeral Partition
		typeID := "AF3DC60F-8384-7247-8E79-3D69D8477DE4"
		opts = append(opts, partition.WithPartitionType(typeID), partition.WithPartitionName(t.Label))
	default:
		typeID := "AF3DC60F-8384-7247-8E79-3D69D8477DE4"
		opts = append(opts, partition.WithPartitionType(typeID))
	}

	part, err := pt.Add(uint64(t.Size), opts...)
	if err != nil {
		return err
	}

	if err = pt.Write(); err != nil {
		return err
	}

	// TODO(andrewrynhard): We should really have a custom type that has all
	// the methods we need. This switch statement shows up in some form in
	// multiple places.
	switch dev := t.Device; {
	case strings.HasPrefix(dev, "/dev/nvme"):
		fallthrough
	case strings.HasPrefix(dev, "/dev/loop"):
		t.PartitionName = t.Device + "p" + strconv.Itoa(int(part.No()))
	default:
		t.PartitionName = t.Device + strconv.Itoa(int(part.No()))
	}

	return nil
}

// Format creates a filesystem on the device/partition.
func (t *Target) Format() error {
	if t.Label == constants.BootPartitionLabel {
		log.Printf("formatting partition %s - %s as %s\n", t.PartitionName, t.Label, "fat")
		return vfat.MakeFS(t.PartitionName, vfat.WithLabel(t.Label))
	}

	log.Printf("formatting partition %s - %s as %s\n", t.PartitionName, t.Label, "xfs")
	opts := []xfs.Option{xfs.WithForce(t.Force)}

	if t.Label != "" {
		opts = append(opts, xfs.WithLabel(t.Label))
	}

	return xfs.MakeFS(t.PartitionName, opts...)
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
