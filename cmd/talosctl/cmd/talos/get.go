// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/hashicorp/go-multierror"
	"github.com/spf13/cobra"
	"google.golang.org/grpc/metadata"

	"github.com/siderolabs/talos/cmd/talosctl/cmd/talos/output"
	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/helpers"
	"github.com/siderolabs/talos/pkg/cli"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/resources/cluster"
)

var getCmdFlags struct {
	insecure bool

	namespace string
	output    string
	watch     bool
}

// getCmd represents the get (resources) command.
var getCmd = &cobra.Command{
	Use:        "get <type> [<id>]",
	Aliases:    []string{"g"},
	SuggestFor: []string{},
	Short:      "Get a specific resource or list of resources (use 'talosctl get rd' to see all available resource types).",
	Long: `Similar to 'kubectl get', 'talosctl get' returns a set of resources from the OS.
To get a list of all available resource definitions, issue 'talosctl get rd'`,
	Example: "",
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		switch len(args) {
		case 0:
			return completeResourceDefinition(toComplete != "")
		case 1:
			return completeResourceID(args[0], getCmdFlags.namespace)
		}

		return nil, cobra.ShellCompDirectiveError | cobra.ShellCompDirectiveNoFileComp
	},
	Args: cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		if getCmdFlags.insecure {
			return WithClientMaintenance(nil, getResources(args))
		}

		if GlobalArgs.SkipVerify {
			return WithClientSkipVerify(getResources(args))
		}

		return WithClient(getResources(args))
	},
}

//nolint:gocyclo,cyclop
func getResources(args []string) func(ctx context.Context, c *client.Client) error {
	return func(ctx context.Context, c *client.Client) error {
		if err := helpers.ClientVersionCheck(ctx, c); err != nil {
			return err
		}

		out, err := output.NewWriter(getCmdFlags.output)
		if err != nil {
			return err
		}

		resourceType := args[0]

		var resourceID string

		if len(args) == 2 {
			resourceID = args[1]
		}

		defer out.Flush() //nolint:errcheck

		if getCmdFlags.watch { // get -w <type> OR get -w <type> <id>
			md, _ := metadata.FromOutgoingContext(ctx)
			nodes := md.Get("nodes")

			if len(nodes) == 0 {
				// use "current" node
				nodes = []string{""}
			}

			// fetch the RD from the first node (it doesn't matter which one to use, so we'll use the first one)
			rd, err := c.ResolveResourceKind(client.WithNode(ctx, nodes[0]), &getCmdFlags.namespace, resourceType)
			if err != nil {
				return err
			}

			resourceType = rd.TypedSpec().Type

			if err = out.WriteHeader(rd, true); err != nil {
				return err
			}

			aggregatedCh := make(chan nodeAndEvent)

			for _, node := range nodes {
				var nodeCtx context.Context

				if node == "" {
					nodeCtx = ctx
				} else {
					nodeCtx = client.WithNode(ctx, node)
				}

				watchCh := make(chan state.Event)

				if resourceID == "" {
					err = c.COSI.WatchKind(
						nodeCtx,
						resource.NewMetadata(getCmdFlags.namespace, resourceType, "", resource.VersionUndefined),
						watchCh,
						state.WithBootstrapContents(true),
						state.WithWatchKindUnmarshalOptions(state.WithSkipProtobufUnmarshal()),
					)
				} else {
					err = c.COSI.Watch(
						nodeCtx,
						resource.NewMetadata(getCmdFlags.namespace, resourceType, resourceID, resource.VersionUndefined),
						watchCh,
						state.WithWatchUnmarshalOptions(state.WithSkipProtobufUnmarshal()),
					)
				}

				if err != nil {
					return fmt.Errorf("error setting up watch on node %s: %w", node, err)
				}

				go aggregateEvents(ctx, aggregatedCh, watchCh, node)
			}

			var bootstrapped bool

			for {
				var nev nodeAndEvent

				select {
				case nev = <-aggregatedCh:
				case <-ctx.Done():
					return nil
				}

				if nev.ev.Type == state.Errored {
					return fmt.Errorf("error watching resource: %w", nev.ev.Error)
				}

				if nev.ev.Type == state.Bootstrapped {
					bootstrapped = true

					if err = out.Flush(); err != nil {
						return err
					}

					continue
				}

				if nev.ev.Resource == nil {
					// new event type without resource, skip it
					continue
				}

				if err = out.WriteResource(nev.node, nev.ev.Resource, nev.ev.Type); err != nil {
					return err
				}

				if bootstrapped {
					if err = out.Flush(); err != nil {
						return err
					}
				}
			}
		}

		var multiErr *multierror.Error

		// get <type>
		// get <type> <id>
		callbackResource := func(parentCtx context.Context, hostname string, r resource.Resource, callError error) error {
			if callError != nil {
				multiErr = multierror.Append(multiErr, callError)

				return nil
			}

			return out.WriteResource(hostname, r, 0)
		}

		callbackRD := func(definition *meta.ResourceDefinition) error {
			return out.WriteHeader(definition, false)
		}

		helperErr := helpers.ForEachResource(ctx, c, callbackRD, callbackResource, getCmdFlags.namespace, args...)
		if helperErr != nil {
			return helperErr
		}

		return multiErr.ErrorOrNil()
	}
}

type nodeAndEvent struct {
	node string
	ev   state.Event
}

func aggregateEvents(ctx context.Context, outCh chan<- nodeAndEvent, watchCh <-chan state.Event, node string) {
	for {
		select {
		case ev := <-watchCh:
			select {
			case outCh <- nodeAndEvent{node, ev}:
			case <-ctx.Done():
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

// completeResourceDefinition represents tab complete options for `get` and `get *` commands.
func completeResourceDefinition(withAliases bool) ([]string, cobra.ShellCompDirective) {
	var result []string

	if WithClientNoNodes(func(ctx context.Context, c *client.Client) error {
		items, err := safe.StateListAll[*meta.ResourceDefinition](ctx, c.COSI)
		if err != nil {
			return err
		}

		for res := range items.All() {
			if withAliases {
				result = append(result, res.TypedSpec().Aliases...)
			}

			result = append(result, res.Metadata().ID())
		}

		return nil
	}) != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	return result, cobra.ShellCompDirectiveNoFileComp
}

// completeResourceID represents tab complete options for `get` and `get *` commands.
func completeResourceID(resourceType, namespace string) ([]string, cobra.ShellCompDirective) {
	var result []string

	if WithClientNoNodes(func(ctx context.Context, c *client.Client) error {
		if len(GlobalArgs.Nodes) > 0 {
			ctx = client.WithNode(ctx, GlobalArgs.Nodes[0])
		}

		rd, err := c.ResolveResourceKind(ctx, &namespace, resourceType)
		if err != nil {
			return err
		}

		items, err := c.COSI.List(ctx, resource.NewMetadata(namespace, rd.TypedSpec().Type, "", resource.VersionUndefined))
		if err != nil {
			return err
		}

		for _, item := range items.Items {
			result = append(result, item.Metadata().ID())
		}

		return nil
	}) != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	return result, cobra.ShellCompDirectiveNoFileComp
}

// CompleteNodes represents tab completion for `--nodes` argument.
func CompleteNodes(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	var nodes []string

	if WithClientNoNodes(func(ctx context.Context, c *client.Client) error {
		items, err := safe.StateListAll[*cluster.Member](ctx, c.COSI)
		if err != nil {
			return err
		}

		for res := range items.All() {
			if hostname := res.TypedSpec().Hostname; hostname != "" {
				nodes = append(nodes, hostname)
			}

			for _, address := range res.TypedSpec().Addresses {
				nodes = append(nodes, address.String())
			}
		}

		return nil
	}) != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	return nodes, cobra.ShellCompDirectiveNoFileComp
}

func init() {
	getCmd.Flags().StringVar(&getCmdFlags.namespace, "namespace", "", "resource namespace (default is to use default namespace per resource)")
	getCmd.Flags().StringVarP(&getCmdFlags.output, "output", "o", "table", "output mode (json, table, yaml, jsonpath)")
	getCmd.Flags().BoolVarP(&getCmdFlags.watch, "watch", "w", false, "watch resource changes")
	getCmd.Flags().BoolVarP(&getCmdFlags.insecure, "insecure", "i", false, "get resources using the insecure (encrypted with no auth) maintenance service")
	cli.Should(getCmd.RegisterFlagCompletionFunc("output", output.CompleteOutputArg))
	addCommand(getCmd)
}
