// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package partition provides common utils for system partition format.
package partition

import (
	"fmt"

	"github.com/siderolabs/go-blockdevice/v2/block"

	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/imager/quirks"
	"github.com/siderolabs/talos/pkg/makefs"
)

// FormatOptions contains format parameters.
type FormatOptions struct {
	Label               string
	SourceDirectory     string
	FileSystemType      FileSystemType
	Force               bool
	UnsupportedFSOption bool
	Reproducible        bool
}

// FormatOption to control options.
type FormatOption func(*FormatOptions)

// WithSourceDirectory sets the source directory for populating the filesystem.
func WithSourceDirectory(dir string) FormatOption {
	return func(o *FormatOptions) {
		o.SourceDirectory = dir
	}
}

// WithUnsupportedFSOption sets the unsupported filesystem option.
func WithUnsupportedFSOption() FormatOption {
	return func(o *FormatOptions) {
		o.UnsupportedFSOption = true
	}
}

// WithForce sets the force option.
func WithForce() FormatOption {
	return func(o *FormatOptions) {
		o.Force = true
	}
}

// WithLabel sets the label for the filesystem.
func WithLabel(label string) FormatOption {
	return func(o *FormatOptions) {
		o.Label = label
	}
}

// WithFileSystemType sets the filesystem type.
func WithFileSystemType(fsType FileSystemType) FormatOption {
	return func(o *FormatOptions) {
		o.FileSystemType = fsType
	}
}

// WithReproducible sets the reproducible option.
func WithReproducible() FormatOption {
	return func(o *FormatOptions) {
		o.Reproducible = true
	}
}

// NewFormatOptions creates a new format options.
func NewFormatOptions(opts ...FormatOption) *FormatOptions {
	o := &FormatOptions{}

	for _, opt := range opts {
		opt(o)
	}

	return systemPartitionsFormatOptions(*o)
}

// Format zeroes the device and formats it using filesystem type provided.
func Format(devname string, t *FormatOptions, talosVersion string, printf func(string, ...any)) error {
	opts := []makefs.Option{
		makefs.WithForce(t.Force),
		makefs.WithLabel(t.Label),
		makefs.WithPrintf(printf),
	}

	if t.UnsupportedFSOption {
		opts = append(opts, makefs.WithUnsupportedFSOption(t.UnsupportedFSOption))
	}

	if t.SourceDirectory != "" {
		opts = append(opts, makefs.WithSourceDirectory(t.SourceDirectory))
	}

	if t.Reproducible {
		opts = append(opts, makefs.WithReproducible(true))
	}

	printf("formatting the partition %q as %q with label %q\n", devname, t.FileSystemType, t.Label)

	switch t.FileSystemType {
	case FilesystemTypeNone:
		return nil
	case FilesystemTypeZeroes:
		return zeroPartition(devname)
	case FilesystemTypeVFAT:
		return makefs.VFAT(devname, opts...)
	case FilesystemTypeXFS:
		opts = append(opts, makefs.WithConfigFile(quirks.New(talosVersion).XFSMkfsConfig()))

		return makefs.XFS(devname, opts...)
	case FileSystemTypeExt4:
		return makefs.Ext4(devname, opts...)
	default:
		return fmt.Errorf("unsupported filesystem type: %q", t.FileSystemType)
	}
}

// zeroPartition fills the partition with zeroes.
func zeroPartition(devname string) (err error) {
	part, err := block.NewFromPath(devname, block.OpenForWrite())
	if err != nil {
		return err
	}

	defer part.Close() //nolint:errcheck

	return part.FastWipe()
}

// systemPartitionsFormatOptions returns format options for system partitions.
func systemPartitionsFormatOptions(opts FormatOptions) *FormatOptions {
	switch opts.Label {
	case constants.EFIPartitionLabel:
		return &FormatOptions{
			Label:           constants.EFIPartitionLabel,
			SourceDirectory: opts.SourceDirectory,
			Reproducible:    opts.Reproducible,
			FileSystemType:  FilesystemTypeVFAT,
			Force:           true,
		}
	case constants.BIOSGrubPartitionLabel:
		return &FormatOptions{
			Label:          constants.BIOSGrubPartitionLabel,
			FileSystemType: FilesystemTypeZeroes,
			Force:          true,
		}
	case constants.BootPartitionLabel:
		return &FormatOptions{
			Label:           constants.BootPartitionLabel,
			SourceDirectory: opts.SourceDirectory,
			Reproducible:    opts.Reproducible,
			FileSystemType:  FilesystemTypeXFS,
			Force:           true,
		}
	case constants.MetaPartitionLabel:
		return &FormatOptions{
			Label:          constants.MetaPartitionLabel,
			FileSystemType: FilesystemTypeZeroes,
			Force:          true,
		}
	case constants.StatePartitionLabel:
		return &FormatOptions{
			Label:          constants.StatePartitionLabel,
			FileSystemType: FilesystemTypeNone,
		}
	case constants.EphemeralPartitionLabel:
		return &FormatOptions{
			Label:          constants.EphemeralPartitionLabel,
			FileSystemType: FilesystemTypeNone,
		}
	case constants.ImageCachePartitionLabel:
		return &FormatOptions{
			Label:           constants.ImageCachePartitionLabel,
			SourceDirectory: opts.SourceDirectory,
			FileSystemType:  FileSystemTypeExt4,
			Reproducible:    opts.Reproducible,
		}
	default:
		return nil
	}
}
