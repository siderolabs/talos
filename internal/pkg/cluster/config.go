// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cluster

import (
	"strings"

	"github.com/talos-systems/talos/pkg/client"
	"github.com/talos-systems/talos/pkg/client/config"
	"github.com/talos-systems/talos/pkg/constants"
	"github.com/talos-systems/talos/pkg/grpc/tls"
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

	configContext, creds, err := client.NewClientContextAndCredentialsFromParsedConfig(c.TalosConfig, "")
	if err != nil {
		return nil, err
	}

	if len(endpoints) == 0 {
		endpoints = configContext.Endpoints
	}

	tlsconfig, err := tls.New(
		tls.WithKeypair(creds.Crt),
		tls.WithClientAuthType(tls.Mutual),
		tls.WithCACertPEM(creds.CA),
	)
	if err != nil {
		return nil, err
	}

	client, err := client.NewClient(tlsconfig, endpoints, constants.ApidPort)
	if err == nil {
		c.clients[key] = client
	}

	return client, err
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
