// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package multiplex

import (
	"context"
	"errors"
	"io"
	"sync"

	"github.com/siderolabs/gen/channel"
	"google.golang.org/grpc"

	"github.com/siderolabs/talos/pkg/machinery/client"
)

// Streaming initiates a multiplexed streaming gRPC client call to multiple nodes.
func Streaming[ResponseT any](ctx context.Context, nodes []string, initiate func(context.Context) (grpc.ServerStreamingClient[ResponseT], error)) <-chan Response[*ResponseT] {
	responseCh := make(chan Response[*ResponseT])

	var wg sync.WaitGroup

	for _, node := range nodes {
		wg.Go(func() {
			stream, err := initiate(client.WithNode(ctx, node))
			if err != nil {
				channel.SendWithContext(ctx, responseCh,
					Response[*ResponseT]{
						Node: node,
						Err:  err,
					},
				)

				return
			}

			for {
				payload, err := stream.Recv()
				if err != nil {
					if errors.Is(err, io.EOF) {
						return
					}

					channel.SendWithContext(ctx, responseCh,
						Response[*ResponseT]{
							Node: node,
							Err:  err,
						},
					)

					return
				}

				if !channel.SendWithContext(ctx, responseCh,
					Response[*ResponseT]{
						Node:    node,
						Payload: payload,
					},
				) {
					return
				}
			}
		})
	}

	go func() {
		wg.Wait()
		close(responseCh)
	}()

	return responseCh
}
