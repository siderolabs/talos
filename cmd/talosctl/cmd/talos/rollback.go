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
	"github.com/siderolabs/talos/pkg/flags"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/client/multiplex"
	"github.com/siderolabs/talos/pkg/reporter"
)

var rollbackCmdFlags = struct {
	trackableActionCmdFlags

	progress flags.PflagExtended[reporter.OutputMode]
}{
	progress: reporter.NewOutputModeFlag(),
}

// rollbackCmd represents the rollback command.
var rollbackCmd = &cobra.Command{
	Use:   "rollback",
	Short: "Rollback a node to the previous installation",
	Long:  ``,
	RunE: func(cmd *cobra.Command, args []string) error {
		if rollbackCmdFlags.debug {
			rollbackCmdFlags.wait = true
		}

		ctx := cmd.Context()

		clientFactory, err := NewClientFactory(ctx, &rollbackCmdFlags, action.GRPCDialOptions()...)
		if err != nil {
			return err
		}

		defer clientFactory.Close() //nolint:errcheck

		if !rollbackCmdFlags.wait {
			if err := helpers.ClientVersionCheck(ctx, clientFactory); err != nil {
				return err
			}

			responseChan := multiplex.UnaryViaFactory(
				ctx, clientFactory,
				func(ctx context.Context, c *client.Client) (struct{}, error) {
					return struct{}{}, c.Rollback(ctx)
				},
			)

			var errs error

			for resp := range responseChan {
				if resp.Err != nil {
					errs = errors.Join(errs, fmt.Errorf("error executing rollback on node %s: %w", resp.Node, resp.Err))
				}
			}

			return errs
		}

		rep := reporter.New(
			reporter.WithOutputMode(rollbackCmdFlags.progress.Value()),
		)

		return action.NewTracker(
			clientFactory,
			action.MachineReadyEventFn,
			rollbackGetActorID,
			action.WithPostCheck(action.BootIDChangedPostCheckFn),
			action.WithDebug(rollbackCmdFlags.debug),
			action.WithTimeout(rollbackCmdFlags.timeout),
			action.WithReporter(rep),
		).Run(ctx)
	},
}

func rollbackGetActorID(ctx context.Context, c *client.Client) (string, error) {
	resp, err := c.RollbackWithResponse(ctx)
	if err != nil {
		return "", err
	}

	if len(resp.GetMessages()) == 0 {
		return "", errors.New("no messages returned from action run")
	}

	return resp.GetMessages()[0].GetActorId(), nil
}

func init() {
	rollbackCmd.Flags().Var(rollbackCmdFlags.progress, "progress", fmt.Sprintf("output mode for rollback progress. Values: %v", rollbackCmdFlags.progress.Options()))
	rollbackCmdFlags.addTrackActionFlags(rollbackCmd)
	addCommand(rollbackCmd)
}
