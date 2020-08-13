// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cluster

import (
	"context"
	"strings"

	"github.com/talos-systems/talos/pkg/machinery/client"
	"github.com/talos-systems/talos/pkg/machinery/client/config"
)

// ConfigClientProvider builds Talos client from client config.
type ConfigClientProvider struct {
	// DefaultClient to be used when using default endpoints.
	//
	// Not required, if missing client will be constructed from the config.
	DefaultClient *client.Client

	// TalosConfig is a client Talos configuration.
	TalosConfig *config.Config

	clients map[string]*client.Client
}

// Client returns Talos client instance for default (if no endpoints are given) or
// specific endpoints.
//
// Client implements ClientProvider interface.
func (c *ConfigClientProvider) Client(endpoints ...string) (*client.Client, error) {
	key := strings.Join(endpoints, ",")

	if c.clients == nil {
		c.clients = make(map[string]*client.Client)
	}

	if cli := c.clients[key]; cli != nil {
		return cli, nil
	}

	if len(endpoints) == 0 && c.DefaultClient != nil {
		return c.DefaultClient, nil
	}

	opts := []client.OptionFunc{
		client.WithConfig(c.TalosConfig),
	}

	if len(endpoints) > 0 {
		opts = append(opts, client.WithEndpoints(endpoints...))
	}

	return client.New(context.TODO(), opts...)
}

// Close all the client connections.
func (c *ConfigClientProvider) Close() error {
	for _, cli := range c.clients {
		if err := cli.Close(); err != nil {
			return err
		}
	}

	c.clients = nil

	return nil
}
