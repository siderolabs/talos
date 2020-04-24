// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package blkpg

import (
	"fmt"
	"os"

	"github.com/talos-systems/talos/pkg/blockdevice/table"
)

// InformKernelOfAdd invokes the BLKPG_ADD_PARTITION ioctl.
func InformKernelOfAdd(f *os.File, partition table.Partition) error {
	return fmt.Errorf("not implemented")
}

// InformKernelOfResize invokes the BLKPG_RESIZE_PARTITION ioctl.
func InformKernelOfResize(f *os.File, partition table.Partition) error {
	return fmt.Errorf("not implemented")
}

// InformKernelOfDelete invokes the BLKPG_DEL_PARTITION ioctl.
func InformKernelOfDelete(f *os.File, partition table.Partition) error {
	return fmt.Errorf("not implemented")
}
