// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package vm

import (
	"context"
	"net"

	"github.com/siderolabs/gen/xslices"

	"github.com/siderolabs/talos/pkg/provision"
)

// CreateNetwork on darwin assigns the bridge name to the to-be created interface name.
// The interface itself is later created by qemu, but the name needs to be known so that the dhcp server can be linked to the interface.
func (p *Provisioner) CreateNetwork(ctx context.Context, state *State, network provision.NetworkRequest, options provision.Options) error {
	ifaces, err := net.Interfaces()
	if err != nil {
		return err
	}

	ifNames := xslices.Map(ifaces, func(iface net.Interface) string { return iface.Name })

	bridgeNAme, err := GetVmnetInterfaceName(ifNames)
	if err != nil {
		return err
	}

	state.BridgeName = bridgeNAme

	return nil
}

// DestroyNetwork does nothing on darwin as the network is automatically cleaned up by qemu when the final machine of a cidr block is killed.
func (p *Provisioner) DestroyNetwork(state *State) error {
	return nil
}
