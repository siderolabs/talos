// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cluster

import (
	"context"
	"fmt"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"

	"github.com/siderolabs/talos/pkg/machinery/client"
	clientconfig "github.com/siderolabs/talos/pkg/machinery/client/config"
	secretsgen "github.com/siderolabs/talos/pkg/machinery/config/generate/secrets"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	"github.com/siderolabs/talos/pkg/machinery/resources/secrets"
	"github.com/siderolabs/talos/pkg/machinery/role"
)

// LocalClientProvider builds Talos client to connect to same-node apid instance over file socket.
type LocalClientProvider struct {
	client    *client.Client
	resources state.State
	roles     role.Set
}

// NewLocalClientProvider creates a new LocalClientProvider instance.
//
// This provider only works on controlplane nodes, as it relies on the
// root Talos API certificate being available.
func NewLocalClientProvider(resources state.State, roles role.Set) *LocalClientProvider {
	return &LocalClientProvider{
		resources: resources,
		roles:     roles,
	}
}

// Client returns Talos client instance for default (if no endpoints are given) or
// specific endpoints.
//
// Client implements ClientProvider interface.
func (c *LocalClientProvider) Client(endpoints ...string) (*client.Client, error) {
	if c.client != nil {
		return c.client, nil
	}

	ctx := context.TODO()

	rootSecrets, err := safe.StateGetByID[*secrets.OSRoot](ctx, c.resources, secrets.OSRootID)
	if err != nil {
		return nil, fmt.Errorf("failed to get OS root secrets: %w", err)
	}

	nodeAddress, err := safe.StateGetByID[*network.NodeAddress](ctx, c.resources, network.NodeAddressDefaultID)
	if err != nil {
		return nil, fmt.Errorf("failed to get node address: %w", err)
	}

	if len(nodeAddress.TypedSpec().IPs()) == 0 {
		return nil, fmt.Errorf("no node IPs found in node address")
	}

	if len(endpoints) == 0 {
		endpoints = []string{nodeAddress.TypedSpec().IPs()[0].String()}
	}

	// use a short-lived certificate, as we need to connect once
	const certificateTTL = 10 * time.Minute

	cert, err := secretsgen.NewAdminCertificateAndKey(
		time.Now(),
		rootSecrets.TypedSpec().IssuingCA,
		c.roles,
		certificateTTL,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to generate client certificate: %w", err)
	}

	talosconfig := clientconfig.NewConfig("local", endpoints, rootSecrets.TypedSpec().IssuingCA.Crt, cert)

	c.client, err = client.New(
		ctx,
		client.WithConfig(talosconfig),
	)

	return c.client, err
}

// Close all the client connections.
func (c *LocalClientProvider) Close() error {
	if c.client != nil {
		if err := c.client.Close(); err != nil {
			return err
		}

		c.client = nil
	}

	return nil
}
