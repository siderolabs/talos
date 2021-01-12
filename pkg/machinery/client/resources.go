// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package client

import (
	"context"

	"github.com/talos-systems/os-runtime/pkg/resource"
	"google.golang.org/grpc"

	"github.com/talos-systems/talos/pkg/machinery/api/common"
	resourceapi "github.com/talos-systems/talos/pkg/machinery/api/resource"
)

// ResourcesClient provides access to resource API.
type ResourcesClient struct {
	client resourceapi.ResourceServiceClient
}

// ResourceResponse is a parsed resource response.
type ResourceResponse struct {
	Metadata   *common.Metadata
	Definition resource.Resource
	Resource   resource.Resource
}

// Get a specified resource.
func (c *ResourcesClient) Get(ctx context.Context, resourceNamespace, resourceType, resourceID string, callOptions ...grpc.CallOption) ([]ResourceResponse, error) {
	resp, err := c.client.Get(ctx, &resourceapi.GetRequest{
		Namespace: resourceNamespace,
		Type:      resourceType,
		Id:        resourceID,
	}, callOptions...)

	var filtered interface{}
	filtered, err = FilterMessages(resp, err)
	resp, _ = filtered.(*resourceapi.GetResponse) //nolint: errcheck

	if resp == nil {
		return nil, err
	}

	items := make([]ResourceResponse, 0, len(resp.GetMessages()))

	for _, msg := range resp.GetMessages() {
		var resourceResp ResourceResponse

		resourceResp.Metadata = msg.GetMetadata()

		if msg.GetDefinition() != nil {
			var e error

			resourceResp.Definition, e = resource.NewAnyFromProto(msg.GetDefinition().GetMetadata(), msg.GetDefinition().GetSpec())
			if e != nil {
				return nil, e
			}
		}

		if msg.GetResource() != nil {
			var e error

			resourceResp.Resource, e = resource.NewAnyFromProto(msg.GetResource().GetMetadata(), msg.GetResource().GetSpec())
			if e != nil {
				return nil, e
			}
		}

		items = append(items, resourceResp)
	}

	return items, err
}

// ResourceListClient wraps gRPC list client.
type ResourceListClient struct {
	grpcClient resourceapi.ResourceService_ListClient
}

// Recv next item from the list.
func (client *ResourceListClient) Recv() (ResourceResponse, error) {
	var resourceResp ResourceResponse

	msg, err := client.grpcClient.Recv()
	if err != nil {
		return resourceResp, err
	}

	resourceResp.Metadata = msg.GetMetadata()

	if msg.GetDefinition() != nil {
		var e error

		resourceResp.Definition, e = resource.NewAnyFromProto(msg.GetDefinition().GetMetadata(), msg.GetDefinition().GetSpec())
		if e != nil {
			return resourceResp, e
		}
	}

	if msg.GetResource() != nil {
		var e error

		resourceResp.Resource, e = resource.NewAnyFromProto(msg.GetResource().GetMetadata(), msg.GetResource().GetSpec())
		if e != nil {
			return resourceResp, e
		}
	}

	return resourceResp, nil
}

// List resources by kind.
func (c *ResourcesClient) List(ctx context.Context, resourceNamespace, resourceType string, callOptions ...grpc.CallOption) (*ResourceListClient, error) {
	client, err := c.client.List(ctx, &resourceapi.ListRequest{
		Namespace: resourceNamespace,
		Type:      resourceType,
	}, callOptions...)

	return &ResourceListClient{
		grpcClient: client,
	}, err
}
