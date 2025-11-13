// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package proxmox

import (
	"context"

	"github.com/siderolabs/talos/pkg/provision"
	"github.com/siderolabs/talos/pkg/provision/providers/vm"
)

// Reflect reconstructs cluster state from Proxmox VMs.
func (p *provisioner) Reflect(ctx context.Context, clusterName, stateDirectory string) (provision.Cluster, error) {
	// Use vm.Provisioner's Reflect method which reads state from disk
	base := vm.Provisioner{Name: p.Name}
	return base.Reflect(ctx, clusterName, stateDirectory)
}

