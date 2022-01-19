// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package jetsonnano

import (
	"os"
	"path/filepath"

	"github.com/talos-systems/go-procfs/procfs"
	"golang.org/x/sys/unix"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/pkg/copy"
	"github.com/talos-systems/talos/pkg/machinery/constants"
)

// References
// - https://github.com/u-boot/u-boot/blob/v2021.10/configs/p3450-0000_defconfig#L8
// - https://github.com/u-boot/u-boot/blob/v2021.10/include/configs/tegra-common.h#L53
// - https://github.com/u-boot/u-boot/blob/v2021.10/include/configs/tegra210-common.h#L49
var dtb = "/dtb/nvidia/tegra210-p3450-0000.dtb"

// JetsonNano represents the JetsonNano board
//
// References:
// - https://developer.nvidia.com/embedded/jetson-nano-developer-kit
type JetsonNano struct{}

// Name implements the runtime.Board.
func (b *JetsonNano) Name() string {
	return constants.BoardJetsonNano
}

// Install implements the runtime.Board.
func (b JetsonNano) Install(disk string) (err error) {
	var f *os.File

	if f, err = os.OpenFile(disk, os.O_RDWR|unix.O_CLOEXEC, 0o666); err != nil {
		return err
	}
	//nolint:errcheck
	defer f.Close()

	// NB: In the case that the block device is a loopback device, we sync here
	// to ensure that the file is written before the loopback device is
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
//
// References:
//  - https://elinux.org/Jetson/Nano/Upstream to enable early console
//  - http://en.techinfodepot.shoutwiki.com/wiki/NVIDIA_Jetson_Nano_Developer_Kit for other chips on the SoC
func (b JetsonNano) KernelArgs() procfs.Parameters {
	return []*procfs.Parameter{
		procfs.NewParameter("console").Append("tty0").Append("ttyS0,115200"),
		// even though PSCI works perfectly on the Jetson Nano, the kernel is stuck
		// trying to kexec. Seems the drivers state is not reset properly.
		// disabling kexec until we have further knowledge on this
		procfs.NewParameter("sysctl.kernel.kexec_load_disabled").Append("1"),
	}
}

// PartitionOptions implements the runtime.Board.
func (b JetsonNano) PartitionOptions() *runtime.PartitionOptions {
	return nil
}
