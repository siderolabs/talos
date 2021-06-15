// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	machineapi "github.com/talos-systems/talos/pkg/machinery/api/machine"
	"github.com/talos-systems/talos/pkg/machinery/client"
	clientconfig "github.com/talos-systems/talos/pkg/machinery/client/config"
	"github.com/talos-systems/talos/pkg/machinery/role"
)

// talosconfigCmdFlags represents the talosconfig command flags.
var talosconfigCmdFlags struct {
	roles []string
}

// talosconfigCmd represents the talosconfig command.
var talosconfigCmd = &cobra.Command{
	Use:   "talosconfig [<path>]",
	Short: "Generate a new client configuration file",
	Long:  `TODO`,
	Args:  cobra.RangeArgs(0, 1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			args = []string{"talosconfig"}
		}

		path := args[0]

		if len(Nodes) > 1 {
			return fmt.Errorf("at most one node can be specified, got %d", len(Nodes))
		}

		return WithClient(func(ctx context.Context, c *client.Client) error {
			roles, err := role.Parse(talosconfigCmdFlags.roles)
			if err != nil {
				return err
			}

			resp, err := c.GenerateClientConfiguration(ctx, &machineapi.GenerateClientConfigurationRequest{
				Roles: roles.Strings(),
			})
			if err != nil {
				return err
			}

			if len(resp.Messages) != 1 {
				panic("oops")
			}

			newConfig, err := clientconfig.FromString(resp.Messages[0].Talosconfig)
			if err != nil {
				return err
			}

			config, err := clientconfig.Open(path)
			if err != nil {
				return err
			}

			config.Merge(newConfig)

			return config.Save(path)
		})
	},
}

func init() {
	talosconfigCmd.Flags().StringSliceVar(&talosconfigCmdFlags.roles, "roles", role.MakeSet(role.Admin).Strings(), "roles")

	addCommand(talosconfigCmd)
}
