// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package client

import (
	"context"

	"google.golang.org/grpc/metadata"
)

// WithNodes wraps the context with metadata to send request to set of nodes.
func WithNodes(ctx context.Context, nodes ...string) context.Context {
	if len(nodes) == 0 {
		return ctx
	}

	md := metadata.New(nil)
	md.Set("nodes", nodes...)

	return metadata.NewOutgoingContext(ctx, md)
}
