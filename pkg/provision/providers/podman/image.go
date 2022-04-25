// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package podman

import (
	"context"

	"github.com/containers/podman/v4/pkg/bindings/images"

	"github.com/talos-systems/talos/pkg/provision"
)

func (p *provisioner) ensureImageExists(ctx context.Context, image string, options *provision.Options) error {
	imageExists, err := images.Exists(p.connection, image, &images.ExistsOptions{})
	if err != nil {
		return err
	}

	if !imageExists {
		_, err = images.Pull(p.connection, image, &images.PullOptions{})
		if err != nil {
			return err
		}
	}

	return nil
}
