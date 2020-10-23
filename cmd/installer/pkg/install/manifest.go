// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package install

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/talos-systems/go-retry/retry"
	"golang.org/x/sys/unix"

	"github.com/talos-systems/go-blockdevice/blockdevice"
	"github.com/talos-systems/go-blockdevice/blockdevice/table"
	"github.com/talos-systems/go-blockdevice/blockdevice/table/gpt/partition"
	"github.com/talos-systems/go-blockdevice/blockdevice/util"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/internal/pkg/mount"
	"github.com/talos-systems/talos/pkg/archiver"
	"github.com/talos-systems/talos/pkg/machinery/constants"
	"github.com/talos-systems/talos/pkg/makefs"
)

// Manifest represents the instructions for preparing all block devices
// for an installation.
type Manifest struct {
	Devices map[string]Device
	Targets map[string][]*Target
}

// Device represents device options.
type Device struct {
	Device string

	ResetPartitionTable bool
	Zero                bool
}

// Target represents an installation partition.
//
//nolint: go-lint
type Target struct {
	Device string

	Label              string
	PartitionType      PartitionType
	FileSystemType     FileSystemType
	LegacyBIOSBootable bool

	Size   uint
	Force  bool
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
	FileSystemType FileSystemType
}

// PartitionType in partition table.
type PartitionType = string

// GPT partition types.
//
// TODO: should be moved into the blockdevice library.
const (
	EFISystemPartition  PartitionType = "C12A7328-F81F-11D2-BA4B-00A0C93EC93B"
	BIOSBootPartition   PartitionType = "21686148-6449-6E6F-744E-656564454649"
	LinuxFilesystemData PartitionType = "0FC63DAF-8483-4772-8E79-3D69D8477DE4"
)

// FileSystemType is used to format partitions.
type FileSystemType = string

// Filesystem types.
const (
	FilesystemTypeNone FileSystemType = "none"
	FilesystemTypeXFS  FileSystemType = "xfs"
	FilesystemTypeVFAT FileSystemType = "vfat"
)

// Partition default sizes.
const (
	MiB = 1024 * 1024

	EFISize      = 100 * MiB
	BIOSGrubSize = 1 * MiB
	BootSize     = 300 * MiB
	MetaSize     = 1 * MiB
	StateSize    = 100 * MiB
)

// NewManifest initializes and returns a Manifest.
//
//nolint: gocyclo
func NewManifest(label string, sequence runtime.Sequence, bootPartitionFound bool, opts *Options) (manifest *Manifest, err error) {
	if label == "" {
		return nil, fmt.Errorf("a label is required, got \"\"")
	}

	manifest = &Manifest{
		Devices: map[string]Device{},
		Targets: map[string][]*Target{},
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

	efiTarget := &Target{
		Device:         opts.Disk,
		Label:          constants.EFIPartitionLabel,
		PartitionType:  EFISystemPartition,
		FileSystemType: FilesystemTypeVFAT,
		Size:           EFISize,
		Force:          true,
	}

	biosTarget := &Target{
		Device:             opts.Disk,
		Label:              constants.BIOSGrubPartitionLabel,
		PartitionType:      BIOSBootPartition,
		FileSystemType:     FilesystemTypeNone,
		LegacyBIOSBootable: true,
		Size:               BIOSGrubSize,
		Force:              true,
	}

	var bootTarget *Target

	if opts.Bootloader {
		bootTarget = &Target{
			Device:           opts.Disk,
			Label:            constants.BootPartitionLabel,
			PartitionType:    LinuxFilesystemData,
			FileSystemType:   FilesystemTypeXFS,
			Size:             BootSize,
			Force:            true,
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
		}
	}

	metaTarget := &Target{
		Device:           opts.Disk,
		Label:            constants.MetaPartitionLabel,
		PartitionType:    LinuxFilesystemData,
		FileSystemType:   FilesystemTypeNone,
		Size:             MetaSize,
		Force:            true,
		PreserveContents: bootPartitionFound,
	}

	stateTarget := &Target{
		Device:           opts.Disk,
		Label:            constants.StatePartitionLabel,
		PartitionType:    LinuxFilesystemData,
		FileSystemType:   FilesystemTypeXFS,
		Size:             StateSize,
		Force:            true,
		PreserveContents: bootPartitionFound,
		ExtraPreserveSources: []PreserveSource{
			{
				Label:          constants.LegacyBootPartitionLabel,
				FileSystemType: FilesystemTypeVFAT,
				FnmatchFilters: []string{"config.yaml"},
			},
		},
	}

	ephemeralTarget := &Target{
		Device:         opts.Disk,
		Label:          constants.EphemeralPartitionLabel,
		PartitionType:  LinuxFilesystemData,
		FileSystemType: FilesystemTypeXFS,
		Size:           0,
	}

	if opts.Force {
		ephemeralTarget.Force = true
	} else {
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

			defer f.Close() //nolint: errcheck

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

//nolint: gocyclo
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

	if bd, err = blockdevice.Open(device.Device, blockdevice.WithNewGPT(device.ResetPartitionTable)); err != nil {
		return err
	}

	// nolint: errcheck
	defer bd.Close()

	if device.ResetPartitionTable {
		log.Printf("resetting partition table on %s", device.Device)

		// TODO: how should it work with zero option above?
		if err = bd.Reset(); err != nil {
			return err
		}

		if err = bd.RereadPartitionTable(); err != nil {
			return err
		}
	} else {
		// clean up partitions which are going to be recreated
		var pt table.PartitionTable

		pt, err = bd.PartitionTable()
		if err != nil {
			return err
		}

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

		for _, part := range pt.Partitions() {
			delete(missingPartitions, part.Label())
		}

		if len(missingPartitions) > 0 {
			return fmt.Errorf("some partitions to be skipped are missing: %v", missingPartitions)
		}

		// delete all partitions which are not skipped
		for _, part := range pt.Partitions() {
			if _, ok := keepPartitions[part.Label()]; !ok {
				log.Printf("deleting partition %s", part.Label())

				if err = pt.Delete(part); err != nil {
					return err
				}
			}
		}

		if err = pt.Write(); err != nil {
			return err
		}

		if err = bd.RereadPartitionTable(); err != nil {
			return err
		}
	}

	var pt table.PartitionTable

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

	if err = bd.RereadPartitionTable(); err != nil {
		log.Printf("failed to re-read partition table on %q: %s, ignoring error...", device.Device, err)
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

//nolint: gocyclo
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

	// nolint: errcheck
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
			sourcePart     table.Partition
			fileSystemType FileSystemType
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
			for _, part := range pt.Partitions() {
				if part.Label() == source.Label {
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
func (m *Manifest) SystemMountpoints() (*mount.Points, error) {
	mountpoints := mount.NewMountPoints()

	for dev := range m.Targets {
		mp, err := mount.SystemMountPointsForDevice(dev)
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

	if bd, err = blockdevice.Open(device.Device); err != nil {
		return err
	}

	defer bd.Close() //nolint: errcheck

	var method string

	if method, err = bd.Wipe(); err != nil {
		return err
	}

	log.Printf("wiped %q with %q", device.Device, method)

	return bd.Close()
}

// Partition creates a new partition on the specified device.
// nolint: dupl, gocyclo
func (t *Target) Partition(pt table.PartitionTable, pos int, bd *blockdevice.BlockDevice) (err error) {
	if t.Skip {
		for _, part := range pt.Partitions() {
			if part.Label() == t.Label {
				t.PartitionName, err = util.PartPath(t.Device, int(part.No()))
				if err != nil {
					return err
				}

				log.Printf("skipped %s (%s) size %d blocks", t.PartitionName, t.Label, part.Length())

				break
			}
		}

		return nil
	}

	log.Printf("partitioning %s - %s\n", t.Device, t.Label)

	opts := []interface{}{
		partition.WithPartitionType(t.PartitionType),
		partition.WithPartitionName(t.Label),
	}

	if t.Size == 0 {
		opts = append(opts, partition.WithMaximumSize(true))
	}

	if t.LegacyBIOSBootable {
		opts = append(opts, partition.WithLegacyBIOSBootableAttribute(true))
	}

	part, err := pt.InsertAt(pos, uint64(t.Size), opts...)
	if err != nil {
		return err
	}

	t.PartitionName, err = util.PartPath(t.Device, int(part.No()))
	if err != nil {
		return err
	}

	log.Printf("created %s (%s) size %d blocks", t.PartitionName, t.Label, part.Length())

	return nil
}

// Format creates a filesystem on the device/partition.
//
//nolint: gocyclo
func (t *Target) Format() error {
	if t.Skip {
		return nil
	}

	if t.FileSystemType == FilesystemTypeNone {
		return nil
	}

	log.Printf("formatting partition %q as %q with label %q\n", t.PartitionName, t.FileSystemType, t.Label)

	opts := []makefs.Option{makefs.WithForce(t.Force), makefs.WithLabel(t.Label)}

	switch t.FileSystemType {
	case FilesystemTypeVFAT:
		return makefs.VFAT(t.PartitionName, opts...)
	case FilesystemTypeXFS:
		return makefs.XFS(t.PartitionName, opts...)
	default:
		return fmt.Errorf("unsupported filesystem type: %q", t.FileSystemType)
	}
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

			return nil
		}()

		if err != nil {
			return err
		}
	}

	return nil
}

func withTemporaryMounted(partPath string, flags uintptr, fileSystemType FileSystemType, label string, f func(mountPath string) error) error {
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
func (t *Target) SaveContents(device Device, source table.Partition, fileSystemType FileSystemType, fnmatchFilters []string) error {
	partPath, err := util.PartPath(device.Device, int(source.No()))
	if err != nil {
		return err
	}

	if fileSystemType == FilesystemTypeNone {
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

	defer src.Close() //nolint: errcheck

	t.Contents = bytes.NewBuffer(nil)

	zw := gzip.NewWriter(t.Contents)
	defer zw.Close() //nolint: errcheck

	_, err = io.Copy(zw, src)
	if err != nil {
		return fmt.Errorf("error copying partition %q contents: %w", partPath, err)
	}

	return src.Close()
}

func (t *Target) saveFilesystemContents(partPath string, fileSystemType FileSystemType, fnmatchFilters []string) error {
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

	if t.FileSystemType == FilesystemTypeNone {
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

	defer dst.Close() //nolint: errcheck

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
