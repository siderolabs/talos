// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package rock64

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
	bin       = fmt.Sprintf("/usr/install/arm64/u-boot/%s/u-boot-rockchip.bin", constants.BoardRockpi4)
	off int64 = 512 * 64
	dtb       = "/dtb/rockchip/rk3399-rock-pi-4c.dtb"
)

// Rockpi4 represents the Radxa rock pi board.
//
// Reference: https://rockpi.org/
type Rockpi4 struct{}

// Name implements the runtime.Board.
func (r *Rockpi4) Name() string {
	return constants.BoardRockpi4
}

// Install implements the runtime.Board.
func (r *Rockpi4) Install(disk string) (err error) {
	var f *os.File

	if f, err = os.OpenFile(disk, os.O_RDWR|unix.O_CLOEXEC, 0o666); err != nil {
		return err
	}

	defer f.Close() //nolint:errcheck

	uboot, err := ioutil.ReadFile(bin)
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

	src := "/usr/install/arm64" + dtb
	dst := "/boot/EFI" + dtb

	err = os.MkdirAll(filepath.Dir(dst), 0o600)
	if err != nil {
		return err
	}

	return copy.File(src, dst)
}

// KernelArgs implements the runtime.Board.
func (r *Rockpi4) KernelArgs() procfs.Parameters {
	return []*procfs.Parameter{
		procfs.NewParameter("console").Append("tty0").Append("ttyS2,1500000n8"),
	}
}

// PartitionOptions implements the runtime.Board.
func (r *Rockpi4) PartitionOptions() *runtime.PartitionOptions {
	return &runtime.PartitionOptions{PartitionsOffset: 2048 * 10}
}
