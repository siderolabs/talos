// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package pine64

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"github.com/talos-systems/go-procfs/procfs"
	"golang.org/x/sys/unix"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/pkg/copy"
	"github.com/talos-systems/talos/pkg/machinery/constants"
)

var (
	bin       = fmt.Sprintf("/usr/install/u-boot/%s/u-boot-sunxi-with-spl.bin", constants.BoardPine64)
	off int64 = 1024 * 8
	dtb       = "/dtb/allwinner/sun50i-a64-pine64-plus.dtb"
)

// Pine64 represents the Pine64 board
//
// References:
//	- http://linux-sunxi.org/Pine64
type Pine64 struct{}

// Name implements the runtime.Board.
func (b *Pine64) Name() string {
	return constants.BoardBananaPiM64
}

// Install implements the runtime.Board.
func (b Pine64) Install(disk string) (err error) {
	var f *os.File

	if f, err = os.OpenFile(disk, os.O_RDWR|unix.O_CLOEXEC, 0o666); err != nil {
		return err
	}
	//nolint:errcheck
	defer f.Close()

	var uboot []byte

	uboot, err = ioutil.ReadFile(bin)
	if err != nil {
		return err
	}

	log.Printf("writing %s at offset %d", bin, off)

	var n int

	n, err = f.WriteAt(uboot, off)
	if err != nil {
		return err
	}

	log.Printf("wrote %d bytes", n)

	// NB: In the case that the block device is a loopback device, we sync here
	// to esure that the file is written before the loopback device is
	// unmounted.
	err = f.Sync()
	if err != nil {
		return err
	}

	src := "/usr/install" + dtb
	dst := "/boot/EFI" + dtb

	err = os.MkdirAll(filepath.Dir(dst), 0o600)
	if err != nil {
		return err
	}

	err = copy.File(src, dst)
	if err != nil {
		return err
	}

	return nil
}

// KernelArgs implements the runtime.Board.
func (b Pine64) KernelArgs() procfs.Parameters {
	return []*procfs.Parameter{
		procfs.NewParameter("console").Append("tty0").Append("ttyS0,115200"),
	}
}

// PartitionOptions implements the runtime.Board.
func (b Pine64) PartitionOptions() *runtime.PartitionOptions {
	return &runtime.PartitionOptions{PartitionsOffset: 2048}
}
