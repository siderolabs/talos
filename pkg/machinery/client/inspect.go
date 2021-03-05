// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package client

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	inspectapi "github.com/talos-systems/talos/pkg/machinery/api/inspect"
)

// InspectClient provides access to inspect API.
type InspectClient struct {
	client inspectapi.InspectServiceClient
}

// ControllerRuntimeDependencies returns graph describing dependencies between controllers.
func (c *InspectClient) ControllerRuntimeDependencies(ctx context.Context, callOptions ...grpc.CallOption) (*inspectapi.ControllerRuntimeDependenciesResponse, error) {
	resp, err := c.client.ControllerRuntimeDependencies(ctx, &empty.Empty{}, callOptions...)

	var filtered interface{}
	filtered, err = FilterMessages(resp, err)
	resp, _ = filtered.(*inspectapi.ControllerRuntimeDependenciesResponse) //nolint:errcheck

	return resp, err
}
