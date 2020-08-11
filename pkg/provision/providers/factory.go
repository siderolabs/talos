// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package providers

import (
	"context"
	"fmt"

	"github.com/talos-systems/talos/pkg/provision"
	"github.com/talos-systems/talos/pkg/provision/providers/docker"
)

// Factory instantiates provision provider by name.
func Factory(ctx context.Context, name string) (provision.Provisioner, error) {
	switch name {
	case "docker":
		return docker.NewProvisioner(ctx)
	case "firecracker":
		return newFirecracker(ctx)
	case "qemu":
		return newQemu(ctx)
	default:
		return nil, fmt.Errorf("unsupported provisioner %q", name)
	}
}
