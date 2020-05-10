// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/talos-systems/talos/pkg/cli"
	"github.com/talos-systems/talos/pkg/client"
	"github.com/talos-systems/talos/pkg/client/config"
)

var (
	kubernetes bool
	useCRI     bool
)

// Common options set on root command
var (
	Talosconfig string
	Endpoints   []string
	Nodes       []string
	Cmdcontext  string
)

// WithClient wraps common code to initialize Talos client and provide cancellable context.
func WithClient(action func(context.Context, *client.Client) error) error {
	return cli.WithContext(context.Background(), func(ctx context.Context) error {
		cfg, err := config.Open(Talosconfig)
		if err != nil {
			return fmt.Errorf("failed to open config file %q: %w", Talosconfig, err)
		}

		configContext := cfg.GetContext(Cmdcontext)
		if configContext == nil {
			return fmt.Errorf("context %q does not exist in config file %q: %w", Cmdcontext, Talosconfig, err)
		}

		opts := []client.OptionFunc{
			client.WithConfigContext(configContext),
		}

		if len(Endpoints) > 0 {
			// override endpoints from command-line flags
			opts = append(opts, client.WithEndpoints(Endpoints...))
		}

		if len(Nodes) < 1 {
			Nodes = configContext.Nodes
		}

		ctx = client.WithNodes(ctx, Nodes...)

		c, err := client.New(ctx, opts...)
		if err != nil {
			return fmt.Errorf("error constructing client: %w", err)
		}
		// nolint: errcheck
		defer c.Close()

		return action(ctx, c)
	})
}

// Commands is a list of commands published by the package
var Commands []*cobra.Command

func addCommand(cmd *cobra.Command) {
	Commands = append(Commands, cmd)
}
