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

	"github.com/siderolabs/go-blockdevice/blockdevice/partition/gpt"
	"github.com/siderolabs/go-blockdevice/blockdevice/util"
	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/internal/pkg/mount"
	"github.com/siderolabs/talos/internal/pkg/partition"
	"github.com/siderolabs/talos/pkg/archiver"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// Target represents an installation partition.
//
//nolint:maligned
type Target struct {
	*partition.FormatOptions
	*partition.Options
	Device string

	LegacyBIOSBootable bool

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

// ParseTarget parses the target from the label and creates a required target.
func ParseTarget(label, deviceName string) (*Target, error) {
	switch label {
	case constants.EFIPartitionLabel:
		return EFITarget(deviceName, nil), nil
	case constants.BIOSGrubPartitionLabel:
		return BIOSTarget(deviceName, nil), nil
	case constants.BootPartitionLabel:
		return BootTarget(deviceName, nil), nil
	case constants.MetaPartitionLabel:
		return MetaTarget(deviceName, nil), nil
	case constants.StatePartitionLabel:
		return StateTarget(deviceName, NoFilesystem), nil
	case constants.EphemeralPartitionLabel:
		return EphemeralTarget(deviceName, NoFilesystem), nil
	default:
		return nil, fmt.Errorf("label %q is not supported", label)
	}
}

// EFITarget builds the default EFI target.
func EFITarget(device string, extra *Target) *Target {
	target := &Target{
		FormatOptions: partition.NewFormatOptions(constants.EFIPartitionLabel),
		Options:       partition.NewPartitionOptions(constants.EFIPartitionLabel, false),
		Device:        device,
	}

	return target.enhance(extra)
}

// EFITargetUKI builds the default EFI UKI target.
func EFITargetUKI(device string, extra *Target) *Target {
	target := &Target{
		FormatOptions: partition.NewFormatOptions(constants.EFIPartitionLabel),
		Options:       partition.NewPartitionOptions(constants.EFIPartitionLabel, true),
		Device:        device,
	}

	return target.enhance(extra)
}

// BIOSTarget builds the default BIOS target.
func BIOSTarget(device string, extra *Target) *Target {
	target := &Target{
		FormatOptions:      partition.NewFormatOptions(constants.BIOSGrubPartitionLabel),
		Options:            partition.NewPartitionOptions(constants.BIOSGrubPartitionLabel, false),
		Device:             device,
		LegacyBIOSBootable: true,
	}

	return target.enhance(extra)
}

// BootTarget builds the default boot target.
func BootTarget(device string, extra *Target) *Target {
	target := &Target{
		FormatOptions: partition.NewFormatOptions(constants.BootPartitionLabel),
		Options:       partition.NewPartitionOptions(constants.BootPartitionLabel, false),
		Device:        device,
	}

	return target.enhance(extra)
}

// MetaTarget builds the default meta target.
func MetaTarget(device string, extra *Target) *Target {
	target := &Target{
		FormatOptions: partition.NewFormatOptions(constants.MetaPartitionLabel),
		Options:       partition.NewPartitionOptions(constants.MetaPartitionLabel, false),
		Device:        device,
	}

	return target.enhance(extra)
}

// StateTarget builds the default state target.
func StateTarget(device string, extra *Target) *Target {
	target := &Target{
		FormatOptions: partition.NewFormatOptions(constants.StatePartitionLabel),
		Options:       partition.NewPartitionOptions(constants.StatePartitionLabel, false),
		Device:        device,
	}

	return target.enhance(extra)
}

// EphemeralTarget builds the default ephemeral target.
func EphemeralTarget(device string, extra *Target) *Target {
	target := &Target{
		FormatOptions: partition.NewFormatOptions(constants.EphemeralPartitionLabel),
		Options:       partition.NewPartitionOptions(constants.EphemeralPartitionLabel, false),
		Device:        device,
	}

	return target.enhance(extra)
}

func (t *Target) enhance(extra *Target) *Target {
	if extra == nil {
		return t
	}

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
	part, err := partition.Locate(pt, t.Label)
	if err != nil {
		return nil, err
	}

	t.PartitionName, err = part.Path()
	if err != nil {
		return nil, err
	}

	return part, nil
}

// partition creates a new partition on the specified device.
func (t *Target) partition(pt *gpt.GPT, pos int) (err error) {
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

	partitionName, err := partition.Partition(pt, pos, t.Device, partition.Options{
		PartitionLabel:     t.Label,
		PartitionType:      t.PartitionType,
		Size:               t.Size,
		LegacyBIOSBootable: t.LegacyBIOSBootable,
	})
	if err != nil {
		return err
	}

	t.PartitionName = partitionName

	return nil
}

// Format creates a filesystem on the device/partition.
func (t *Target) Format() error {
	if t.Skip {
		return nil
	}

	return partition.Format(t.PartitionName, t.FormatOptions)
}

// GetLabel returns the underlaying partition label.
func (t *Target) GetLabel() string {
	return t.Label
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
