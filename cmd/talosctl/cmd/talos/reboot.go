// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/talos-systems/talos/pkg/machinery/client"
)

// rebootCmd represents the reboot command.
var rebootCmd = &cobra.Command{
	Use:   "reboot",
	Short: "Reboot a node",
	Long:  ``,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return WithClient(func(ctx context.Context, c *client.Client) error {
			mode, err := cmd.Flags().GetInt("mode")
			if err != nil {
				return fmt.Errorf("error getting input value for --mode flag: %s", err)
			}

			switch mode {
			case 1:
				if err := c.Reboot(ctx, client.WithPowerCycle); err != nil {
					return fmt.Errorf("error executing reboot in powercycle mode: %s", err)
				}
			default:
				if err := c.Reboot(ctx); err != nil {
					return fmt.Errorf("error executing reboot: %s", err)
				}
			}

			return nil
		})
	},
}

func init() {
	rebootCmd.Flags().IntP("mode", "m", 0, "skips kexec and reboots with power cycle")
	addCommand(rebootCmd)
}
