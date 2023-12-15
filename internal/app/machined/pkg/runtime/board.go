// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import "github.com/siderolabs/go-procfs/procfs"

// PartitionOptions are the board specific options for customizing the
// partition table.
type PartitionOptions struct {
	PartitionsOffset uint64
}

// BoardInstallOptions are the board specific options for installation of various boot assets.
type BoardInstallOptions struct {
	InstallDisk     string
	MountPrefix     string
	DTBPath         string
	UBootPath       string
	RPiFirmwarePath string
	Printf          func(string, ...any)
}

// Board defines the requirements for a SBC.
type Board interface {
	Name() string
	Install(options BoardInstallOptions) error
	KernelArgs() procfs.Parameters
	PartitionOptions() *PartitionOptions
}
