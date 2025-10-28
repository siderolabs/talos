// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package vm

import (
	"context"

	"github.com/siderolabs/talos/pkg/provision"
)

// CreateDHCPd creates a DHCP server.
func (p *Provisioner) CreateDHCPd(ctx context.Context, state *State, clusterReq provision.ClusterRequest) error {
	return p.startDHCPd(state, clusterReq)
}
