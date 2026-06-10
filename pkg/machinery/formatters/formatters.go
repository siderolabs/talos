// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package formatters contains the API response formatters used in the CLI output.
package formatters

import (
	"context"
	"fmt"
	"io"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/emicklei/dot"

	"github.com/siderolabs/talos/pkg/machinery/api/inspect"
	"github.com/siderolabs/talos/pkg/machinery/client"
)

// RenderGraph renders inspect controller runtime graph.
//
//nolint:gocyclo,cyclop
func RenderGraph(ctx context.Context, c *client.Client, resp *inspect.ControllerRuntimeDependenciesResponse, output io.Writer, withResources bool) error {
	graph := dot.NewGraph(dot.Directed)

	resourceTypeID := func(edge *inspect.ControllerDependencyEdge) string {
		return edge.GetResourceType()
	}

	resourceID := func(r resource.Resource) string {
		return fmt.Sprintf("%s/%s/%s", r.Metadata().Namespace(), r.Metadata().Type(), r.Metadata().ID())
	}

	if withResources {
		resources := map[string][]resource.Resource{}

		for _, msg := range resp.GetMessages() {
			for _, edge := range msg.GetEdges() {
				resourceType := resourceTypeID(edge)

				if _, ok := resources[resourceType]; ok {
					continue
				}

				namespace := edge.GetResourceNamespace()

				rd, err := c.ResolveResourceKind(ctx, &namespace, edge.GetResourceType())
				if err != nil {
					return err
				}

				items, err := c.COSI.List(ctx, resource.NewMetadata(namespace, rd.TypedSpec().Type, "", resource.VersionUndefined))
				if err != nil {
					// ignore errors here
					continue
				}

				resources[resourceType] = append(resources[resourceType], items.Items...)
			}
		}

		for _, msg := range resp.GetMessages() {
			for _, edge := range msg.GetEdges() {
				graph.Node(edge.ControllerName).Box()
			}
		}

		for resourceType, resourceList := range resources {
			cluster := graph.Subgraph(resourceType, dot.ClusterOption{})

			for _, resource := range resourceList {
				cluster.Node(resourceID(resource)).
					Attr("shape", "note").
					Attr("fillcolor", "azure2").
					Attr("style", "filled")
			}
		}

		for _, msg := range resp.GetMessages() {
			for _, edge := range msg.GetEdges() {
				for _, resource := range resources[resourceTypeID(edge)] {
					if edge.GetResourceId() != "" && resource.Metadata().ID() != edge.GetResourceId() {
						continue
					}

					if (edge.GetEdgeType() == inspect.DependencyEdgeType_OUTPUT_EXCLUSIVE ||
						edge.GetEdgeType() == inspect.DependencyEdgeType_OUTPUT_SHARED) &&
						edge.GetControllerName() != resource.Metadata().Owner() {
						continue
					}

					switch edge.GetEdgeType() {
					case inspect.DependencyEdgeType_OUTPUT_EXCLUSIVE:
						graph.Edge(graph.Node(edge.ControllerName), graph.Subgraph(resourceTypeID(edge)).Node(resourceID(resource))).Solid()
					case inspect.DependencyEdgeType_OUTPUT_SHARED:
						graph.Edge(graph.Node(edge.ControllerName), graph.Subgraph(resourceTypeID(edge)).Node(resourceID(resource))).Solid()
					case inspect.DependencyEdgeType_INPUT_STRONG:
						graph.Edge(graph.Subgraph(resourceTypeID(edge)).Node(resourceID(resource)), graph.Node(edge.ControllerName)).Solid()
					case inspect.DependencyEdgeType_INPUT_WEAK:
						graph.Edge(graph.Subgraph(resourceTypeID(edge)).Node(resourceID(resource)), graph.Node(edge.ControllerName)).Dotted()
					case inspect.DependencyEdgeType_INPUT_DESTROY_READY: // don't show the DestroyReady inputs to reduce the visual clutter
					}
				}
			}
		}
	} else {
		for _, msg := range resp.GetMessages() {
			for _, edge := range msg.GetEdges() {
				graph.Node(edge.ControllerName).Box()

				graph.Node(resourceTypeID(edge)).
					Attr("shape", "note").
					Attr("fillcolor", "azure2").
					Attr("style", "filled")
			}
		}

		for _, msg := range resp.GetMessages() {
			for _, edge := range msg.GetEdges() {
				var idLabels []string

				if edge.GetResourceId() != "" {
					idLabels = append(idLabels, edge.GetResourceId())
				}

				switch edge.GetEdgeType() {
				case inspect.DependencyEdgeType_OUTPUT_EXCLUSIVE:
					graph.Edge(graph.Node(edge.ControllerName), graph.Node(resourceTypeID(edge))).Bold()
				case inspect.DependencyEdgeType_OUTPUT_SHARED:
					graph.Edge(graph.Node(edge.ControllerName), graph.Node(resourceTypeID(edge))).Solid()
				case inspect.DependencyEdgeType_INPUT_STRONG:
					graph.Edge(graph.Node(resourceTypeID(edge)), graph.Node(edge.ControllerName), idLabels...).Solid()
				case inspect.DependencyEdgeType_INPUT_WEAK:
					graph.Edge(graph.Node(resourceTypeID(edge)), graph.Node(edge.ControllerName), idLabels...).Dotted()
				case inspect.DependencyEdgeType_INPUT_DESTROY_READY: // don't show the DestroyReady inputs to reduce the visual clutter
				}
			}
		}
	}

	graph.Write(output)

	return nil
}
