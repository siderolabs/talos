// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/talos-systems/talos/pkg/cli"
	"github.com/talos-systems/talos/pkg/machinery/client"
	"github.com/talos-systems/talos/pkg/machinery/client/config"
)

var kubernetes bool

// Common options set on root command.
var (
	Talosconfig string
	Endpoints   []string
	Nodes       []string
	Cmdcontext  string
)

// WithClientNoNodes wraps common code to initialize Talos client and provide cancellable context.
//
// WithClientNoNodes doesn't set any node information on request context.
func WithClientNoNodes(action func(context.Context, *client.Client) error) error {
	return cli.WithContext(context.Background(), func(ctx context.Context) error {
		cfg, err := config.Open(Talosconfig)
		if err != nil {
			return fmt.Errorf("failed to open config file %q: %w", Talosconfig, err)
		}

		opts := []client.OptionFunc{
			client.WithConfig(cfg),
		}

		if Cmdcontext != "" {
			opts = append(opts, client.WithContextName(Cmdcontext))
		}

		if len(Endpoints) > 0 {
			// override endpoints from command-line flags
			opts = append(opts, client.WithEndpoints(Endpoints...))
		}

		c, err := client.New(ctx, opts...)
		if err != nil {
			return fmt.Errorf("error constructing client: %w", err)
		}
		//nolint:errcheck
		defer c.Close()

		return action(ctx, c)
	})
}

// WithClient builds upon WithClientNoNodes to provide set of nodes on request context based on config & flags.
func WithClient(action func(context.Context, *client.Client) error) error {
	return WithClientNoNodes(func(ctx context.Context, c *client.Client) error {
		if len(Nodes) < 1 {
			configContext := c.GetConfigContext()
			if configContext == nil {
				return fmt.Errorf("failed to resolve config context")
			}

			Nodes = configContext.Nodes
		}

		if len(Nodes) < 1 {
			return fmt.Errorf("nodes are not set for the command: please use `--nodes` flag or configuration file to set the nodes to run the command against")
		}

		ctx = client.WithNodes(ctx, Nodes...)

		return action(ctx, c)
	})
}

// Commands is a list of commands published by the package.
var Commands []*cobra.Command

func addCommand(cmd *cobra.Command) {
	Commands = append(Commands, cmd)
}
