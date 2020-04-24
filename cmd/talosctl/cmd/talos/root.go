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
	"github.com/talos-systems/talos/pkg/grpc/tls"
	"github.com/talos-systems/talos/pkg/universe"
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
		configContext, creds, err := client.NewClientContextAndCredentialsFromConfig(Talosconfig, Cmdcontext)
		if err != nil {
			return fmt.Errorf("error getting client credentials: %w", err)
		}

		configEndpoints := configContext.Endpoints

		if len(Endpoints) > 0 {
			// override endpoints from command-line flags
			configEndpoints = Endpoints
		}

		targetNodes := configContext.Nodes

		if len(Nodes) > 0 {
			targetNodes = Nodes
		}

		// Update context with grpc metadata for proxy/relay requests
		ctx = client.WithNodes(ctx, targetNodes...)

		tlsconfig, err := tls.New(
			tls.WithKeypair(creds.Crt),
			tls.WithClientAuthType(tls.Mutual),
			tls.WithCACertPEM(creds.CA),
		)
		if err != nil {
			return err
		}

		c, err := client.NewClient(tlsconfig, configEndpoints, universe.ApidPort)
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
