// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package installer

import (
	"context"
	"io"
	"net"

	"google.golang.org/grpc"
	"gopkg.in/yaml.v3"

	"github.com/talos-systems/talos/pkg/machinery/api/machine"
	"github.com/talos-systems/talos/pkg/machinery/api/storage"
	"github.com/talos-systems/talos/pkg/machinery/client"
	"github.com/talos-systems/talos/pkg/machinery/nethelpers"
	"github.com/talos-systems/talos/pkg/machinery/resources/network"
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

// Link a subset of fields from LinkStatus resource.
type Link struct {
	Name         string
	Physical     bool
	Up           bool
	HardwareAddr net.HardwareAddr
	MTU          int
}

// Links gets a list of network interfaces.
//
//nolint:gocyclo
func (c *Connection) Links(callOptions ...grpc.CallOption) ([]Link, error) {
	client, err := c.nodeClient.Resources.List(c.nodeCtx, network.NamespaceName, network.LinkStatusType, callOptions...)
	if err != nil {
		return nil, err
	}

	var links []Link

	for {
		msg, err := client.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}

			return nil, err
		}

		if msg.Resource == nil {
			continue
		}

		var link Link

		// this is a hack until we get proper encoding for resources in the API (protobuf!)
		// plus our resources are Linux-specific and don't build on OS X (we need to solve this as well!)

		link.Name = msg.Resource.Metadata().ID()

		b, err := yaml.Marshal(msg.Resource.Spec())
		if err != nil {
			return nil, err
		}

		var raw map[string]interface{}

		if err = yaml.Unmarshal(b, &raw); err != nil {
			return nil, err
		}

		kind := raw["kind"].(string) //nolint:errcheck,forcetypeassert

		linkType := raw["type"].(string) //nolint:errcheck,forcetypeassert

		link.Physical = kind == "" && linkType == "ether"
		link.MTU = raw["mtu"].(int) //nolint:errcheck,forcetypeassert

		switch raw["operationalState"].(string) {
		case nethelpers.OperStateUnknown.String():
			link.Up = true
		case nethelpers.OperStateUp.String():
			link.Up = true
		default:
			link.Up = false
		}

		mac, err := net.ParseMAC(raw["hardwareAddr"].(string))
		if err == nil {
			link.HardwareAddr = mac
		}

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
		c.bootstrapCtx = ctx

		return nil
	}
}
