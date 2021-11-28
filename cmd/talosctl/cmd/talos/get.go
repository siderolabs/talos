// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/spf13/cobra"
	"google.golang.org/grpc/codes"
	yaml "gopkg.in/yaml.v3"

	"github.com/talos-systems/talos/cmd/talosctl/cmd/talos/output"
	"github.com/talos-systems/talos/cmd/talosctl/pkg/talos/helpers"
	"github.com/talos-systems/talos/pkg/cli"
	"github.com/talos-systems/talos/pkg/machinery/client"
	"github.com/talos-systems/talos/pkg/machinery/resources/cluster"
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
	Short:      "Get a specific resource or list of resources.",
	Long:       `Similar to 'kubectl get', 'talosctl get' returns a set of resources from the OS.  To get a list of all available resource definitions, issue 'talosctl get rd'`,
	Example:    "",
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		switch len(args) {
		case 0:
			if toComplete == "" {
				return completeResource(meta.ResourceDefinitionType, true, false), cobra.ShellCompDirectiveNoFileComp
			}

			return completeResource(meta.ResourceDefinitionType, true, true), cobra.ShellCompDirectiveNoFileComp
		case 1:

			return completeResource(args[0], false, true), cobra.ShellCompDirectiveNoFileComp | cobra.ShellCompDirectiveNoSpace
		}

		return nil, cobra.ShellCompDirectiveError | cobra.ShellCompDirectiveNoFileComp
	},
	Args: cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		if getCmdFlags.insecure {
			return WithClientMaintenance(nil, getResources(args))
		}

		return WithClient(getResources(args))
	},
}

//nolint:gocyclo,cyclop
func getResources(args []string) func(ctx context.Context, c *client.Client) error {
	return func(ctx context.Context, c *client.Client) error {
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

		var headerWritten bool

		if getCmdFlags.watch { // get -w <type> OR get -w <type> <id>
			watchClient, err := c.Resources.Watch(ctx, getCmdFlags.namespace, resourceType, resourceID)
			if err != nil {
				return err
			}

			for {
				msg, err := watchClient.Recv()
				if err != nil {
					if err == io.EOF || client.StatusCode(err) == codes.Canceled {
						return nil
					}

					return err
				}

				if msg.Metadata.GetError() != "" {
					fmt.Fprintf(os.Stderr, "%s: %s\n", msg.Metadata.GetHostname(), msg.Metadata.GetError())

					continue
				}

				if msg.Definition != nil && !headerWritten {
					if e := out.WriteHeader(msg.Definition, true); e != nil {
						return e
					}

					headerWritten = true
				}

				if msg.Resource != nil {
					if err := out.WriteResource(msg.Metadata.GetHostname(), msg.Resource, msg.EventType); err != nil {
						return err
					}

					if err := out.Flush(); err != nil {
						return err
					}
				}
			}
		}

		// get <type>
		// get <type> <id>
		printOut := func(parentCtx context.Context, msg client.ResourceResponse) error {
			if msg.Definition != nil && !headerWritten {
				if e := out.WriteHeader(msg.Definition, false); e != nil {
					return e
				}

				headerWritten = true
			}

			if msg.Resource != nil {
				if err := out.WriteResource(msg.Metadata.GetHostname(), msg.Resource, 0); err != nil {
					return err
				}
			}

			return nil
		}

		return helpers.ForEachResource(ctx, c, printOut, getCmdFlags.namespace, args...)
	}
}

//nolint:gocyclo
func getResourcesResponse(args []string, clientmsg *[]client.ResourceResponse) func(ctx context.Context, c *client.Client) error {
	return func(ctx context.Context, c *client.Client) error {
		var resourceID string

		resourceType := args[0]
		namespace := getCmdFlags.namespace

		if len(args) > 1 {
			resourceID = args[1]
		}

		if resourceID != "" {
			resp, err := c.Resources.Get(ctx, namespace, resourceType, resourceID)
			if err != nil {
				return err
			}

			for _, msg := range resp {
				if msg.Resource == nil {
					continue
				}

				*clientmsg = append(*clientmsg, msg)
			}
		} else {
			listClient, err := c.Resources.List(ctx, namespace, resourceType)
			if err != nil {
				return err
			}

			for {
				msg, err := listClient.Recv()
				if err != nil {
					if err == io.EOF || client.StatusCode(err) == codes.Canceled {
						return nil
					}

					return err
				}

				if msg.Metadata.GetError() != "" {
					fmt.Fprintf(os.Stderr, "%s: %s\n", msg.Metadata.GetHostname(), msg.Metadata.GetError())

					continue
				}
				if msg.Resource == nil {
					continue
				}
				*clientmsg = append(*clientmsg, msg)
			}
		}

		return nil
	}
}

//nolint:gocyclo
// completeResource represents tab complete options for `get` and `get *` commands.
func completeResource(resourceType string, hasAliasses bool, completeDot bool) []string {
	var (
		resourceResponse []client.ResourceResponse
		resourceOptions  []string
	)

	if WithClient(getResourcesResponse([]string{resourceType}, &resourceResponse)) != nil {
		return nil
	}

	for _, msg := range resourceResponse {
		if completeDot {
			resourceOptions = append(resourceOptions, msg.Resource.Metadata().ID())
		}

		if !hasAliasses {
			continue
		}

		resourceSpec, err := yaml.Marshal(msg.Resource.Spec())
		if err != nil {
			continue
		}

		var resourceSpecRaw map[string]interface{}

		if yaml.Unmarshal(resourceSpec, &resourceSpecRaw) != nil {
			continue
		}

		if aliasSlice, ok := resourceSpecRaw["aliases"].([]interface{}); ok {
			for _, alias := range aliasSlice {
				if !completeDot && strings.Contains(alias.(string), ".") {
					continue
				}

				resourceOptions = append(resourceOptions, alias.(string))
			}
		}
	}

	return resourceOptions
}

// CompleteNodes represents tab completion for `--nodes` argument.
func CompleteNodes(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	var (
		resourceResponse []client.ResourceResponse
		nodes            []string
	)

	if WithClientNoNodes(getResourcesResponse([]string{cluster.MemberType}, &resourceResponse)) != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	for _, msg := range resourceResponse {
		var resourceSpecRaw map[string]interface{}

		resourceSpec, err := yaml.Marshal(msg.Resource.Spec())
		if err != nil {
			continue
		}

		if err = yaml.Unmarshal(resourceSpec, &resourceSpecRaw); err != nil {
			continue
		}

		if hostname, ok := resourceSpecRaw["hostname"].(string); ok {
			nodes = append(nodes, hostname)
		}

		if addressSlice, ok := resourceSpecRaw["addresses"].([]interface{}); ok {
			for _, address := range addressSlice {
				nodes = append(nodes, address.(string))
			}
		}
	}

	return nodes, cobra.ShellCompDirectiveNoFileComp
}

func init() {
	getCmd.Flags().StringVar(&getCmdFlags.namespace, "namespace", "", "resource namespace (default is to use default namespace per resource)")
	getCmd.Flags().StringVarP(&getCmdFlags.output, "output", "o", "table", "output mode (json, table, yaml)")
	getCmd.Flags().BoolVarP(&getCmdFlags.watch, "watch", "w", false, "watch resource changes")
	getCmd.Flags().BoolVarP(&getCmdFlags.insecure, "insecure", "i", false, "get resources using the insecure (encrypted with no auth) maintenance service")
	cli.Should(getCmd.RegisterFlagCompletionFunc("output", output.CompleteOutputArg))
	addCommand(getCmd)
}
