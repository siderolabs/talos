// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package install

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/dustin/go-humanize"
	"github.com/talos-systems/go-blockdevice/blockdevice"
	"github.com/talos-systems/go-blockdevice/blockdevice/partition/gpt"
	"github.com/talos-systems/go-blockdevice/blockdevice/util"
	"golang.org/x/sys/unix"

	"github.com/talos-systems/talos/internal/pkg/mount"
	"github.com/talos-systems/talos/internal/pkg/partition"
	"github.com/talos-systems/talos/pkg/archiver"
	"github.com/talos-systems/talos/pkg/machinery/constants"
)

// Target represents an installation partition.
//
//nolint:maligned
type Target struct {
	*partition.FormatOptions
	Device string

	LegacyBIOSBootable bool

	Assets []*Asset

	// Preserve contents of the partition with the same label (if it exists).
	PreserveContents bool

	// Extra preserved locations (for upgrading from older versions of Talos).
	//
	// Used only if PreserveContents is true.
	ExtraPreserveSources []PreserveSource

	// Skip makes manifest skip any actions with the partition (creating, formatting).
	//
	// Skipped partitions should exist on the disk by the time manifest execution starts.
	Skip bool

	// set during execution
	PartitionName string
	Contents      *bytes.Buffer
}

// Asset represents a file required by a target.
type Asset struct {
	Source      string
	Destination string
}

// PreserveSource instructs Talos where to look for source files to preserve.
type PreserveSource struct {
	Label          string
	FnmatchFilters []string
	FileSystemType partition.FileSystemType
}

// NoFilesystem preset to override default filesystem type to none.
var NoFilesystem = &Target{
	FormatOptions: &partition.FormatOptions{
		FileSystemType: partition.FilesystemTypeNone,
	},
}

// EFITarget builds the default EFI target.
func EFITarget(device string, extra *Target) *Target {
	target := &Target{
		FormatOptions: partition.NewFormatOptions(constants.EFIPartitionLabel),
		Device:        device,
	}

	return target.enhance(extra)
}

// BIOSTarget builds the default BIOS target.
func BIOSTarget(device string, extra *Target) *Target {
	target := &Target{
		FormatOptions:      partition.NewFormatOptions(constants.BIOSGrubPartitionLabel),
		Device:             device,
		LegacyBIOSBootable: true,
	}

	return target.enhance(extra)
}

// BootTarget builds the default boot target.
func BootTarget(device string, extra *Target) *Target {
	target := &Target{
		FormatOptions: partition.NewFormatOptions(constants.BootPartitionLabel),
		Device:        device,
	}

	return target.enhance(extra)
}

// MetaTarget builds the default meta target.
func MetaTarget(device string, extra *Target) *Target {
	target := &Target{
		FormatOptions: partition.NewFormatOptions(constants.MetaPartitionLabel),
		Device:        device,
	}

	return target.enhance(extra)
}

// StateTarget builds the default state target.
func StateTarget(device string, extra *Target) *Target {
	target := &Target{
		FormatOptions: partition.NewFormatOptions(constants.StatePartitionLabel),
		Device:        device,
	}

	return target.enhance(extra)
}

// EphemeralTarget builds the default ephemeral target.
func EphemeralTarget(device string, extra *Target) *Target {
	target := &Target{
		FormatOptions: partition.NewFormatOptions(constants.EphemeralPartitionLabel),
		Device:        device,
	}

	return target.enhance(extra)
}

func (t *Target) enhance(extra *Target) *Target {
	if extra == nil {
		return t
	}

	t.Assets = extra.Assets
	t.PreserveContents = extra.PreserveContents
	t.ExtraPreserveSources = extra.ExtraPreserveSources
	t.Skip = extra.Skip

	if extra.FormatOptions != nil {
		t.FormatOptions.FileSystemType = extra.FormatOptions.FileSystemType
	}

	return t
}

func (t *Target) String() string {
	return fmt.Sprintf("%s (%q)", t.PartitionName, t.Label)
}

// Locate existing partition on the disk.
func (t *Target) Locate(pt *gpt.GPT) (*gpt.Partition, error) {
	for _, part := range pt.Partitions().Items() {
		if part.Name == t.Label {
			var err error

			t.PartitionName, err = part.Path()
			if err != nil {
				return part, err
			}

			return part, nil
		}
	}

	return nil, nil
}

// Partition creates a new partition on the specified device.
func (t *Target) Partition(pt *gpt.GPT, pos int, bd *blockdevice.BlockDevice) (err error) {
	if t.Skip {
		part := pt.Partitions().FindByName(t.Label)
		if part != nil {
			log.Printf("skipped %s (%s) size %d blocks", t.PartitionName, t.Label, part.Length())

			t.PartitionName, err = part.Path()
			if err != nil {
				return err
			}
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

	t.PartitionName, err = part.Path()
	if err != nil {
		return err
	}

	log.Printf("created %s (%s) size %d blocks", t.PartitionName, t.Label, part.Length())

	return nil
}

// Format creates a filesystem on the device/partition.
func (t *Target) Format() error {
	if t.Skip {
		return nil
	}

	return partition.Format(t.PartitionName, t.FormatOptions)
}

// Save copies the assets to the bootloader partition.
func (t *Target) Save() (err error) {
	for _, asset := range t.Assets {
		asset := asset

		err = func() error {
			var (
				sourceFile *os.File
				destFile   *os.File
			)

			if sourceFile, err = os.Open(asset.Source); err != nil {
				return err
			}
			//nolint:errcheck
			defer sourceFile.Close()

			if err = os.MkdirAll(filepath.Dir(asset.Destination), os.ModeDir); err != nil {
				return err
			}

			if destFile, err = os.Create(asset.Destination); err != nil {
				return err
			}

			//nolint:errcheck
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

			return nil
		}()

		if err != nil {
			return err
		}
	}

	return nil
}

func withTemporaryMounted(partPath string, flags uintptr, fileSystemType partition.FileSystemType, label string, f func(mountPath string) error) error {
	mountPath := filepath.Join(constants.SystemPath, "mnt")

	mountpoints := mount.NewMountPoints()

	mountpoint := mount.NewMountPoint(partPath, mountPath, fileSystemType, unix.MS_NOATIME|flags, "")
	mountpoints.Set(label, mountpoint)

	if err := mount.Mount(mountpoints); err != nil {
		return fmt.Errorf("failed to mount %q: %w", partPath, err)
	}

	defer func() {
		if err := mount.Unmount(mountpoints); err != nil {
			log.Printf("failed to unmount: %s", err)
		}
	}()

	return f(mountPath)
}

// SaveContents saves contents of partition to the target (in-memory).
func (t *Target) SaveContents(device Device, source *gpt.Partition, fileSystemType partition.FileSystemType, fnmatchFilters []string) error {
	partPath, err := util.PartPath(device.Device, int(source.Number))
	if err != nil {
		return err
	}

	if fileSystemType == partition.FilesystemTypeNone {
		err = t.saveRawContents(partPath)
	} else {
		err = t.saveFilesystemContents(partPath, fileSystemType, fnmatchFilters)
	}

	if err != nil {
		t.Contents = nil

		return err
	}

	log.Printf("preserved contents of %q: %d bytes", t.Label, t.Contents.Len())

	return nil
}

func (t *Target) saveRawContents(partPath string) error {
	src, err := os.Open(partPath)
	if err != nil {
		return fmt.Errorf("error opening source partition: %q", err)
	}

	defer src.Close() //nolint:errcheck

	t.Contents = bytes.NewBuffer(nil)

	zw := gzip.NewWriter(t.Contents)
	defer zw.Close() //nolint:errcheck

	_, err = io.Copy(zw, src)
	if err != nil {
		return fmt.Errorf("error copying partition %q contents: %w", partPath, err)
	}

	return src.Close()
}

func (t *Target) saveFilesystemContents(partPath string, fileSystemType partition.FileSystemType, fnmatchFilters []string) error {
	t.Contents = bytes.NewBuffer(nil)

	return withTemporaryMounted(partPath, unix.MS_RDONLY, fileSystemType, t.Label, func(mountPath string) error {
		return archiver.TarGz(context.TODO(), mountPath, t.Contents, archiver.WithFnmatchPatterns(fnmatchFilters...))
	})
}

// RestoreContents restores previously saved contents to the disk.
func (t *Target) RestoreContents() error {
	if t.Contents == nil {
		return nil
	}

	var err error

	if t.FileSystemType == partition.FilesystemTypeNone {
		err = t.restoreRawContents()
	} else {
		err = t.restoreFilesystemContents()
	}

	t.Contents = nil

	if err != nil {
		return err
	}

	log.Printf("restored contents of %q", t.Label)

	return nil
}

func (t *Target) restoreRawContents() error {
	dst, err := os.OpenFile(t.PartitionName, os.O_WRONLY, 0)
	if err != nil {
		return fmt.Errorf("error opening source partition: %q", err)
	}

	defer dst.Close() //nolint:errcheck

	zr, err := gzip.NewReader(t.Contents)
	if err != nil {
		return err
	}

	_, err = io.Copy(dst, zr)
	if err != nil {
		return fmt.Errorf("error restoring partition %q contents: %w", t.PartitionName, err)
	}

	return dst.Close()
}

func (t *Target) restoreFilesystemContents() error {
	return withTemporaryMounted(t.PartitionName, 0, t.FileSystemType, t.Label, func(mountPath string) error {
		return archiver.UntarGz(context.TODO(), t.Contents, mountPath)
	})
}
