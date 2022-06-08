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

var rebootCmdFlags struct {
	mode string
}

// rebootCmd represents the reboot command.
var rebootCmd = &cobra.Command{
	Use:   "reboot",
	Short: "Reboot a node",
	Long:  ``,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return WithClient(func(ctx context.Context, c *client.Client) error {
			opts := []client.RebootMode{}

			switch rebootCmdFlags.mode {
			// skips kexec and reboots with power cycle
			case "powercycle":
				opts = append(opts, client.WithPowerCycle)
			case "default":
			default:
				return fmt.Errorf("invalid reboot mode: %q", rebootCmdFlags.mode)
			}

			if err := c.Reboot(ctx, opts...); err != nil {
				return fmt.Errorf("error executing reboot: %s", err)
			}

			return nil
		})
	},
}

func init() {
	rebootCmd.Flags().StringVarP(&rebootCmdFlags.mode, "mode", "m", "default", "select the reboot mode: \"default\", \"powercyle\" (skips kexec)")
	addCommand(rebootCmd)
}
