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
	bin       = fmt.Sprintf("/usr/install/arm64/u-boot/%s/u-boot-rockchip.bin", constants.BoardRock64)
	off int64 = 512 * 64
	dtb       = "/dtb/rockchip/rk3328-rock64.dtb"
)

// Rock64 represents the Pine64 Rock64 board.
//
// Reference: https://www.pine64.org/devices/single-board-computers/rock64/
type Rock64 struct{}

// Name implements the runtime.Board.
func (r *Rock64) Name() string {
	return constants.BoardRock64
}

// Install implements the runtime.Board.
func (r *Rock64) Install(disk string) (err error) {
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

	src := "/usr/install/arm64" + dtb
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
func (r *Rock64) KernelArgs() procfs.Parameters {
	return []*procfs.Parameter{
		procfs.NewParameter("console").Append("tty0").Append("ttyS2,115200n8"),
	}
}

// PartitionOptions implements the runtime.Board.
func (r *Rock64) PartitionOptions() *runtime.PartitionOptions {
	return &runtime.PartitionOptions{PartitionsOffset: 2048 * 10}
}
