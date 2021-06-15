// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/talos-systems/talos/cmd/talosctl/pkg/talos/helpers"
	machineapi "github.com/talos-systems/talos/pkg/machinery/api/machine"
	"github.com/talos-systems/talos/pkg/machinery/client"
	clientconfig "github.com/talos-systems/talos/pkg/machinery/client/config"
	"github.com/talos-systems/talos/pkg/machinery/role"
)

// talosconfigCmdFlags represents the talosconfig command flags.
var talosconfigCmdFlags struct {
	roles  []string
	crtTTL time.Duration
}

// talosconfigCmd represents the talosconfig command.
var talosconfigCmd = &cobra.Command{
	Use:   "talosconfig [<path>]",
	Short: "Generate a new client configuration file",
	Args:  cobra.RangeArgs(0, 1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			args = []string{"talosconfig"}
		}

		path := args[0]

		return WithClient(func(ctx context.Context, c *client.Client) error {
			if err := helpers.FailIfMultiNodes(ctx, "talosconfig"); err != nil {
				return err
			}

			roles, unknownRoles := role.Parse(talosconfigCmdFlags.roles)
			if unknownRoles != nil {
				return fmt.Errorf("unknown roles: %s", strings.Join(unknownRoles, ", "))
			}

			if _, err := os.Stat(path); err == nil {
				return fmt.Errorf("talosconfig file already exists: %q", path)
			}

			resp, err := c.GenerateClientConfiguration(ctx, &machineapi.GenerateClientConfigurationRequest{
				Roles:  roles.Strings(),
				CrtTtl: durationpb.New(talosconfigCmdFlags.crtTTL),
			})
			if err != nil {
				return err
			}

			if l := len(resp.Messages); l != 1 {
				panic(fmt.Sprintf("expected 1 message, got %d", l))
			}

			config, err := clientconfig.FromBytes(resp.Messages[0].Talosconfig)
			if err != nil {
				return err
			}

			return config.Save(path)
		})
	},
}

func init() {
	talosconfigCmd.Flags().StringSliceVar(&talosconfigCmdFlags.roles, "roles", role.MakeSet(role.Admin).Strings(), "roles")
	talosconfigCmd.Flags().DurationVar(&talosconfigCmdFlags.crtTTL, "crt-ttl", 87600*time.Hour, "certificate TTL")

	addCommand(talosconfigCmd)
}
