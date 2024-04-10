// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package util provides utility functions for the dashboard.
package util

import (
	"context"

	"google.golang.org/grpc/metadata"

	"github.com/siderolabs/talos/pkg/machinery/client"
)

// NodeContext contains the context.Context for a single node and the node name.
type NodeContext struct {
	Ctx  context.Context //nolint:containedctx
	Node string
}

// NodeContexts returns a list of NodeContexts from the given context.
//
// It extracts the node names from the outgoing GRPC context metadata.
// If the node name is not present in the metadata, context will be returned as-is with an empty node name.
func NodeContexts(ctx context.Context) []NodeContext {
	md, mdOk := metadata.FromOutgoingContext(ctx)
	if !mdOk {
		return []NodeContext{{Ctx: ctx}}
	}

	nodeVal := md.Get("node")
	if len(nodeVal) > 0 {
		return []NodeContext{{Ctx: ctx, Node: nodeVal[0]}}
	}

	nodesVal := md.Get("nodes")
	if len(nodesVal) == 0 {
		return []NodeContext{{Ctx: ctx}}
	}

	nodeContexts := make([]NodeContext, 0, len(nodesVal))

	for _, node := range nodesVal {
		nodeContexts = append(nodeContexts, NodeContext{Ctx: client.WithNode(ctx, node), Node: node})
	}

	return nodeContexts
}
