// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package rock4cplus provides the Radxa ROCK 4C+ implementation.
package rock4cplus

import (
	"os"
	"path/filepath"

	"github.com/siderolabs/go-procfs/procfs"
	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/pkg/copy"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

var (
	bin       = constants.BoardRock4cPlus + "/u-boot-rockchip.bin"
	off int64 = 512 * 64
	// https://github.com/u-boot/u-boot/blob/abd4fb5ac13215733569925a06991e0a182ede14/configs/rock-4c-plus-rk3399_defconfig#L22
	dtb = "rockchip/rk3399-rock-4c-plus.dtb"
)

// Rock4cplus represents the Radxa ROCK 4C+ board.
//
// Reference: https://rockpi.org/
type Rock4cplus struct{}

// Name implements the runtime.Board.
func (r *Rock4cplus) Name() string {
	return constants.BoardRock4cPlus
}

// Install implements the runtime.Board.
func (r *Rock4cplus) Install(options runtime.BoardInstallOptions) (err error) {
	var f *os.File

	if f, err = os.OpenFile(options.InstallDisk, os.O_RDWR|unix.O_CLOEXEC, 0o666); err != nil {
		return err
	}

	defer f.Close() //nolint:errcheck

	uboot, err := os.ReadFile(filepath.Join(options.UBootPath, bin))
	if err != nil {
		return err
	}

	options.Printf("writing %s at offset %d", bin, off)

	var n int

	n, err = f.WriteAt(uboot, off)
	if err != nil {
		return err
	}

	options.Printf("wrote %d bytes", n)

	// NB: In the case that the block device is a loopback device, we sync here
	// to esure that the file is written before the loopback device is
	// unmounted.
	err = f.Sync()
	if err != nil {
		return err
	}

	src := filepath.Join(options.DTBPath, dtb)
	dst := filepath.Join(options.MountPrefix, "/boot/EFI/dtb", dtb)

	err = os.MkdirAll(filepath.Dir(dst), 0o600)
	if err != nil {
		return err
	}

	return copy.File(src, dst)
}

// KernelArgs implements the runtime.Board.
func (r *Rock4cplus) KernelArgs() procfs.Parameters {
	return []*procfs.Parameter{
		procfs.NewParameter("console").Append("tty0").Append("ttyS2,1500000n8"),
		procfs.NewParameter("sysctl.kernel.kexec_load_disabled").Append("1"),
		procfs.NewParameter(constants.KernelParamDashboardDisabled).Append("1"),
	}
}

// PartitionOptions implements the runtime.Board.
func (r *Rock4cplus) PartitionOptions() *runtime.PartitionOptions {
	return &runtime.PartitionOptions{PartitionsOffset: 2048 * 10}
}
