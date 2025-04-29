// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package vm

import (
	"context"

	"github.com/siderolabs/talos/pkg/provision"
)

// CreateNetwork TODO
func (p *Provisioner) CreateNetwork(ctx context.Context, state *State, network provision.NetworkRequest, options provision.Options) error {
	panic("not implemented")
}

// DestroyNetwork TODO
func (p *Provisioner) DestroyNetwork(state *State) error {
	panic("not implemented")
}
