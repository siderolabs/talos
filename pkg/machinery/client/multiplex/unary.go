// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package multiplex

import (
	"context"
	"sync"

	"github.com/siderolabs/gen/channel"

	"github.com/siderolabs/talos/pkg/machinery/client"
)

// Unary initiates a multiplexed unary gRPC client call to multiple nodes.
func Unary[ResponseT any](ctx context.Context, nodes []string, initiate func(context.Context) (ResponseT, error)) <-chan Response[ResponseT] {
	responseCh := make(chan Response[ResponseT])

	var wg sync.WaitGroup

	for _, node := range nodes {
		wg.Go(func() {
			response, err := initiate(client.WithNode(ctx, node))
			if err != nil {
				channel.SendWithContext(
					ctx, responseCh,
					Response[ResponseT]{
						Node: node,
						Err:  err,
					},
				)

				return
			}

			channel.SendWithContext(
				ctx, responseCh,
				Response[ResponseT]{
					Node:    node,
					Payload: response,
				},
			)
		})
	}

	go func() {
		wg.Wait()
		close(responseCh)
	}()

	return responseCh
}

// UnaryViaFactory is a helper which performs unary multiplexing using a ClientFactory to create clients for each node.
func UnaryViaFactory[ResponseT any](origCtx context.Context, factory ClientFactory, initiate func(context.Context, *client.Client) (ResponseT, error)) <-chan Response[ResponseT] {
	responseCh := make(chan Response[ResponseT])

	var wg sync.WaitGroup

	for _, node := range factory.Nodes() {
		wg.Go(func() {
			ctx, client, err := factory.BuildClient(origCtx, node)
			if err != nil {
				channel.SendWithContext(
					origCtx, responseCh,
					Response[ResponseT]{
						Node: node,
						Err:  err,
					},
				)

				return
			}

			response, err := initiate(ctx, client)
			if err != nil {
				channel.SendWithContext(
					ctx, responseCh,
					Response[ResponseT]{
						Node: node,
						Err:  err,
					},
				)

				return
			}

			channel.SendWithContext(
				ctx, responseCh,
				Response[ResponseT]{
					Node:    node,
					Payload: response,
				},
			)
		})
	}

	go func() {
		wg.Wait()
		close(responseCh)
	}()

	return responseCh
}
