// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cli

import (
	"context"
	"fmt"
	"io"
	"math"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/emicklei/dot"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"

	"github.com/talos-systems/talos/pkg/machinery/api/inspect"
	"github.com/talos-systems/talos/pkg/machinery/api/machine"
	"github.com/talos-systems/talos/pkg/machinery/client"
)

// RenderMounts renders mounts output.
func RenderMounts(resp *machine.MountsResponse, output io.Writer, remotePeer *peer.Peer) error {
	w := tabwriter.NewWriter(output, 0, 0, 3, ' ', 0)
	parts := []string{"FILESYSTEM", "SIZE(GB)", "USED(GB)", "AVAILABLE(GB)", "PERCENT USED", "MOUNTED ON"}

	var defaultNode string

	if remotePeer != nil {
		parts = append([]string{"NODE"}, parts...)
		defaultNode = client.AddrFromPeer(remotePeer)
	}

	fmt.Fprintln(w, strings.Join(parts, "\t"))

	for _, msg := range resp.Messages {
		for _, r := range msg.Stats {
			percentAvailable := 100.0 - 100.0*(float64(r.Available)/float64(r.Size))

			if math.IsNaN(percentAvailable) {
				continue
			}

			node := defaultNode

			if msg.Metadata != nil {
				node = msg.Metadata.Hostname
			}

			format := "%s\t%.02f\t%.02f\t%.02f\t%.02f%%\t%s\n"
			args := []interface{}{r.Filesystem, float64(r.Size) * 1e-9, float64(r.Size-r.Available) * 1e-9, float64(r.Available) * 1e-9, percentAvailable, r.MountedOn}

			if defaultNode != "" {
				format = "%s\t" + format

				args = append([]interface{}{node}, args...)
			}

			fmt.Fprintf(w, format, args...)
		}
	}

	return w.Flush()
}

// RenderGraph renders inspect controller runtime graph.
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

				listClient, err := c.Resources.List(ctx, edge.GetResourceNamespace(), edge.GetResourceType())
				if err != nil {
					return fmt.Errorf("error listing resources: %w", err)
				}

				for {
					resp, err := listClient.Recv()
					if err != nil {
						if err == io.EOF || client.StatusCode(err) == codes.Canceled {
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
				case inspect.DependencyEdgeType_INPUT_DESTROY_READY:
					// don't show the DestroyReady inputs to reduce the visual clutter
				}
			}
		}
	}

	graph.Write(output)

	return nil
}

// RenderServicesInfo writes human readable service information to the io.Writer.
func RenderServicesInfo(services []client.ServiceInfo, output io.Writer, defaultNode string, withNodeInfo bool) error {
	w := tabwriter.NewWriter(output, 0, 0, 3, ' ', 0)

	node := defaultNode

	for _, s := range services {
		if s.Metadata != nil {
			node = s.Metadata.Hostname
		}

		if withNodeInfo {
			fmt.Fprintf(w, "NODE\t%s\n", node)
		}

		svc := ServiceInfoWrapper{s.Service}
		fmt.Fprintf(w, "ID\t%s\n", svc.Id)
		fmt.Fprintf(w, "STATE\t%s\n", svc.State)
		fmt.Fprintf(w, "HEALTH\t%s\n", svc.HealthStatus())

		if svc.Health.LastMessage != "" {
			fmt.Fprintf(w, "LAST HEALTH MESSAGE\t%s\n", svc.Health.LastMessage)
		}

		label := "EVENTS"

		for i := range svc.Events.Events {
			event := svc.Events.Events[len(svc.Events.Events)-1-i]

			ts := event.Ts.AsTime()
			fmt.Fprintf(w, "%s\t[%s]: %s (%s ago)\n", label, event.State, event.Msg, time.Since(ts).Round(time.Second))
			label = ""
		}
	}

	return w.Flush()
}

// ServiceInfoWrapper helper that allows generating rich service information.
type ServiceInfoWrapper struct {
	*machine.ServiceInfo
}

// LastUpdated derive last updated time from events stream.
func (svc ServiceInfoWrapper) LastUpdated() string {
	if len(svc.Events.Events) == 0 {
		return ""
	}

	ts := svc.Events.Events[len(svc.Events.Events)-1].Ts.AsTime()

	return time.Since(ts).Round(time.Second).String()
}

// LastEvent return last service event.
func (svc ServiceInfoWrapper) LastEvent() string {
	if len(svc.Events.Events) == 0 {
		return "<none>"
	}

	return svc.Events.Events[len(svc.Events.Events)-1].Msg
}

// HealthStatus service health status.
func (svc ServiceInfoWrapper) HealthStatus() string {
	if svc.Health.Unknown {
		return "?"
	}

	if svc.Health.Healthy {
		return "OK"
	}

	return "Fail"
}
