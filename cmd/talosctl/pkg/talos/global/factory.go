// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package global

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"google.golang.org/grpc"

	"github.com/siderolabs/talos/pkg/machinery/client"
	clientconfig "github.com/siderolabs/talos/pkg/machinery/client/config"
)

// ClientFactory produces Talos clients and nodes arguments.
type ClientFactory struct {
	args          *Args
	insecureFlags InsecureArgser
	dialOptions   []grpc.DialOption
	nodes         []string

	mu             sync.Mutex
	client         *client.Client
	perNodeClients map[string]*client.Client
}

// NewClientFactory creates a new ClientFactory.
func NewClientFactory(ctx context.Context, args *Args, flags any, dialOptions ...grpc.DialOption) (*ClientFactory, error) {
	factory := &ClientFactory{
		args:        args,
		dialOptions: dialOptions,
	}

	if insecureFlags, ok := flags.(InsecureArgser); ok {
		factory.insecureFlags = insecureFlags
	}

	// if args were set on the command line, always prefer them
	if len(args.Nodes) > 0 {
		factory.nodes = args.Nodes
	} else if factory.insecureFlags == nil || !factory.insecureFlags.GetInsecureFlag() {
		// in secure mode, we can pull nodes from the config context, so build a client to parse the config
		c, err := factory.buildClientFromConfig(ctx)
		if err != nil {
			return nil, err
		}

		configContext := c.GetConfigContext()
		if configContext == nil {
			factory.Close() //nolint:errcheck

			return nil, ErrConfigContext
		}

		factory.nodes = configContext.Nodes
		factory.client = c
	}

	if len(factory.nodes) < 1 {
		factory.Close() //nolint:errcheck

		return nil, errors.New("nodes are not set for the command: please use `--nodes` flag or configuration file to set the nodes to run the command against")
	}

	return factory, nil
}

// Nodes provides a list of nodes to run the command against.
func (f *ClientFactory) Nodes() []string {
	return f.nodes
}

// Close all clients created by the factory.
func (f *ClientFactory) Close() error {
	var errs error

	if f.client != nil {
		errs = errors.Join(errs, f.client.Close())
	}

	for _, perNodeClient := range f.perNodeClients {
		if err := perNodeClient.Close(); err != nil {
			errs = errors.Join(errs, err)
		}
	}

	return errs
}

func (f *ClientFactory) buildMaintenanceClient(ctx context.Context, node string) (*client.Client, error) {
	return client.New(
		ctx,
		client.WithDefaultGRPCDialOptions(),
		client.WithMaintenanceMode(node, f.insecureFlags.GetCertFingerprints()),
		client.WithGRPCDialOptions(f.dialOptions...),
	)
}

func (f *ClientFactory) buildClientFromConfig(ctx context.Context) (*client.Client, error) {
	cfg, err := clientconfig.Open(f.args.Talosconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to open config file %q: %w", f.args.Talosconfig, err)
	}

	opts := []client.OptionFunc{
		client.WithConfig(cfg),
		client.WithDefaultGRPCDialOptions(),
		client.WithGRPCDialOptions(f.dialOptions...),
		client.WithSideroV1KeysDir(clientconfig.CustomSideroV1KeysDirPath(f.args.SideroV1KeysDir)),
	}

	if f.args.CmdContext != "" {
		opts = append(opts, client.WithContextName(f.args.CmdContext))
	}

	if len(f.args.Endpoints) > 0 {
		// override endpoints from command-line flags
		opts = append(opts, client.WithEndpoints(f.args.Endpoints...))
	}

	if f.args.Cluster != "" {
		opts = append(opts, client.WithCluster(f.args.Cluster))
	}

	return client.New(ctx, opts...)
}

// BuildClient builds a Talos client and for a specific node.
//
//nolint:gocyclo
func (f *ClientFactory) BuildClient(ctx context.Context, node string) (context.Context, *client.Client, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.client != nil {
		return client.WithNode(ctx, node), f.client, nil
	}

	if perNodeClient, ok := f.perNodeClients[node]; ok {
		return ctx, perNodeClient, nil
	}

	if f.insecureFlags != nil && f.insecureFlags.GetInsecureFlag() {
		c, err := f.buildMaintenanceClient(ctx, node)
		if err != nil {
			return nil, nil, fmt.Errorf("error constructing maintenance mode client for node %q: %w", node, err)
		}

		if f.perNodeClients == nil {
			f.perNodeClients = make(map[string]*client.Client)
		}

		f.perNodeClients[node] = c

		return ctx, c, nil
	}

	c, err := f.buildClientFromConfig(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("error constructing client: %w", err)
	}

	f.client = c

	return client.WithNode(ctx, node), c, nil
}

// BuildClientFirstNode is a helper which builds a client for the first node in the list.
func (f *ClientFactory) BuildClientFirstNode(ctx context.Context) (context.Context, *client.Client, error) {
	// the nodes list is non-empty (see NewClientFactory), so this is safe
	return f.BuildClient(ctx, f.nodes[0])
}

// BuildClientEnforceSingleNode is a helper which enforces that there is exactly one node in the list, and builds a client for it.
func (f *ClientFactory) BuildClientEnforceSingleNode(ctx context.Context, commandName string) (context.Context, *client.Client, string, error) {
	if len(f.nodes) != 1 {
		return nil, nil, "", fmt.Errorf("command %q requires exactly one node (got %d)", commandName, len(f.nodes))
	}

	ctx, c, err := f.BuildClientFirstNode(ctx)

	return ctx, c, f.nodes[0], err
}

// BuildRandomEndpointClient is a helper which builds which talks to a random endpoint in the cluster.
//
// Note: this method is dangerous, as every new gRPC call might land on a different node.
//
// This method can't build maintenance mode clients.
func (f *ClientFactory) BuildRandomEndpointClient(ctx context.Context) (*client.Client, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.insecureFlags != nil && f.insecureFlags.GetInsecureFlag() {
		return nil, errors.New("random endpoint client can't be used in insecure mode")
	}

	if f.client != nil {
		return f.client, nil
	}

	c, err := f.buildClientFromConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("error constructing client: %w", err)
	}

	f.client = c

	return c, nil
}
