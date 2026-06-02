// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package global provides global flags for talosctl.
package global

import (
	"context"
	"errors"

	"google.golang.org/grpc"

	"github.com/siderolabs/talos/pkg/machinery/client"
)

// ErrConfigContext is returned when config context cannot be resolved.
var ErrConfigContext = errors.New("failed to resolve config context")

// Args is a context for the Talos command line client.
type Args struct {
	Talosconfig     string
	CmdContext      string
	Cluster         string
	Nodes           []string
	Endpoints       []string
	SideroV1KeysDir string
}

// NodeList returns the list of nodes to run the command against.
//
// Deprecated: returns wrong information.
func (args *Args) NodeList() []string {
	return args.Nodes
}

// WithClientNoNodes wraps common code to initialize Talos client and provide cancellable context.
//
// WithClientNoNodes doesn't set any node information on the request context.
func (args *Args) WithClientNoNodes(ctx context.Context, action func(context.Context, *client.Client) error, dialOptions ...grpc.DialOption) error {
	factory, err := NewClientFactory(ctx, args, nil, dialOptions...)
	if err != nil {
		return err
	}

	defer factory.Close() //nolint:errcheck

	_, c, err := factory.BuildClient(ctx, "")
	if err != nil {
		return err
	}

	return action(ctx, c)
}

// WithClient builds upon WithClientNoNodes to provide set of nodes on request context based on config & flags.
func (args *Args) WithClient(ctx context.Context, action func(context.Context, *client.Client) error, dialOptions ...grpc.DialOption) error {
	factory, err := NewClientFactory(ctx, args, nil, dialOptions...)
	if err != nil {
		return err
	}

	defer factory.Close() //nolint:errcheck

	_, c, err := factory.BuildClient(ctx, "")
	if err != nil {
		return err
	}

	ctx = client.WithNodes(ctx, factory.Nodes()...) //nolint:staticcheck // to be refactored next

	return action(ctx, c)
}

// WithClientAndNodes builds upon WithClientNoNodes to provide a list of nodes to the function.
func (args *Args) WithClientAndNodes(ctx context.Context, action func(context.Context, *client.Client, []string) error, dialOptions ...grpc.DialOption) error {
	factory, err := NewClientFactory(ctx, args, nil, dialOptions...)
	if err != nil {
		return err
	}

	defer factory.Close() //nolint:errcheck

	_, c, err := factory.BuildClient(ctx, "")
	if err != nil {
		return err
	}

	return action(ctx, c, factory.Nodes())
}
