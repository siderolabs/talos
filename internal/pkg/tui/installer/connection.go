// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package installer

import (
	"context"

	"google.golang.org/grpc"

	"github.com/talos-systems/talos/pkg/machinery/api/machine"
	"github.com/talos-systems/talos/pkg/machinery/api/network"
	"github.com/talos-systems/talos/pkg/machinery/api/storage"
	"github.com/talos-systems/talos/pkg/machinery/client"
)

// Connection unifies clients for bootstrap node and the node which is being configured.
type Connection struct {
	nodeEndpoint      string
	bootstrapEndpoint string
	nodeClient        *client.Client
	bootstrapClient   *client.Client
	nodeCtx           context.Context
	bootstrapCtx      context.Context
}

// NewConnection creates new installer connection.
func NewConnection(ctx context.Context, nodeClient *client.Client, endpoint string, options ...Option) (*Connection, error) {
	c := &Connection{
		nodeEndpoint: endpoint,
		nodeClient:   nodeClient,
		nodeCtx:      ctx,
	}

	for _, opt := range options {
		err := opt(c)
		if err != nil {
			return nil, err
		}
	}

	return c, nil
}

// GenerateConfiguration calls GenerateConfiguration on the target/bootstrap node.
func (c *Connection) GenerateConfiguration(req *machine.GenerateConfigurationRequest, callOptions ...grpc.CallOption) (*machine.GenerateConfigurationResponse, error) {
	if c.bootstrapClient != nil {
		return c.bootstrapClient.GenerateConfiguration(c.bootstrapCtx, req, callOptions...)
	}

	return c.nodeClient.GenerateConfiguration(c.nodeCtx, req, callOptions...)
}

// ApplyConfiguration calls ApplyConfiguration on the target node using appropriate node context.
func (c *Connection) ApplyConfiguration(req *machine.ApplyConfigurationRequest, callOptions ...grpc.CallOption) (*machine.ApplyConfigurationResponse, error) {
	return c.nodeClient.ApplyConfiguration(c.nodeCtx, req, callOptions...)
}

// Disks get disks list from the target node.
func (c *Connection) Disks(callOptions ...grpc.CallOption) (*storage.DisksResponse, error) {
	return c.nodeClient.Disks(c.nodeCtx, callOptions...)
}

// Interfaces get list of network interfaces.
func (c *Connection) Interfaces(callOptions ...grpc.CallOption) (*network.InterfacesResponse, error) {
	return c.nodeClient.Interfaces(c.nodeCtx, callOptions...)
}

// ExpandingCluster check if bootstrap node is set.
func (c *Connection) ExpandingCluster() bool {
	return c.bootstrapClient != nil
}

// Option represents a single connection option.
type Option func(c *Connection) error

// WithBootstrapNode configures bootstrap node endpoint.
func WithBootstrapNode(ctx context.Context, bootstrapClient *client.Client, bootstrapNode string) Option {
	return func(c *Connection) error {
		c.bootstrapEndpoint = bootstrapNode
		c.bootstrapClient = bootstrapClient
		c.bootstrapCtx = ctx

		return nil
	}
}
