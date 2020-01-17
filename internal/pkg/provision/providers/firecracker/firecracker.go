// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package firecracker implements Provisioner via Firecracker VMs.
package firecracker

import (
	"context"

	"github.com/talos-systems/talos/internal/pkg/provision"
	"github.com/talos-systems/talos/pkg/config/types/v1alpha1/generate"
)

const stateFileName = "state.yaml"

type provisioner struct {
}

// NewProvisioner initializes docker provisioner.
func NewProvisioner(ctx context.Context) (provision.Provisioner, error) {
	p := &provisioner{}

	return p, nil
}

// Close and release resources.
func (p *provisioner) Close() error {
	return nil
}

// GenOptions provides a list of additional config generate options.
func (p *provisioner) GenOptions() []generate.GenOption {
	return []generate.GenOption{
		generate.WithInstallDisk("/dev/vda"),
	}
}
