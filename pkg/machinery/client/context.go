// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package client

import (
	"context"

	"google.golang.org/grpc/metadata"
)

// WithNodes wraps the context with metadata to send request to a set of nodes.
//
// Responses from all nodes are aggregated by the `apid` service and sent back as a single response.
func WithNodes(ctx context.Context, nodes ...string) context.Context {
	md, _ := metadata.FromOutgoingContext(ctx)

	// overwrite any previous nodes in the context metadata with new value
	md = md.Copy()
	md.Delete("node")
	md.Set("nodes", nodes...)

	return metadata.NewOutgoingContext(ctx, md)
}

// WithNode wraps the context with metadata to send request to a single node.
//
// Request will be proxied by the endpoint to the specified node without any further processing.
func WithNode(ctx context.Context, node string) context.Context {
	md, _ := metadata.FromOutgoingContext(ctx)

	// overwrite any previous nodes in the context metadata with new value
	md = md.Copy()
	md.Delete("nodes")
	md.Set("node", node)

	return metadata.NewOutgoingContext(ctx, md)
}

// ClearNodeMetadata removes any node/nodeS metadata from the context.
func ClearNodeMetadata(ctx context.Context) context.Context {
	md, _ := metadata.FromOutgoingContext(ctx)

	// overwrite any previous nodes in the context metadata with new value
	md = md.Copy()
	md.Delete("nodes")
	md.Delete("node")

	return metadata.NewOutgoingContext(ctx, md)
}
