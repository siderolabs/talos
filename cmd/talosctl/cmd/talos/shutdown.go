// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/talos-systems/talos/cmd/talosctl/pkg/talos/action"
	"github.com/talos-systems/talos/cmd/talosctl/pkg/talos/helpers"
	"github.com/talos-systems/talos/pkg/machinery/client"
)

var shutdownCmdFlags struct {
	force bool
	wait  bool
	debug bool
}

// shutdownCmd represents the shutdown command.
var shutdownCmd = &cobra.Command{
	Use:   "shutdown",
	Short: "Shutdown a node",
	Long:  ``,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if shutdownCmdFlags.debug {
			shutdownCmdFlags.wait = true
		}

		opts := []client.ShutdownOption{
			client.WithShutdownForce(shutdownCmdFlags.force),
		}

		if !shutdownCmdFlags.wait {
			return WithClient(func(ctx context.Context, c *client.Client) error {
				if err := helpers.ClientVersionCheck(ctx, c); err != nil {
					return err
				}

				if err := c.Shutdown(ctx, opts...); err != nil {
					return fmt.Errorf("error executing shutdown: %s", err)
				}

				return nil
			})
		}

		cmd.SilenceErrors = true

		return action.NewTracker(&GlobalArgs, action.StopAllServicesEventFn, shutdownGetActorID, nil, shutdownCmdFlags.debug).Run()
	},
}

func shutdownGetActorID(ctx context.Context, c *client.Client) (string, error) {
	resp, err := c.ShutdownWithResponse(ctx, client.WithShutdownForce(shutdownCmdFlags.force))
	if err != nil {
		return "", err
	}

	if len(resp.GetMessages()) == 0 {
		return "", fmt.Errorf("no messages returned from action run")
	}

	return resp.GetMessages()[0].GetActorId(), nil
}

func init() {
	shutdownCmd.Flags().BoolVar(&shutdownCmdFlags.force, "force", false, "if true, force a node to shutdown without a cordon/drain")
	shutdownCmd.Flags().BoolVar(&shutdownCmdFlags.wait, "wait", false, "wait for the operation to complete, tracking its progress. always set to true when --debug is set")
	shutdownCmd.Flags().BoolVar(&shutdownCmdFlags.debug, "debug", false, "debug operation from kernel logs. --no-wait is set to false when this flag is set")
	addCommand(shutdownCmd)
}
