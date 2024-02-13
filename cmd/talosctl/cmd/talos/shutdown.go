// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/action"
	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/helpers"
	"github.com/siderolabs/talos/pkg/machinery/client"
)

var shutdownCmdFlags struct {
	trackableActionCmdFlags
	force bool
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

		return action.NewTracker(
			&GlobalArgs,
			action.StopAllServicesEventFn,
			shutdownGetActorID,
			action.WithDebug(shutdownCmdFlags.debug),
			action.WithTimeout(shutdownCmdFlags.timeout),
		).Run()
	},
}

func shutdownGetActorID(ctx context.Context, c *client.Client) (string, error) {
	resp, err := c.ShutdownWithResponse(ctx, client.WithShutdownForce(shutdownCmdFlags.force))
	if err != nil {
		return "", err
	}

	if len(resp.GetMessages()) == 0 {
		return "", errors.New("no messages returned from action run")
	}

	return resp.GetMessages()[0].GetActorId(), nil
}

func init() {
	shutdownCmd.Flags().BoolVar(&shutdownCmdFlags.force, "force", false, "if true, force a node to shutdown without a cordon/drain")
	shutdownCmdFlags.addTrackActionFlags(shutdownCmd)
	addCommand(shutdownCmd)
}
