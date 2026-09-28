// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kexec

import (
	"github.com/siderolabs/go-procfs/procfs"

	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// BootImageParam is the kernel argument GRUB's `linux` command prepends to the command line,
// holding the path to the kernel being booted.
const BootImageParam = "BOOT_IMAGE"

// AppendBootImage sets the BOOT_IMAGE kernel argument (replacing any existing value) the way GRUB does.
//
// It is used on kexec, when GRUB is skipped and can't report the kernel being booted itself.
func AppendBootImage(cmdline, kernelPath string) string {
	parsed := procfs.NewCmdline(cmdline)
	parsed.Set(BootImageParam, procfs.NewParameter(BootImageParam).Append(kernelPath))

	return parsed.String()
}

// AppendBootPartitionUUID sets the boot partition kernel argument (replacing any existing value), unless the UUID is empty.
//
// It is used on kexec, when the bootloader is skipped and can't report the partition itself.
func AppendBootPartitionUUID(cmdline, partitionUUID string) string {
	if partitionUUID == "" {
		return cmdline
	}

	parsed := procfs.NewCmdline(cmdline)
	parsed.Set(constants.KernelParamBootPartitionUUID, procfs.NewParameter(constants.KernelParamBootPartitionUUID).Append(partitionUUID))

	return parsed.String()
}
