// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package odroidhc4 provides the ODroid HC4 board implementation.
package odroidhc4

import (
	"log"
	"os"
	"path/filepath"

	"github.com/siderolabs/go-procfs/procfs"
	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/pkg/copy"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

var dtb = "/amlogic/meson-sm1-odroid-hc4.dtb"

// ODroidHC4 represents the ODroid HC4.
//
// References:
//   - https://wiki.odroid.com/odroid-hc4/odroid-hc4
//   - https://wiki.odroid.com/odroid-hc4/software/partition_table
//   - https://github.com/u-boot/u-boot/blob/master/doc/board/amlogic/odroid-c4.rst
type ODroidHC4 struct{}

// Name implements the runtime.Board.
func (b *ODroidHC4) Name() string {
	return constants.BoardODroidHC4
}

// Install implements the runtime.Board.
func (b *ODroidHC4) Install(disk string) (err error) {
	var f *os.File

	if f, err = os.OpenFile(disk, os.O_RDWR|unix.O_CLOEXEC, 0o666); err != nil {
		return err
	}
	//nolint:errcheck
	defer f.Close()

	// NB: In the case that the block device is a loopback device, we sync here
	// to esure that the file is written before the loopback device is
	// unmounted.
	err = f.Sync()
	if err != nil {
		return err
	}

	src := "/usr/install/arm64/dtb" + dtb
	dst := "/boot/EFI" + dtb

	log.Printf("write %s to %s", src, dst)

	err = os.MkdirAll(filepath.Dir(dst), 0o600)
	if err != nil {
		return err
	}

	err = copy.File(src, dst)
	if err != nil {
		return err
	}

	log.Printf("wrote %s to %s", src, dst)

	return nil
}

// KernelArgs implements the runtime.Board.
func (b *ODroidHC4) KernelArgs() procfs.Parameters {
	return []*procfs.Parameter{
		// https://wiki.odroid.com/odroid-hc4/application_note/misc/dmesg_on_display
		procfs.NewParameter("console").Append("tty1").Append("ttyAML0,115200n8"),
	}
}

// PartitionOptions implements the runtime.Board.
func (b *ODroidHC4) PartitionOptions() *runtime.PartitionOptions {
	return &runtime.PartitionOptions{PartitionsOffset: 2048}
}
