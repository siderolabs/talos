// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/emicklei/dot"
	"github.com/spf13/cobra"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/talos-systems/talos/cmd/talosctl/pkg/talos/helpers"
	"github.com/talos-systems/talos/pkg/cli"
	"github.com/talos-systems/talos/pkg/machinery/api/inspect"
	"github.com/talos-systems/talos/pkg/machinery/client"
)

// inspectCmd represents the inspect command.
var inspectCmd = &cobra.Command{
	Use:   "inspect",
	Short: "Inspect internals of Talos",
	Long:  ``,
}

var inspectDependenciesCmdFlags struct {
	withResources bool
}

// inspectDependenciesCmd represents the inspect dependencies command.
var inspectDependenciesCmd = &cobra.Command{
	Use:   "dependencies",
	Short: "Inspect controller-resource dependencies as graphviz graph.",
	Long: `Inspect controller-resource dependencies as graphviz graph.

Pipe the output of the command through the "dot" program (part of graphviz package)
to render the graph:

  talosctl inspect dependencies | dot -Tpng > graph.png
`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return WithClient(func(ctx context.Context, c *client.Client) error {
			if err := helpers.FailIfMultiNodes(ctx, "inspect dependencies"); err != nil {
				return err
			}

			resp, err := c.Inspect.ControllerRuntimeDependencies(ctx)
			if err != nil {
				if resp == nil {
					return fmt.Errorf("error getting controller runtime dependencies: %s", err)
				}

				cli.Warning("%s", err)
			}

			graph := dot.NewGraph(dot.Directed)

			resourceTypeID := func(edge *inspect.ControllerDependencyEdge) string {
				return edge.GetResourceType()
			}

			resourceID := func(r resource.Resource) string {
				return fmt.Sprintf("%s/%s/%s", r.Metadata().Namespace(), r.Metadata().Type(), r.Metadata().ID())
			}

			if inspectDependenciesCmdFlags.withResources {
				resources := map[string][]resource.Resource{}

				for _, msg := range resp.GetMessages() {
					for _, edge := range msg.GetEdges() {
						resourceType := resourceTypeID(edge)

						if _, ok := resources[resourceType]; ok {
							continue
						}

						listClient, err := c.Resources.List(ctx, edge.GetResourceNamespace(), edge.GetResourceType())
						if err != nil {
							return fmt.Errorf("error listing resources: %w", err)
						}

						for {
							resp, err := listClient.Recv()
							if err != nil {
								if err == io.EOF || status.Code(err) == codes.Canceled {
									break
								}

								return fmt.Errorf("error listing resources: %w", err)
							}

							if resp.Resource != nil {
								resources[resourceType] = append(resources[resourceType], resp.Resource)
							}
						}
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
						idLabels := []string{}

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
						}
					}
				}
			}

			graph.Write(os.Stdout)

			return nil
		})
	},
}

func init() {
	addCommand(inspectCmd)

	inspectCmd.AddCommand(inspectDependenciesCmd)
	inspectDependenciesCmd.Flags().BoolVar(&inspectDependenciesCmdFlags.withResources, "with-resources", false, "display live resource information with dependencies")
}
