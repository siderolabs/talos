// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package providers

import (
	"context"
	"fmt"

	"github.com/siderolabs/talos/pkg/provision"
	"github.com/siderolabs/talos/pkg/provision/providers/docker"
)

const (
	// QemuProviderName is the name of the qemu provider.
	QemuProviderName = "qemu"
	// DockerProviderName is the name of the docker provider.
	DockerProviderName = "docker"
)

// Factory instantiates provision provider by name.
func Factory(ctx context.Context, name string) (provision.Provisioner, error) {
	if err := IsValidProvider(name); err != nil {
		return nil, err
	}

	switch name {
	case DockerProviderName:
		return docker.NewProvisioner(ctx)
	case QemuProviderName:
		return newQemu(ctx)
	}

	panic("unknown valid provisioner")
}

// IsValidProvider returns an error if the passed provider doesn't exist.
func IsValidProvider(name string) error {
	switch name {
	case QemuProviderName, DockerProviderName:
		return nil
	}

	return fmt.Errorf("unsupported provisioner %q", name)
}
