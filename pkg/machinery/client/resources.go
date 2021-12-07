// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package client

import (
	"context"
	"errors"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"

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

// WatchResponse is a parsed resource watch response.
type WatchResponse struct {
	ResourceResponse
	EventType state.EventType
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
	resp, _ = filtered.(*resourceapi.GetResponse) //nolint:errcheck

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

// ResourceWatchClient wraps gRPC watch client.
type ResourceWatchClient struct {
	grpcClient resourceapi.ResourceService_WatchClient
}

// Recv next item from the list.
//
//nolint:gocyclo
func (client *ResourceWatchClient) Recv() (WatchResponse, error) {
	var watchResp WatchResponse

	msg, err := client.grpcClient.Recv()
	if err != nil {
		return watchResp, err
	}

	if msg.GetMetadata().GetError() != "" {
		if msg.GetMetadata().Status != nil {
			return watchResp, status.ErrorProto(msg.GetMetadata().GetStatus())
		}

		return watchResp, errors.New(msg.GetMetadata().GetError())
	}

	if msg.GetDefinition() != nil {
		var e error

		watchResp.Definition, e = resource.NewAnyFromProto(msg.GetDefinition().GetMetadata(), msg.GetDefinition().GetSpec())
		if e != nil {
			return watchResp, e
		}
	}

	if msg.GetResource() != nil {
		var e error

		watchResp.Resource, e = resource.NewAnyFromProto(msg.GetResource().GetMetadata(), msg.GetResource().GetSpec())
		if e != nil {
			return watchResp, e
		}
	}

	switch msg.GetEventType() {
	case resourceapi.EventType_CREATED:
		watchResp.EventType = state.Created
	case resourceapi.EventType_UPDATED:
		watchResp.EventType = state.Updated
	case resourceapi.EventType_DESTROYED:
		watchResp.EventType = state.Destroyed
	}

	return watchResp, nil
}

// Watch resources by kind or by kind and ID.
func (c *ResourcesClient) Watch(ctx context.Context, resourceNamespace, resourceType, resourceID string, callOptions ...grpc.CallOption) (*ResourceWatchClient, error) {
	return c.WatchRequest(ctx, &resourceapi.WatchRequest{
		Namespace: resourceNamespace,
		Type:      resourceType,
		Id:        resourceID,
	}, callOptions...)
}

// WatchRequest resources by watch request.
func (c *ResourcesClient) WatchRequest(ctx context.Context, request *resourceapi.WatchRequest, callOptions ...grpc.CallOption) (*ResourceWatchClient, error) {
	client, err := c.client.Watch(ctx, request, callOptions...)

	return &ResourceWatchClient{
		grpcClient: client,
	}, err
}
