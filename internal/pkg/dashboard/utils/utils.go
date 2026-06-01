// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package utils contains utility functions for the dashboard.
package utils

import (
	"context"

	"github.com/siderolabs/talos/pkg/machinery/client"
)

// NodeContext returns a context with the node set if selectedNode is not empty.
func NodeContext(ctx context.Context, selectedNode string) context.Context {
	if selectedNode != "" {
		ctx = client.WithNode(ctx, selectedNode)
	}

	return ctx
}
