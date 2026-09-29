// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package vm

import (
	"github.com/siderolabs/talos/pkg/provision"
)

// CreateNFS starts the development NFS server.
func (p *Provisioner) CreateNFS(state *provision.State, clusterReq provision.ClusterRequest) error {
	return p.startNFSd(state, clusterReq)
}

// DestroyNFS stops the development NFS server.
func (p *Provisioner) DestroyNFS(state *provision.State) error {
	return p.stopNFSd(state)
}
