// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package nanopir4s

import (
	"fmt"
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
	bin       = fmt.Sprintf("/usr/install/arm64/u-boot/%s/u-boot-rockchip.bin", constants.BoardNanoPiR4S)
	off int64 = 512 * 64
	dtb       = "/dtb/rockchip/rk3399-nanopi-r4s.dtb"
)

// NanoPiR4S represents the Friendlyelec Nano Pi R4S board.
//
// Reference: https://wiki.friendlyelec.com/wiki/index.php/NanoPi_R4S
type NanoPiR4S struct{}

// Name implements the runtime.Board.
func (n *NanoPiR4S) Name() string {
	return constants.BoardNanoPiR4S
}

// Install implements the runtime.Board.
func (n *NanoPiR4S) Install(disk string) (err error) {
	file, err := os.OpenFile(disk, os.O_RDWR|unix.O_CLOEXEC, 0o666)
	if err != nil {
		return err
	}

	defer file.Close() //nolint:errcheck

	uboot, err := os.ReadFile(bin)
	if err != nil {
		return err
	}

	log.Printf("writing %s at offset %d", bin, off)

	amount, err := file.WriteAt(uboot, off)
	if err != nil {
		return err
	}

	log.Printf("wrote %d bytes", amount)

	// NB: In the case that the block device is a loopback device, we sync here
	// to esure that the file is written before the loopback device is
	// unmounted.
	if err := file.Sync(); err != nil {
		return err
	}

	src := "/usr/install/arm64" + dtb
	dst := "/boot/EFI" + dtb

	if err := os.MkdirAll(filepath.Dir(dst), 0o600); err != nil {
		return err
	}

	return copy.File(src, dst)
}

// KernelArgs implements the runtime.Board.
func (n *NanoPiR4S) KernelArgs() procfs.Parameters {
	return []*procfs.Parameter{
		procfs.NewParameter("console").Append("tty0").Append("ttyS2,1500000n8"),
		procfs.NewParameter("sysctl.kernel.kexec_load_disabled").Append("1"),
	}
}

// PartitionOptions implements the runtime.Board.
func (n *NanoPiR4S) PartitionOptions() *runtime.PartitionOptions {
	return &runtime.PartitionOptions{PartitionsOffset: 2048 * 10}
}
