// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package espmirror keeps the ESP copies on the members of an MD RAID1 boot disk identical.
//
// When the boot disk is an MD array (RAID1 with the metadata at the end, so that the firmware sees a plain
// ESP on every member), anything written to the ESP before Linux assembles the array lands on a single
// member, behind the back of MD: the copies diverge, and the array can't tell. Reads through the array
// are balanced across the members, so a FAT whose copies differ might be read as a mix of both.
//
// Talos itself only writes to the ESP through the array. The writes before Linux come from systemd-boot
// (the random seed) and systemd-stub (the boot secret mixin): both treat a file which carries the FAT
// read-only attribute as one they must not update, and both create the file when it is missing, so the
// installer provides both files, marked read-only.
//
// Installs which predate that already have diverged copies, so the upgrade repairs the ESP range of the
// array before mounting it.
package espmirror

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/siderolabs/go-blockdevice/v2/blkid"
	"github.com/siderolabs/go-pointer"
	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/options"
	"github.com/siderolabs/talos/internal/pkg/md"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

const (
	// pinnedFileSize is the size of the files placed on the ESP, which matches what systemd-boot and
	// systemd-stub would write.
	pinnedFileSize = 32

	// espRepairTimeout bounds the ESP repair on upgrade.
	espRepairTimeout = 10 * time.Minute
)

// FAT attribute ioctls, see include/uapi/linux/msdos_fs.h.
//
// The encoding is the generic one (asm-generic/ioctl.h), which is used by every architecture Talos supports.
const (
	fatIoctlGetAttributes = 0x80047210 // _IOR('r', 0x10, __u32)
	fatIoctlSetAttributes = 0x40047211 // _IOW('r', 0x11, __u32)

	fatAttrReadOnly = 0x01
)

type pinnedFile struct {
	path    string
	content func() []byte
}

var pinnedFiles = []pinnedFile{
	{
		// systemd-boot skips a read-only seed altogether, so the contents are never consumed
		path:    "loader/random-seed",
		content: func() []byte { return make([]byte, pinnedFileSize) },
	},
	{
		// systemd-stub still reads a read-only mixin to derive the boot secret, so it should be random
		path: "loader/boot-secret-mixin",
		content: func() []byte {
			mixin := make([]byte, pinnedFileSize)
			rand.Read(mixin) //nolint:errcheck // never fails

			return mixin
		},
	},
}

// IsMDBootDisk returns true if the boot disk is an MD array.
func IsMDBootDisk(opts options.InstallOptions) (bool, error) {
	if opts.ImageMode {
		return false, nil
	}

	mdManager, err := md.New()
	if err != nil {
		return false, err
	}

	isArray, err := mdManager.IsArray(opts.BootDisk)
	if err != nil {
		return false, fmt.Errorf("failed to check whether %s is an MD array: %w", opts.BootDisk, err)
	}

	return isArray, nil
}

// PinFiles places the files systemd-boot and systemd-stub would otherwise create or update on the ESP,
// marked read-only.
//
// Existing files are kept if they are long enough to be used: their contents don't matter for the random
// seed, and keep the boot secret stable for the mixin.
func PinFiles(espDir string, printf func(format string, v ...any)) error {
	for _, file := range pinnedFiles {
		path := filepath.Join(espDir, file.path)

		st, err := os.Stat(path)

		switch {
		case err == nil && st.Size() >= pinnedFileSize:
		case err == nil || errors.Is(err, fs.ErrNotExist):
			// a short file is left alone by systemd, but it's not usable either: replace it
			if err = os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
				return err
			}

			if err = os.WriteFile(path, file.content(), 0o644); err != nil {
				return err
			}
		default:
			return err
		}

		if err = setFATReadOnly(path); err != nil {
			return fmt.Errorf("failed to mark %s read-only: %w", file.path, err)
		}

		printf("sd-boot: %s is read-only to protect the ESP mirror", file.path)
	}

	return nil
}

// setFATReadOnly sets the FAT read-only attribute on the file.
//
// The ioctl is used instead of chmod, as vfat only maps the mode to the attribute when the fmask of the
// mount leaves the owner write bit.
func setFATReadOnly(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}

	defer f.Close() //nolint:errcheck

	attrs, err := unix.IoctlGetUint32(int(f.Fd()), fatIoctlGetAttributes)
	if err != nil {
		return fmt.Errorf("failed to get FAT attributes: %w", err)
	}

	if attrs&fatAttrReadOnly != 0 {
		return nil
	}

	if err = unix.IoctlSetPointerInt(int(f.Fd()), fatIoctlSetAttributes, int(attrs|fatAttrReadOnly)); err != nil {
		return fmt.Errorf("failed to set FAT attributes: %w", err)
	}

	attrs, err = unix.IoctlGetUint32(int(f.Fd()), fatIoctlGetAttributes)
	if err != nil {
		return fmt.Errorf("failed to get FAT attributes: %w", err)
	}

	if attrs&fatAttrReadOnly == 0 {
		return errors.New("read-only attribute was not set")
	}

	return nil
}

// Repair makes the copies of the ESP identical on all members of the MD array, by running an MD
// repair over the ESP range of the array.
//
// The copies only diverge in the files written before Linux (random seed, boot secret mixin), so it doesn't
// matter which member the repair picks the data from, as long as all of them end up with the same one.
func Repair(opts options.InstallOptions, info *blkid.Info) error {
	mdManager, err := md.New()
	if err != nil {
		return err
	}

	members, err := mdManager.InSyncMembers(opts.BootDisk)
	if err != nil {
		return fmt.Errorf("failed to list members of %s: %w", opts.BootDisk, err)
	}

	if len(members) < 2 {
		opts.Printf("sd-boot: skipping ESP repair, %s has %d in-sync member(s)", opts.BootDisk, len(members))

		return nil
	}

	var esp *blkid.NestedProbeResult

	for i := range info.Parts {
		if pointer.SafeDeref(info.Parts[i].PartitionLabel) == constants.EFIPartitionLabel {
			esp = &info.Parts[i]

			break
		}
	}

	if esp == nil {
		return fmt.Errorf("partition with label %s not found on %s", constants.EFIPartitionLabel, opts.BootDisk)
	}

	opts.Printf("sd-boot: repairing ESP copies on %s members %v", opts.BootDisk, members)

	ctx, cancel := context.WithTimeout(context.Background(), espRepairTimeout)
	defer cancel()

	mismatches, err := mdManager.RepairRange(ctx, opts.BootDisk, esp.PartitionOffset, esp.PartitionSize)
	if err != nil {
		return err
	}

	if mismatches > 0 {
		opts.Printf("sd-boot: ESP copies on %s differed in %d sector(s), repaired", opts.BootDisk, mismatches)
	} else {
		opts.Printf("sd-boot: ESP copies on %s are identical", opts.BootDisk)
	}

	return nil
}
