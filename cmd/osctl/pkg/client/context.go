// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package client

import (
	"context"

	"google.golang.org/grpc/metadata"
)

// WithTargets wraps the context with metadata to send request to set of nodes.
func WithTargets(ctx context.Context, target ...string) context.Context {
	md := metadata.New(nil)
	md.Set("targets", target...)

	return metadata.NewOutgoingContext(ctx, md)
}
