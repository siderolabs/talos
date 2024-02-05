// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"google.golang.org/protobuf/types/known/emptypb"

	inspectapi "github.com/siderolabs/talos/pkg/machinery/api/inspect"
)

// InspectServer implements InspectService API.
type InspectServer struct {
	inspectapi.UnimplementedInspectServiceServer

	server *Server
}

// ControllerRuntimeDependencies implements inspect.InspectService interface.
func (s *InspectServer) ControllerRuntimeDependencies(ctx context.Context, in *emptypb.Empty) (*inspectapi.ControllerRuntimeDependenciesResponse, error) {
	graph, err := s.server.Controller.V1Alpha2().DependencyGraph()
	if err != nil {
		return nil, fmt.Errorf("error fetching dependency graph: %w", err)
	}

	edges := make([]*inspectapi.ControllerDependencyEdge, 0, len(graph.Edges))

	for i := range graph.Edges {
		var edgeType inspectapi.DependencyEdgeType

		switch graph.Edges[i].EdgeType {
		case controller.EdgeOutputExclusive:
			edgeType = inspectapi.DependencyEdgeType_OUTPUT_EXCLUSIVE
		case controller.EdgeOutputShared:
			edgeType = inspectapi.DependencyEdgeType_OUTPUT_SHARED
		case controller.EdgeInputStrong:
			edgeType = inspectapi.DependencyEdgeType_INPUT_STRONG
		case controller.EdgeInputWeak:
			edgeType = inspectapi.DependencyEdgeType_INPUT_WEAK
		case controller.EdgeInputDestroyReady:
			edgeType = inspectapi.DependencyEdgeType_INPUT_DESTROY_READY
		case controller.EdgeInputQPrimary,
			controller.EdgeInputQMapped,
			controller.EdgeInputQMappedDestroyReady:
			return nil, fmt.Errorf("unexpected edge type: %v", graph.Edges[i].EdgeType)
		}

		edges = append(edges, &inspectapi.ControllerDependencyEdge{
			ControllerName: graph.Edges[i].ControllerName,

			EdgeType: edgeType,

			ResourceNamespace: graph.Edges[i].ResourceNamespace,
			ResourceType:      graph.Edges[i].ResourceType,
			ResourceId:        graph.Edges[i].ResourceID,
		})
	}

	return &inspectapi.ControllerRuntimeDependenciesResponse{
		Messages: []*inspectapi.ControllerRuntimeDependency{
			{
				Edges: edges,
			},
		},
	}, nil
}
