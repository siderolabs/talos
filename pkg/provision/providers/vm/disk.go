// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package vm

import (
	"fmt"
	"os"

	"github.com/talos-systems/talos/pkg/provision"
)

// CreateDisk creates an empty disk file.
func (p *Provisioner) CreateDisk(state *State, nodeReq provision.NodeRequest) (diskPath string, err error) {
	diskPath = state.GetRelativePath(fmt.Sprintf("%s.disk", nodeReq.Name))

	var diskF *os.File

	diskF, err = os.Create(diskPath)
	if err != nil {
		return
	}

	defer diskF.Close() //nolint: errcheck

	err = diskF.Truncate(nodeReq.DiskSize)

	return
}
