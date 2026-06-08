// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package qemu

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/siderolabs/talos/pkg/provision"
)

// RebootNode forcefully reboots a single cluster node via the QEMU monitor.
//
// Sending "q" (quit) to the node's monitor socket makes the QEMU process exit; the per-node
// launcher (see Launch) then restarts the VM while it is still powered on, resulting in a cold
// reboot. This mirrors the manual `echo q | socat - unix-connect:<node>.monitor` workflow.
func (p *provisioner) RebootNode(_ context.Context, cluster provision.Cluster, node provision.NodeInfo) error {
	statePath, err := cluster.StatePath()
	if err != nil {
		return err
	}

	monitorPath := filepath.Join(statePath, fmt.Sprintf("%s.monitor", node.Name))

	if err := sendMonitorCommand(monitorPath, "q"); err != nil {
		return fmt.Errorf("failed to reboot node %q: %w", node.Name, err)
	}

	return nil
}
