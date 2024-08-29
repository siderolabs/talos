// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package helpers

import (
	"context"
	"errors"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/state"
	"google.golang.org/grpc/metadata"

	"github.com/siderolabs/talos/pkg/machinery/client"
)

// ForEachResource gets resources from the controller runtime and runs a callback for each resource.
//
//nolint:gocyclo
func ForEachResource(ctx context.Context,
	c *client.Client,
	callbackRD func(rd *meta.ResourceDefinition) error,
	callback func(ctx context.Context, hostname string, r resource.Resource, callError error) error,
	namespace string,
	args ...string,
) error {
	if len(args) == 0 {
		return errors.New("not enough arguments: at least 1 is expected")
	}

	resourceType := args[0]

	var resourceID string

	if len(args) > 1 {
		resourceID = args[1]
	}

	md, _ := metadata.FromOutgoingContext(ctx)
	nodes := md.Get("nodes")

	if len(nodes) == 0 {
		nodes = []string{""}
	}

	// fetch the RD from the first node (it doesn't matter which one to use, so we'll use the first one)
	rd, err := c.ResolveResourceKind(client.WithNode(ctx, nodes[0]), &namespace, resourceType)
	if err != nil {
		return err
	}

	if callbackRD != nil {
		if err = callbackRD(rd); err != nil {
			return err
		}
	}

	resourceType = rd.TypedSpec().Type

	for _, node := range nodes {
		var nodeCtx context.Context

		if node == "" {
			nodeCtx = ctx //nolint:fatcontext
		} else {
			nodeCtx = client.WithNode(ctx, node)
		}

		if resourceID != "" {
			r, callErr := c.COSI.Get(
				nodeCtx,
				resource.NewMetadata(namespace, rd.TypedSpec().Type, resourceID, resource.VersionUndefined),
				state.WithGetUnmarshalOptions(state.WithSkipProtobufUnmarshal()),
			)

			if err = callback(ctx, node, r, callErr); err != nil {
				return err
			}
		} else {
			items, callErr := c.COSI.List(
				nodeCtx,
				resource.NewMetadata(namespace, resourceType, "", resource.VersionUndefined),
				state.WithListUnmarshalOptions(state.WithSkipProtobufUnmarshal()),
			)
			if callErr != nil {
				if err = callback(ctx, node, nil, callErr); err != nil {
					return err
				}

				continue
			}

			for _, r := range items.Items {
				if err = callback(ctx, node, r, nil); err != nil {
					return err
				}
			}
		}
	}

	return nil
}
