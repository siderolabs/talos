// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package docker

import (
	"context"
	"fmt"
	"os"

	"github.com/siderolabs/talos/pkg/provision"
)

// Destroy Talos cluster as set of Docker nodes.
//
// Only cluster.Info().ClusterName and cluster.Info().Network.Name is being used.
func (p *provisioner) Destroy(ctx context.Context, cluster provision.Cluster, opts ...provision.Option) error {
	options := provision.DefaultOptions()

	for _, opt := range opts {
		if err := opt(&options); err != nil {
			return err
		}
	}

	if err := p.destroyNodes(ctx, cluster.Info().ClusterName, &options); err != nil {
		return err
	}

	fmt.Fprintln(os.Stderr, "destroying network", cluster.Info().Network.Name)

	if err := p.destroyNetwork(ctx, cluster.Info().Network.Name); err != nil {
		return err
	}

	statePath, err := cluster.StatePath()
	if err != nil {
		return err
	}

	return os.RemoveAll(statePath)
}
