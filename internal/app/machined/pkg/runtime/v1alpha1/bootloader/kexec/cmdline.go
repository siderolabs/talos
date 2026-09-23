// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kexec

import (
	"github.com/siderolabs/go-procfs/procfs"

	"github.com/siderolabs/talos/pkg/machinery/constants"
)

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
