// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package rock5 provides the Radxa Rock 5 implementation.
package rock5

import (
	"os"
	"path/filepath"

	"github.com/siderolabs/go-copy/copy"
	"github.com/siderolabs/go-procfs/procfs"
	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

const (
	ubootBin    = "u-boot-rockchip.bin"
	ubootOffset = 512 * 64 // U-Boot offset for RK3588 is at sector 64 (32KB)
)

// Rock5B represents the Radxa Rock 5 Model B board.
//
// Reference: https://radxa.com/products/rock5/5b
type Rock5B struct{}

// Rock5T represents the Radxa Rock 5 Model T board.
//
// Reference: https://radxa.com/products/rock5/5t
type Rock5T struct{}

// Name implements the runtime.Board interface.
func (r *Rock5B) Name() string {
	return constants.BoardRock5B
}

// Name implements the runtime.Board interface.
func (r *Rock5T) Name() string {
	return constants.BoardRock5T
}

// Install implements the runtime.Board interface for Rock 5B.
func (r *Rock5B) Install(options runtime.BoardInstallOptions) error {
	return installRock5(options, "rockchip/rk3588-rock-5b.dtb")
}

// Install implements the runtime.Board interface for Rock 5T.
func (r *Rock5T) Install(options runtime.BoardInstallOptions) error {
	return installRock5(options, "rockchip/rk3588-rock-5t.dtb")
}

// installRock5 is the common installation logic for Rock 5 boards.
func installRock5(options runtime.BoardInstallOptions, dtbFile string) error {
	// Install U-Boot to disk offset
	f, err := os.OpenFile(options.InstallDisk, os.O_RDWR|unix.O_CLOEXEC, 0o666)
	if err != nil {
		return err
	}

	defer f.Close() //nolint:errcheck

	// Read U-Boot binary
	ubootPath := filepath.Join(options.UBootPath, ubootBin)
	uboot, err := os.ReadFile(ubootPath)
	if err != nil {
		return err
	}

	options.Printf("writing %s at offset %d", ubootBin, ubootOffset)

	// Write U-Boot to disk at the specified offset
	n, err := f.WriteAt(uboot, ubootOffset)
	if err != nil {
		return err
	}

	options.Printf("wrote %d bytes", n)

	// Sync to ensure write completes (important for loopback devices)
	if err := f.Sync(); err != nil {
		return err
	}

	// Copy DTB file to boot partition
	src := filepath.Join(options.DTBPath, dtbFile)
	dst := filepath.Join(options.MountPrefix, "/boot/EFI/dtb", dtbFile)

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}

	options.Printf("copying DTB %s to %s", dtbFile, dst)

	return copy.File(src, dst)
}

// KernelArgs implements the runtime.Board interface.
func (r *Rock5B) KernelArgs() procfs.Parameters {
	return rock5KernelArgs()
}

// KernelArgs implements the runtime.Board interface.
func (r *Rock5T) KernelArgs() procfs.Parameters {
	return rock5KernelArgs()
}

// rock5KernelArgs returns common kernel arguments for Rock 5 boards.
func rock5KernelArgs() procfs.Parameters {
	return []*procfs.Parameter{
		procfs.NewParameter("console").Append("tty0").Append("ttyS2,1500000n8"),
		procfs.NewParameter("sysctl.kernel.kexec_load_disabled").Append("1"),
		procfs.NewParameter(constants.KernelParamDashboardDisabled).Append("1"),
	}
}

// PartitionOptions implements the runtime.Board interface.
func (r *Rock5B) PartitionOptions() *runtime.PartitionOptions {
	return rock5PartitionOptions()
}

// PartitionOptions implements the runtime.Board interface.
func (r *Rock5T) PartitionOptions() *runtime.PartitionOptions {
	return rock5PartitionOptions()
}

// rock5PartitionOptions returns common partition options for Rock 5 boards.
// Start partitions at 32MB offset to leave space for U-Boot and other firmware.
func rock5PartitionOptions() *runtime.PartitionOptions {
	return &runtime.PartitionOptions{
		PartitionsOffset: 2048 * 32, // 32MB offset (sector 2048 * 32)
	}
}
