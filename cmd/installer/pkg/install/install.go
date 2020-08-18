// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package install

import (
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"unsafe"

	"github.com/talos-systems/go-procfs/procfs"
	"golang.org/x/sys/unix"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/grub"
	"github.com/talos-systems/talos/internal/pkg/mount"
	"github.com/talos-systems/talos/pkg/blockdevice/probe"
	"github.com/talos-systems/talos/pkg/machinery/constants"
	"github.com/talos-systems/talos/pkg/version"
)

// Options represents the set of options available for an install.
type Options struct {
	ConfigSource    string
	Disk            string
	Platform        string
	ExtraKernelArgs []string
	Bootloader      bool
	Upgrade         bool
	Force           bool
	Zero            bool
	Save            bool
}

// Install installs Talos.
func Install(p runtime.Platform, seq runtime.Sequence, opts *Options) (err error) {
	cmdline := procfs.NewCmdline("")
	cmdline.Append(constants.KernelParamPlatform, p.Name())
	cmdline.Append(constants.KernelParamConfig, opts.ConfigSource)

	if err = cmdline.AppendAll(p.KernelArgs().Strings()); err != nil {
		return err
	}

	if err = cmdline.AppendAll(opts.ExtraKernelArgs); err != nil {
		return err
	}

	cmdline.AppendDefaults()

	i, err := NewInstaller(cmdline, seq, opts)
	if err != nil {
		return err
	}

	if err = i.Install(seq); err != nil {
		return err
	}

	log.Printf("installation of %s complete", version.Tag)

	return nil
}

// Installer represents the installer logic. It serves as the entrypoint to all
// installation methods.
type Installer struct {
	cmdline    *procfs.Cmdline
	options    *Options
	manifest   *Manifest
	bootloader bootloader.Bootloader

	Current string
	Next    string

	bootPartitionFound bool
}

// NewInstaller initializes and returns an Installer.
//
// nolint: gocyclo
func NewInstaller(cmdline *procfs.Cmdline, seq runtime.Sequence, opts *Options) (i *Installer, err error) {
	i = &Installer{
		cmdline: cmdline,
		options: opts,
		bootloader: &grub.Grub{
			BootDisk: opts.Disk,
		},
	}

	var dev *probe.ProbedBlockDevice

	if dev, err = probe.DevForFileSystemLabel(opts.Disk, constants.BootPartitionLabel); err != nil {
		i.bootPartitionFound = false
	} else {
		//nolint: errcheck
		defer dev.Close()
		i.bootPartitionFound = true
	}

	i.Current, i.Next, err = i.bootloader.Labels()
	if err != nil {
		return nil, err
	}

	label := i.Next

	i.manifest, err = NewManifest(label, seq, i.options)
	if err != nil {
		return nil, fmt.Errorf("failed to create installation manifest: %w", err)
	}

	return i, nil
}

// Install fetches the necessary data locations and copies or extracts
// to the target locations.
//
// nolint: gocyclo
func (i *Installer) Install(seq runtime.Sequence) (err error) {
	if i.options.Force {
		if i.bootPartitionFound {
			var dev *probe.ProbedBlockDevice

			if dev, err = probe.DevForFileSystemLabel(i.options.Disk, constants.BootPartitionLabel); err != nil {
				return err
			}

			// Reset the partition table.

			if err = dev.Reset(); err != nil {
				return err
			}

			if err = dev.RereadPartitionTable(); err != nil {
				return err
			}

			if err = dev.Close(); err != nil {
				return err
			}
		}

		// Zero the disk.

		if i.options.Zero {
			if err = zero(i.manifest); err != nil {
				return fmt.Errorf("failed to wipe device(s): %w", err)
			}
		}

		// Partition and format the block device(s).

		if err = i.manifest.ExecuteManifest(); err != nil {
			return err
		}
	} else if !i.bootPartitionFound {
		if i.options.Zero {
			if err = zero(i.manifest); err != nil {
				return fmt.Errorf("failed to wipe device(s): %w", err)
			}
		}

		if err = i.manifest.ExecuteManifest(); err != nil {
			return err
		}
	}

	if seq == runtime.SequenceUpgrade {
		var meta *bootloader.Meta

		if meta, err = bootloader.NewMeta(); err != nil {
			return err
		}

		//nolint: errcheck
		defer meta.Close()

		if ok := meta.SetTag(bootloader.AdvUpgrade, i.Current); !ok {
			return fmt.Errorf("failed to set upgrade tag: %q", i.Current)
		}

		if _, err = meta.Write(); err != nil {
			return err
		}
	}

	// Mount the partitions.

	mountpoints := mount.NewMountPoints()

	for dev := range i.manifest.Targets {
		var mp *mount.Points

		mp, err = mount.SystemMountPointsForDevice(dev)
		if err != nil {
			return err
		}

		iter := mp.Iter()
		for iter.Next() {
			mountpoints.Set(iter.Key(), iter.Value())
		}
	}

	if err = mount.Mount(mountpoints); err != nil {
		return err
	}

	defer func() {
		e := mount.Unmount(mountpoints)
		if e != nil {
			log.Printf("failed to unmount: %v", e)
		}
	}()

	// Install the assets.

	for _, targets := range i.manifest.Targets {
		for _, target := range targets {
			// Handle the download and extraction of assets.
			if err = target.Save(); err != nil {
				return err
			}
		}
	}

	// Install the bootloader.

	if !i.options.Bootloader {
		return nil
	}

	i.cmdline.Append("initrd", filepath.Join("/", i.Next, constants.InitramfsAsset))

	grubcfg := &grub.Cfg{
		Default: i.Next,
		Labels: []*grub.Label{
			{
				Root:   i.Next,
				Initrd: filepath.Join("/", i.Next, constants.InitramfsAsset),
				Kernel: filepath.Join("/", i.Next, constants.KernelAsset),
				Append: i.cmdline.String(),
			},
		},
	}

	if i.bootPartitionFound && i.Current != "" {
		grubcfg.Fallback = i.Current

		grubcfg.Labels = append(grubcfg.Labels, &grub.Label{
			Root:   i.Current,
			Initrd: filepath.Join("/", i.Current, constants.InitramfsAsset),
			Kernel: filepath.Join("/", i.Current, constants.KernelAsset),
			Append: procfs.ProcCmdline().String(),
		})
	}

	if err = i.bootloader.Install(i.Current, grubcfg, seq, i.bootPartitionFound); err != nil {
		return err
	}

	if i.options.Save {
		u, err := url.Parse(i.options.ConfigSource)
		if err != nil {
			return err
		}

		if u.Scheme != "file" {
			return fmt.Errorf("file:// scheme must be used with the save option, have %s", u.Scheme)
		}

		src, err := os.Open(u.Path)
		if err != nil {
			return err
		}

		// nolint: errcheck
		defer src.Close()

		dst, err := os.OpenFile(constants.ConfigPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o600)
		if err != nil {
			return err
		}

		// nolint: errcheck
		defer dst.Close()

		_, err = io.Copy(dst, src)
		if err != nil {
			return err
		}
	}

	return nil
}

func zero(manifest *Manifest) (err error) {
	var zero *os.File

	if zero, err = os.Open("/dev/zero"); err != nil {
		return err
	}

	defer zero.Close() //nolint: errcheck

	for dev := range manifest.Targets {
		if err = func(dev string) error {
			var f *os.File

			if f, err = os.OpenFile(dev, os.O_RDWR, os.ModeDevice); err != nil {
				return err
			}

			defer f.Close() //nolint: errcheck

			var size uint64

			if _, _, ret := unix.Syscall(unix.SYS_IOCTL, f.Fd(), unix.BLKGETSIZE64, uintptr(unsafe.Pointer(&size))); ret != 0 {
				return fmt.Errorf("failed to got block device size: %v", ret)
			}

			if _, err = io.CopyN(f, zero, int64(size)); err != nil {
				return err
			}

			return f.Close()
		}(dev); err != nil {
			return err
		}
	}

	return zero.Close()
}
