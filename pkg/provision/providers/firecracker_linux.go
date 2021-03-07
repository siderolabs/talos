// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// +build linux

package providers

import (
	"context"

	"github.com/talos-systems/talos/pkg/provision"
	"github.com/talos-systems/talos/pkg/provision/providers/firecracker"
)

func newFirecracker(ctx context.Context) (provision.Provisioner, error) {
	return firecracker.NewProvisioner(ctx)
}
