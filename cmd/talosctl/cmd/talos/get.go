// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"context"
	"errors"
	"fmt"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/spf13/cobra"

	"github.com/siderolabs/talos/cmd/talosctl/cmd/talos/output"
	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/global"
	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/helpers"
	"github.com/siderolabs/talos/pkg/cli"
	"github.com/siderolabs/talos/pkg/machinery/resources/cluster"
)

var getCmdFlags struct {
	global.InsecureFlags

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
			return completeResourceDefinition(cmd.Context(), toComplete != "")
		case 1:
			return completeResourceID(cmd.Context(), args[0], getCmdFlags.namespace)
		}

		return nil, cobra.ShellCompDirectiveError | cobra.ShellCompDirectiveNoFileComp
	},
	Args: cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		clientFactory, err := NewClientFactory(ctx, &getCmdFlags)
		if err != nil {
			return err
		}

		defer clientFactory.Close() //nolint:errcheck

		return getResources(ctx, args, clientFactory)
	},
}

//nolint:gocyclo,cyclop
func getResources(ctx context.Context, args []string, clientFactory *global.ClientFactory) error {
	if err := helpers.ClientVersionCheck(ctx, clientFactory); err != nil { //nolint:staticcheck // to be refactored next
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

	if len(clientFactory.Nodes()) == 0 {
		return nil
	}

	// resolver resource kind
	firstCtx, firstC, err := clientFactory.BuildClient(ctx, clientFactory.Nodes()[0])
	if err != nil {
		return err
	}

	rd, err := firstC.ResolveResourceKind(firstCtx, &getCmdFlags.namespace, resourceType)
	if err != nil {
		return err
	}

	if getCmdFlags.watch { // get -w <type> OR get -w <type> <id>
		return watchResources(ctx, out, clientFactory, rd, resourceID)
	}

	// get <type>
	// get <type> <id>
	return listResources(ctx, out, clientFactory, rd, resourceID)
}

func listResources(ctx context.Context, out output.Writer, clientFactory *global.ClientFactory, rd *meta.ResourceDefinition, resourceID string) error {
	if err := out.WriteHeader(rd, false); err != nil {
		return err
	}

	resourceType := rd.TypedSpec().Type

	var errs error

	for _, node := range clientFactory.Nodes() {
		nodeCtx, nodeClient, err := clientFactory.BuildClient(ctx, node)
		if err != nil {
			errs = errors.Join(errs, fmt.Errorf("error building client for node %s: %w", node, err))

			continue
		}

		if resourceID == "" {
			items, err := nodeClient.COSI.List(
				nodeCtx,
				resource.NewMetadata(getCmdFlags.namespace, resourceType, "", resource.VersionUndefined),
				state.WithListUnmarshalOptions(state.WithSkipProtobufUnmarshal()),
			)
			if err != nil {
				errs = errors.Join(errs, fmt.Errorf("error listing resources on node %s: %w", node, err))

				continue
			}

			for _, item := range items.Items {
				if err = out.WriteResource(node, item, 0); err != nil {
					errs = errors.Join(errs, err)
				}
			}
		} else {
			r, err := nodeClient.COSI.Get(
				nodeCtx,
				resource.NewMetadata(getCmdFlags.namespace, resourceType, resourceID, resource.VersionUndefined),
				state.WithGetUnmarshalOptions(state.WithSkipProtobufUnmarshal()),
			)
			if err != nil {
				errs = errors.Join(errs, fmt.Errorf("error getting resource on node %s: %w", node, err))

				continue
			}

			if err = out.WriteResource(node, r, 0); err != nil {
				errs = errors.Join(errs, err)
			}
		}
	}

	return errs
}

//nolint:gocyclo
func watchResources(ctx context.Context, out output.Writer, clientFactory *global.ClientFactory, rd *meta.ResourceDefinition, resourceID string) error {
	resourceType := rd.TypedSpec().Type

	if err := out.WriteHeader(rd, true); err != nil {
		return err
	}

	aggregatedCh := make(chan nodeAndEvent)

	for _, node := range clientFactory.Nodes() {
		nodeCtx, nodeClient, err := clientFactory.BuildClient(ctx, node)
		if err != nil {
			return fmt.Errorf("error building client for node %s: %w", node, err)
		}

		watchCh := make(chan state.Event)

		if resourceID == "" {
			err = nodeClient.COSI.WatchKind(
				nodeCtx,
				resource.NewMetadata(getCmdFlags.namespace, resourceType, "", resource.VersionUndefined),
				watchCh,
				state.WithBootstrapContents(true),
				state.WithWatchKindUnmarshalOptions(state.WithSkipProtobufUnmarshal()),
			)
		} else {
			err = nodeClient.COSI.Watch(
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

	bootstrapped := resourceID != "" // if we're watching a specific resource, we can consider it bootstrapped immediately, otherwise we need to wait for the bootstrapped event

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

			if err := out.Flush(); err != nil {
				return err
			}

			continue
		}

		if nev.ev.Resource == nil {
			// new event type without resource, skip it
			continue
		}

		if err := out.WriteResource(nev.node, nev.ev.Resource, nev.ev.Type); err != nil {
			return err
		}

		if bootstrapped {
			if err := out.Flush(); err != nil {
				return err
			}
		}
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
func completeResourceDefinition(ctx context.Context, withAliases bool) ([]string, cobra.ShellCompDirective) {
	clientFactory, err := NewClientFactory(ctx, nil)
	if err != nil {
		cobra.CompError(fmt.Sprintf("error creating client factory: %v", err))

		return nil, cobra.ShellCompDirectiveError
	}

	defer clientFactory.Close() //nolint:errcheck

	ctx, c, err := clientFactory.BuildClientFirstNode(ctx)
	if err != nil {
		cobra.CompError(fmt.Sprintf("error building client: %v", err))

		return nil, cobra.ShellCompDirectiveError
	}

	var result []string

	items, err := safe.StateListAll[*meta.ResourceDefinition](ctx, c.COSI)
	if err != nil {
		cobra.CompError(fmt.Sprintf("error listing resource definitions: %v", err))

		return nil, cobra.ShellCompDirectiveError
	}

	for res := range items.All() {
		if withAliases {
			result = append(result, res.TypedSpec().Aliases...)
		}

		result = append(result, res.Metadata().ID())
	}

	return result, cobra.ShellCompDirectiveNoFileComp
}

// completeResourceID represents tab complete options for `get` and `get *` commands.
func completeResourceID(ctx context.Context, resourceType, namespace string) ([]string, cobra.ShellCompDirective) {
	clientFactory, err := NewClientFactory(ctx, nil)
	if err != nil {
		cobra.CompError(fmt.Sprintf("error creating client factory: %v", err))

		return nil, cobra.ShellCompDirectiveError
	}

	defer clientFactory.Close() //nolint:errcheck

	ctx, c, err := clientFactory.BuildClientFirstNode(ctx)
	if err != nil {
		cobra.CompError(fmt.Sprintf("error building client: %v", err))

		return nil, cobra.ShellCompDirectiveError
	}

	var result []string

	rd, err := c.ResolveResourceKind(ctx, &namespace, resourceType)
	if err != nil {
		cobra.CompError(fmt.Sprintf("error resolving resource kind: %v", err))

		return nil, cobra.ShellCompDirectiveError
	}

	items, err := c.COSI.List(ctx, resource.NewMetadata(namespace, rd.TypedSpec().Type, "", resource.VersionUndefined))
	if err != nil {
		cobra.CompError(fmt.Sprintf("error listing resources: %v", err))

		return nil, cobra.ShellCompDirectiveError
	}

	for _, item := range items.Items {
		result = append(result, item.Metadata().ID())
	}

	return result, cobra.ShellCompDirectiveNoFileComp
}

// completeNodes represents tab completion for `--nodes` argument.
func completeNodes(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	ctx := cmd.Context()

	// setup fake nodes list in the args to make NewClientFactory ignore the fact there are no nodes specified
	// (as we are completing the nodes themselves)
	GlobalArgs.Nodes = []string{"fake-node"}

	clientFactory, err := NewClientFactory(ctx, nil)
	if err != nil {
		cobra.CompError(fmt.Sprintf("error creating client factory: %v", err))

		return nil, cobra.ShellCompDirectiveError
	}

	defer clientFactory.Close() //nolint:errcheck

	c, err := clientFactory.BuildRandomEndpointClient(ctx)
	if err != nil {
		cobra.CompError(fmt.Sprintf("error building client: %v", err))

		return nil, cobra.ShellCompDirectiveError
	}

	var nodes []string

	items, err := safe.StateListAll[*cluster.Member](ctx, c.COSI)
	if err != nil {
		cobra.CompError(fmt.Sprintf("error listing cluster members: %v", err))

		return nil, cobra.ShellCompDirectiveError
	}

	for res := range items.All() {
		if hostname := res.TypedSpec().Hostname; hostname != "" {
			nodes = append(nodes, hostname)
		}

		for _, address := range res.TypedSpec().Addresses {
			nodes = append(nodes, address.String())
		}
	}

	return nodes, cobra.ShellCompDirectiveNoFileComp
}

func init() {
	getCmd.Flags().StringVar(&getCmdFlags.namespace, "namespace", "", "resource namespace (default is to use default namespace per resource)")
	getCmd.Flags().StringVarP(&getCmdFlags.output, "output", "o", "table", "output mode (json, table, yaml, jsonpath)")
	getCmd.Flags().BoolVarP(&getCmdFlags.watch, "watch", "w", false, "watch resource changes")
	getCmdFlags.InsecureFlags.AddFlags(getCmd)
	cli.Should(getCmd.RegisterFlagCompletionFunc("output", output.CompleteOutputArg))
	addCommand(getCmd)
}
