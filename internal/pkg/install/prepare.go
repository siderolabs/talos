/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package install

import (
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"unsafe"

	"github.com/pkg/errors"
	"github.com/talos-systems/talos/internal/pkg/blockdevice"
	"github.com/talos-systems/talos/internal/pkg/blockdevice/filesystem/vfat"
	"github.com/talos-systems/talos/internal/pkg/blockdevice/filesystem/xfs"
	"github.com/talos-systems/talos/internal/pkg/blockdevice/probe"
	"github.com/talos-systems/talos/internal/pkg/blockdevice/table"
	"github.com/talos-systems/talos/internal/pkg/blockdevice/table/gpt/partition"
	"github.com/talos-systems/talos/internal/pkg/constants"
	"github.com/talos-systems/talos/internal/pkg/version"
	"github.com/talos-systems/talos/pkg/userdata"
	"golang.org/x/sys/unix"
)

const (
	// DefaultSizeRootDevice is the default size of the root partition.
	// TODO(andrewrynhard): We should inspect the tarball's uncompressed size and dynamically set the root partition's size.
	DefaultSizeRootDevice = 2048 * 1000 * 1000

	// DefaultSizeDataDevice is the default size of the data partition.
	DefaultSizeDataDevice = 1024 * 1000 * 1000

	// DefaultSizeBootDevice is the default size of the boot partition.
	// TODO(andrewrynhard): We should inspect the sizes of the artifacts and dynamically set the boot partition's size.
	DefaultSizeBootDevice = 512 * 1000 * 1000
)

var (
	// DefaultURLBase is the base URL for all default artifacts.
	// TODO(andrewrynhard): We need to setup infrastructure for publishing artifacts and not depend on GitHub.
	DefaultURLBase = "https://github.com/talos-systems/talos/releases/download/" + version.Tag

	// DefaultRootfsURL is the URL to the rootfs.
	DefaultRootfsURL = DefaultURLBase + "/rootfs.tar.gz"

	// DefaultKernelURL is the URL to the kernel.
	DefaultKernelURL = DefaultURLBase + "/vmlinuz"

	// DefaultInitramfsURL is the URL to the initramfs.
	DefaultInitramfsURL = DefaultURLBase + "/initramfs.xz"
)

// Prepare handles setting/consolidating/defaulting userdata pieces specific to
// installation
// TODO: See if this would be more appropriate in userdata
// nolint: dupl, gocyclo
func Prepare(data *userdata.UserData) (err error) {
	if data.Install == nil {
		return nil
	}

	var exists bool
	if exists, err = Exists(data.Install.Boot.InstallDevice.Device); err != nil {
		return err
	}

	if exists {
		log.Println("found existing installation, skipping prepare step")
		return nil
	}

	// Verify that the target device(s) can satisify the requested options.

	if err = VerifyRootDevice(data); err != nil {
		return errors.Wrap(err, "failed to prepare root device")
	}
	if err = VerifyDataDevice(data); err != nil {
		return errors.Wrap(err, "failed to prepare data device")
	}
	if err = VerifyBootDevice(data); err != nil {
		return errors.Wrap(err, "failed to prepare boot device")
	}

	manifest := NewManifest(data)

	if data.Install.Wipe {
		if err = WipeDevices(manifest); err != nil {
			return errors.Wrap(err, "failed to wipe device(s)")
		}
	}

	// Create and format all partitions.

	if err = ExecuteManifest(data, manifest); err != nil {
		return err
	}

	return err
}

// ExecuteManifest partitions and formats all disks in a manifest.
func ExecuteManifest(data *userdata.UserData, manifest *Manifest) (err error) {
	for dev, targets := range manifest.Targets {
		var bd *blockdevice.BlockDevice
		if bd, err = blockdevice.Open(dev, blockdevice.WithNewGPT(data.Install.Force)); err != nil {
			return err
		}
		// nolint: errcheck
		defer bd.Close()

		for _, target := range targets {
			if err = target.Partition(bd); err != nil {
				return errors.Wrap(err, "failed to partition device")
			}
		}

		if err = bd.RereadPartitionTable(); err != nil {
			return err
		}

		for _, target := range targets {
			if err = target.Format(); err != nil {
				return errors.Wrap(err, "failed to format device")
			}
		}
	}

	return nil
}

// VerifyRootDevice verifies the supplied root device options.
func VerifyRootDevice(data *userdata.UserData) (err error) {
	if data.Install == nil || data.Install.Root == nil {
		return errors.New("a root device is required")
	}

	if data.Install.Root.Device == "" {
		return errors.New("a root device is required")
	}

	if data.Install.Root.Size == 0 {
		data.Install.Root.Size = DefaultSizeRootDevice
	}

	if data.Install.Root.Rootfs == "" {
		data.Install.Root.Rootfs = DefaultRootfsURL
	}

	if !data.Install.Force {
		if err = VerifyDiskAvailability(constants.CurrentRootPartitionLabel()); err != nil {
			return errors.Wrap(err, "failed to verify disk availability")
		}
	}

	return nil
}

// VerifyDataDevice verifies the supplied data device options.
func VerifyDataDevice(data *userdata.UserData) (err error) {
	// Set data device to root device if not specified
	if data.Install.Data == nil {
		data.Install.Data = &userdata.InstallDevice{}
	}

	if data.Install.Data.Device == "" {
		// We can safely assume root device is defined at this point
		// because VerifyRootDevice should have been called first in
		// in the chain
		data.Install.Data.Device = data.Install.Root.Device
	}

	if data.Install.Data.Size == 0 {
		data.Install.Data.Size = DefaultSizeDataDevice
	}

	if !data.Install.Force {
		if err = VerifyDiskAvailability(constants.DataPartitionLabel); err != nil {
			return errors.Wrap(err, "failed to verify disk availability")
		}
	}

	return nil
}

// VerifyBootDevice verifies the supplied boot device options.
func VerifyBootDevice(data *userdata.UserData) (err error) {
	if data.Install.Boot == nil {
		return nil
	}

	if data.Install.Boot.Device == "" {
		// We can safely assume root device is defined at this point
		// because VerifyRootDevice should have been called first in
		// in the chain
		data.Install.Boot.Device = data.Install.Root.Device
	}

	if data.Install.Boot.Size == 0 {
		data.Install.Boot.Size = DefaultSizeBootDevice
	}

	if data.Install.Boot.Kernel == "" {
		data.Install.Boot.Kernel = DefaultKernelURL
	}

	if data.Install.Boot.Initramfs == "" {
		data.Install.Boot.Initramfs = DefaultInitramfsURL
	}

	if !data.Install.Force {
		if err = VerifyDiskAvailability(constants.BootPartitionLabel); err != nil {
			return errors.Wrap(err, "failed to verify disk availability")
		}
	}
	return nil
}

// VerifyDiskAvailability verifies that no filesystems currently exist with
// the labels used by the OS.
func VerifyDiskAvailability(label string) (err error) {
	var dev *probe.ProbedBlockDevice
	if dev, err = probe.GetDevWithFileSystemLabel(label); err != nil {
		// We return here because we only care if we can discover the
		// device successfully and confirm that the disk is not in use.
		// TODO(andrewrynhard): We should return a custom error type here
		// that we can use to confirm the device was not found.
		return nil
	}
	if dev.SuperBlock != nil {
		return errors.Errorf("target install device %s is not empty, found existing %s file system", label, dev.SuperBlock.Type())
	}

	return nil
}

// WipeDevices writes zeros to each block device in the preparation manifest.
func WipeDevices(manifest *Manifest) (err error) {
	var zero *os.File
	if zero, err = os.Open("/dev/zero"); err != nil {
		return err
	}

	for dev := range manifest.Targets {
		var f *os.File
		if f, err = os.OpenFile(dev, os.O_RDWR, os.ModeDevice); err != nil {
			return err
		}
		var size uint64
		if _, _, ret := unix.Syscall(unix.SYS_IOCTL, f.Fd(), unix.BLKGETSIZE64, uintptr(unsafe.Pointer(&size))); ret != 0 {
			return errors.Errorf("failed to got block device size: %v", ret)
		}
		if _, err = io.CopyN(f, zero, int64(size)); err != nil {
			return err
		}

		if err = f.Close(); err != nil {
			return err
		}
	}

	if err = zero.Close(); err != nil {
		return err
	}

	return nil
}

// NewManifest initializes and returns a Manifest.
func NewManifest(data *userdata.UserData) (manifest *Manifest) {
	manifest = &Manifest{
		Targets: map[string][]*Target{},
	}

	// Initialize any slices we need. Note that a boot paritition is not
	// required.

	if manifest.Targets[data.Install.Root.Device] == nil {
		manifest.Targets[data.Install.Root.Device] = []*Target{}
	}
	if manifest.Targets[data.Install.Data.Device] == nil {
		manifest.Targets[data.Install.Data.Device] = []*Target{}
	}

	var bootTarget *Target
	if data.Install.Boot != nil {
		bootTarget = &Target{
			Device: data.Install.Boot.Device,
			Label:  constants.BootPartitionLabel,
			Size:   data.Install.Boot.Size,
			Force:  data.Install.Force,
			Test:   false,
			Assets: []*Asset{
				{
					Source:      data.Install.Boot.Kernel,
					Destination: filepath.Join("/", constants.CurrentRootPartitionLabel(), filepath.Base(data.Install.Boot.Kernel)),
				},
				{
					Source:      data.Install.Boot.Initramfs,
					Destination: filepath.Join("/", constants.CurrentRootPartitionLabel(), filepath.Base(data.Install.Boot.Initramfs)),
				},
			},
			MountPoint: path.Join(constants.NewRoot, constants.BootMountPoint),
		}
	}

	rootATarget := &Target{
		Device: data.Install.Root.Device,
		Label:  constants.RootAPartitionLabel,
		Size:   data.Install.Root.Size,
		Force:  data.Install.Force,
		Test:   false,
		Assets: []*Asset{
			{
				Source:      data.Install.Root.Rootfs,
				Destination: filepath.Base(data.Install.Root.Rootfs),
			},
		},
		MountPoint: path.Join(constants.NewRoot, constants.RootMountPoint),
	}

	rootBTarget := &Target{
		Device:     data.Install.Root.Device,
		Label:      constants.RootBPartitionLabel,
		Size:       data.Install.Root.Size,
		Force:      data.Install.Force,
		Test:       false,
		MountPoint: path.Join(constants.NewRoot, constants.RootMountPoint),
	}

	dataTarget := &Target{
		Device:     data.Install.Data.Device,
		Label:      constants.DataPartitionLabel,
		Size:       data.Install.Data.Size,
		Force:      data.Install.Force,
		Test:       false,
		MountPoint: path.Join(constants.NewRoot, constants.DataMountPoint),
	}

	for _, target := range []*Target{bootTarget, rootATarget, rootBTarget, dataTarget} {
		if target == nil {
			continue
		}
		manifest.Targets[target.Device] = append(manifest.Targets[target.Device], target)
	}

	for _, extra := range data.Install.ExtraDevices {
		if manifest.Targets[extra.Device] == nil {
			manifest.Targets[extra.Device] = []*Target{}
		}

		for _, part := range extra.Partitions {
			extraTarget := &Target{
				Device: extra.Device,
				Size:   part.Size,
				Force:  data.Install.Force,
				Test:   false,
			}

			manifest.Targets[extra.Device] = append(manifest.Targets[extra.Device], extraTarget)
		}
	}

	return manifest
}

// Manifest represents the instructions for preparing all block devices
// for an installation.
type Manifest struct {
	Targets map[string][]*Target
}

// Target represents an installation partition.
type Target struct {
	Label          string
	MountPoint     string
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

// Partition creates a new partition on the specified device
// nolint: dupl, gocyclo
func (t *Target) Partition(bd *blockdevice.BlockDevice) (err error) {
	log.Printf("partitioning %s - %s\n", t.Device, t.Label)

	var pt table.PartitionTable
	if pt, err = bd.PartitionTable(true); err != nil {
		return err
	}

	opts := []interface{}{partition.WithPartitionTest(t.Test)}

	switch t.Label {
	case constants.BootPartitionLabel:
		// EFI System Partition
		typeID := "C12A7328-F81F-11D2-BA4B-00A0C93EC93B"
		opts = append(opts, partition.WithPartitionType(typeID), partition.WithPartitionName(t.Label), partition.WithLegacyBIOSBootableAttribute(true))
	case constants.CurrentRootPartitionLabel():
		// Root Partition
		var typeID string
		switch runtime.GOARCH {
		case "386":
			typeID = "44479540-F297-41B2-9AF7-D131D5F0458A"
		case "amd64":
			typeID = "4F68BCE3-E8CD-4DB1-96E7-FBCAF984B709"
		case "arm64":
			typeID = "B921B045-1DF0-41C3-AF44-4C6F280D3FAE"
		default:
			return errors.Errorf("%s", "unsupported cpu architecture")
		}
		opts = append(opts, partition.WithPartitionType(typeID), partition.WithPartitionName(t.Label))
	case constants.DataPartitionLabel:
		// Data Partition
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
	case strings.HasPrefix(dev, "/dev/loop"):
		t.PartitionName = t.Device + "p" + strconv.Itoa(int(part.No()))
	default:
		t.PartitionName = t.Device + strconv.Itoa(int(part.No()))
	}

	return nil
}

// Format creates a xfs filesystem on the device/partition
func (t *Target) Format() error {
	log.Printf("formatting partition %s - %s\n", t.PartitionName, t.Label)
	if t.Label == constants.BootPartitionLabel {
		return vfat.MakeFS(t.PartitionName, vfat.WithLabel(t.Label))
	}
	opts := []xfs.Option{xfs.WithForce(t.Force)}
	if t.Label != "" {
		opts = append(opts, xfs.WithLabel(t.Label))
	}
	return xfs.MakeFS(t.PartitionName, opts...)
}

// Install handles downloading the necessary assets and extracting them to
// the appropriate locations
// nolint: gocyclo
func (t *Target) Install() error {
	// Download and extract all artifacts.
	var sourceFile *os.File
	var destFile *os.File
	var err error

	if err = os.MkdirAll(t.MountPoint, os.ModeDir); err != nil {
		return err
	}

	// Extract artifact if necessary, otherwise place at root of partition/filesystem
	for _, asset := range t.Assets {
		var u *url.URL
		if u, err = url.Parse(asset.Source); err != nil {
			return err
		}

		sourceFile = nil
		destFile = nil

		// Handle fetching of asset
		switch u.Scheme {
		case "http":
			fallthrough
		case "https":
			log.Printf("downloading %s\n", asset.Source)
			dest := filepath.Join(t.MountPoint, asset.Destination)
			sourceFile, err = download(u.String(), dest)
			if err != nil {
				return err
			}
		case "file":
			source := u.Path
			dest := filepath.Join(t.MountPoint, asset.Destination)

			log.Printf("copying %s to %s\n", asset.Source, dest)
			sourceFile, err = os.Open(source)
			if err != nil {
				return err
			}

			if err = os.MkdirAll(filepath.Dir(dest), 0700); err != nil {
				return err
			}

			destFile, err = os.Create(dest)
			if err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupported path scheme, got %s but supported %s", u.Scheme, "https://|http://|file://")
		}

		// Handle extraction/Installation
		switch {
		// tar
		case strings.HasSuffix(sourceFile.Name(), ".tar") || strings.HasSuffix(sourceFile.Name(), ".tar.gz"):
			log.Printf("extracting %s to %s\n", sourceFile.Name(), t.MountPoint)

			err = untar(sourceFile, t.MountPoint)
			if err != nil {
				log.Printf("failed to extract %s to %s\n", sourceFile.Name(), t.MountPoint)
				return err
			}

			if err = sourceFile.Close(); err != nil {
				log.Printf("failed to close %s", sourceFile.Name())
				return err
			}

			if err = os.Remove(sourceFile.Name()); err != nil {
				log.Printf("failed to remove %s", sourceFile.Name())
				return err
			}
		// single file
		case strings.HasPrefix(sourceFile.Name(), "/") && destFile != nil:
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
		default:
			if err = sourceFile.Close(); err != nil {
				log.Printf("failed to close %s", sourceFile.Name())
				return err
			}
		}
	}

	return nil
}
