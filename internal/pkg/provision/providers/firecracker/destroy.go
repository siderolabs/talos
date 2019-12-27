// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package firecracker

import (
	"context"

	"github.com/talos-systems/talos/internal/pkg/provision"
)

// Destroy Talos cluster as set of Firecracker VMs.
func (p *provisioner) Destroy(ctx context.Context, cluster provision.Cluster, opts ...provision.Option) error {
	options := provision.DefaultOptions()

	for _, opt := range opts {
		if err := opt(&options); err != nil {
			return err
		}
	}

	// TODO: implement me

	return nil
}
