// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package dashboard

import (
	"context"

	"google.golang.org/grpc/metadata"

	"github.com/siderolabs/talos/pkg/machinery/client"
)

func nodeContext(ctx context.Context, selectedNode string) context.Context {
	md, mdOk := metadata.FromOutgoingContext(ctx)
	if mdOk {
		md.Delete("nodes")

		ctx = metadata.NewOutgoingContext(ctx, md)
	}

	if selectedNode != "" {
		ctx = client.WithNode(ctx, selectedNode)
	}

	return ctx
}
