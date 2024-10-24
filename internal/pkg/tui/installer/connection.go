// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package installer

import (
	"context"
	"net"

	"github.com/cosi-project/runtime/pkg/safe"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/api/storage"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// Connection unifies clients for bootstrap node and the node which is being configured.
type Connection struct {
	nodeEndpoint      string
	bootstrapEndpoint string
	nodeClient        *client.Client
	bootstrapClient   *client.Client
	nodeCtx           context.Context //nolint:containedctx
	bootstrapCtx      context.Context //nolint:containedctx
	dryRun            bool
}

// NewConnection creates new installer connection.
func NewConnection(ctx context.Context, nodeClient *client.Client, endpoint string, options ...Option) (
	*Connection,
	error,
) {
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
func (c *Connection) GenerateConfiguration(
	req *machine.GenerateConfigurationRequest,
	callOptions ...grpc.CallOption,
) (*machine.GenerateConfigurationResponse, error) {
	if c.bootstrapClient != nil {
		return c.bootstrapClient.GenerateConfiguration(c.bootstrapCtx, req, callOptions...)
	}

	return c.nodeClient.GenerateConfiguration(c.nodeCtx, req, callOptions...)
}

// ApplyConfiguration calls ApplyConfiguration on the target node using appropriate node context.
func (c *Connection) ApplyConfiguration(
	req *machine.ApplyConfigurationRequest,
	callOptions ...grpc.CallOption,
) (*machine.ApplyConfigurationResponse, error) {
	return c.nodeClient.ApplyConfiguration(c.nodeCtx, req, callOptions...)
}

// Disks get disks list from the target node.
func (c *Connection) Disks(callOptions ...grpc.CallOption) (*storage.DisksResponse, error) {
	return c.nodeClient.Disks(c.nodeCtx, callOptions...)
}

// Link a subset of fields from LinkStatus resource.
type Link struct {
	Name         string
	Physical     bool
	Up           bool
	HardwareAddr net.HardwareAddr
	MTU          int
}

// Links gets a list of network interfaces.
func (c *Connection) Links() ([]Link, error) {
	ctx := c.nodeCtx

	md, _ := metadata.FromOutgoingContext(c.nodeCtx)
	if nodes := md["nodes"]; len(nodes) > 0 {
		ctx = client.WithNode(ctx, nodes[0])
	}

	items, err := safe.StateListAll[*network.LinkStatus](ctx, c.nodeClient.COSI)
	if err != nil {
		return nil, err
	}

	links := make([]Link, 0, items.Len())

	for res := range items.All() {
		var link Link

		link.Name = res.Metadata().ID()
		link.Physical = res.TypedSpec().Physical()
		link.MTU = int(res.TypedSpec().MTU)

		switch res.TypedSpec().OperationalState { //nolint:exhaustive
		case nethelpers.OperStateUnknown:
			link.Up = true
		case nethelpers.OperStateUp:
			link.Up = true
		default:
			link.Up = false
		}

		link.HardwareAddr = net.HardwareAddr(res.TypedSpec().HardwareAddr)

		links = append(links, link)
	}

	return links, nil
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
		c.bootstrapCtx = ctx //nolint:fatcontext

		return nil
	}
}

// WithDryRun enables dry run mode in the installer.
func WithDryRun(dryRun bool) Option {
	return func(c *Connection) error {
		c.dryRun = dryRun

		return nil
	}
}
