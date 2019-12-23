// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package docker implements Provisioner via docker.
package docker

import (
	"context"

	"github.com/docker/docker/client"

	"github.com/talos-systems/talos/internal/pkg/provision"
)

type provisioner struct {
	client *client.Client
}

// NewProvisioner initializes docker provisioner.
func NewProvisioner(ctx context.Context) (provision.Provisioner, error) {
	p := &provisioner{}

	var err error

	p.client, err = client.NewEnvClient()
	if err != nil {
		return nil, err
	}

	return p, nil
}

// Close and release resources.
func (p *provisioner) Close() error {
	if p.client != nil {
		return p.client.Close()
	}

	return nil
}
