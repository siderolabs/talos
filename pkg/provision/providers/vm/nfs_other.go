// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build !linux || (!amd64 && !arm64 && !riscv64)

package vm

import (
	"context"

	"github.com/siderolabs/talos/pkg/provision"
)

func NFSd(ctx context.Context, bindAddress string, port int) error {
	return nil
}

func (p *Provisioner) startNFSd(_ *provision.State, _ provision.ClusterRequest) error {
	return nil
}

// DestroyNFS stops the development NFS server.
func (p *Provisioner) stopNFSd(_ *provision.State) error {
	return nil
}
