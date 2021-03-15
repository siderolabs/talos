// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"context"
	"fmt"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/talos-systems/os-runtime/pkg/controller"

	inspectapi "github.com/talos-systems/talos/pkg/machinery/api/inspect"
)

// InspectServer implements InspectService API.
type InspectServer struct {
	inspectapi.UnimplementedInspectServiceServer

	server *Server
}

// ControllerRuntimeDependencies implements inspect.InspectService interface.
func (s *InspectServer) ControllerRuntimeDependencies(ctx context.Context, in *empty.Empty) (*inspectapi.ControllerRuntimeDependenciesResponse, error) {
	graph, err := s.server.Controller.V1Alpha2().DependencyGraph()
	if err != nil {
		return nil, fmt.Errorf("error fetching dependency graph: %w", err)
	}

	edges := make([]*inspectapi.ControllerDependencyEdge, 0, len(graph.Edges))

	for i := range graph.Edges {
		var edgeType inspectapi.DependencyEdgeType

		switch graph.Edges[i].EdgeType {
		case controller.EdgeManages:
			edgeType = inspectapi.DependencyEdgeType_MANAGES
		case controller.EdgeDependsStrong:
			edgeType = inspectapi.DependencyEdgeType_STRONG
		case controller.EdgeDependsWeak:
			edgeType = inspectapi.DependencyEdgeType_WEAK
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
