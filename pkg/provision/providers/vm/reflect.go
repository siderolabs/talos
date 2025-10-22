// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package vm

import (
	"context"
	"fmt"

	"github.com/siderolabs/talos/pkg/provision"
)

// Reflect decode state file.
func (p *Provisioner) Reflect(ctx context.Context, clusterName, stateDirectory string) (provision.Cluster, error) {
	state, err := provision.ReadState(ctx, clusterName, stateDirectory)
	if err != nil {
		return nil, err
	}

	if state.ProvisionerName != p.Name {
		return nil, fmt.Errorf("cluster %q was created with different provisioner %q", clusterName, state.ProvisionerName)
	}

	return state, nil
}
