// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package qemu

import (
	"context"
	"net"

	"github.com/siderolabs/talos/pkg/provision"
)

func (p *provisioner) findAPIBindAddrs(ctx context.Context, clusterReq provision.ClusterRequest) (*net.TCPAddr, error) {
	return p.apiPorts.allocate(ctx, clusterReq.Network.GatewayAddrs[0].String())
}
