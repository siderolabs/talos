// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/helpers"
	"github.com/siderolabs/talos/pkg/cli"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/formatters"
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

			return formatters.RenderGraph(ctx, c, resp, os.Stdout, inspectDependenciesCmdFlags.withResources)
		})
	},
}

func init() {
	addCommand(inspectCmd)

	inspectCmd.AddCommand(inspectDependenciesCmd)
	inspectDependenciesCmd.Flags().BoolVar(&inspectDependenciesCmdFlags.withResources, "with-resources", false, "display live resource information with dependencies")
}
