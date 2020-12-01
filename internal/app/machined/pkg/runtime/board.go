// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import "github.com/talos-systems/go-procfs/procfs"

// PartitionOptions are the board specific options for customizing the
// partition table.
type PartitionOptions struct {
	PartitionsOffset uint64
}

// Board defines the requirements for a SBC.
type Board interface {
	Name() string
	Install(string) error
	KernelArgs() procfs.Parameters
	PartitionOptions() *PartitionOptions
}
