// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cluster

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/talos-systems/talos/pkg/machinery/client"
	"github.com/talos-systems/talos/pkg/machinery/constants"
)

// LocalClientProvider builds Talos client to connect to same-node apid instance over file socket.
type LocalClientProvider struct {
	client *client.Client
}

// Client returns Talos client instance for default (if no endpoints are given) or
// specific endpoints.
//
// Client implements ClientProvider interface.
func (c *LocalClientProvider) Client(endpoints ...string) (*client.Client, error) {
	if len(endpoints) > 0 {
		return nil, fmt.Errorf("custom endpoints not supported with LocalClientProvider")
	}

	var err error

	if c.client == nil {
		c.client, err = client.New(context.TODO(), client.WithUnixSocket(constants.APISocketPath), client.WithGRPCDialOptions(grpc.WithTransportCredentials(insecure.NewCredentials())))
	}

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
