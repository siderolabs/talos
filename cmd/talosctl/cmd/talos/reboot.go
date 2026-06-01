// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/action"
	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/helpers"
	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/nodedrain"
	"github.com/siderolabs/talos/pkg/flags"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/reporter"
)

var rebootCmdFlags = struct {
	trackableActionCmdFlags

	progress     flags.PflagExtended[reporter.OutputMode]
	rebootMode   flags.PflagExtended[machine.RebootRequest_Mode]
	drain        bool
	drainTimeout time.Duration
}{
	rebootMode: flags.ProtoEnum(machine.RebootRequest_DEFAULT, machine.RebootRequest_Mode_value, machine.RebootRequest_Mode_name),
	progress:   reporter.NewOutputModeFlag(),
}

// rebootCmd represents the reboot command.
var rebootCmd = &cobra.Command{
	Use:   "reboot",
	Short: "Reboot a node",
	Long:  ``,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if rebootCmdFlags.debug {
			rebootCmdFlags.wait = true
		}

		if rebootCmdFlags.drain {
			rebootCmdFlags.wait = true
		}

		opts := []client.RebootMode{
			client.WithRebootMode(rebootCmdFlags.rebootMode.Value()),
		}

		return rebootRun(cmd.Context(), opts)
	},
}

func rebootRun(ctx context.Context, opts []client.RebootMode) (retErr error) {
	rep := reporter.New(
		reporter.WithOutputMode(rebootCmdFlags.progress.Value()),
	)

	if !rebootCmdFlags.drain {
		return rebootInternal(ctx, rebootCmdFlags.wait, rebootCmdFlags.debug, rebootCmdFlags.timeout, rep, opts...)
	}

	var nodeNames map[string]string

	if err := WithClientAndNodes(ctx, func(ctx context.Context, c *client.Client, nodes []string) error {
		var drainErr error

		nodeNames, drainErr = drainNodes(ctx, c, nodes, rebootCmdFlags.drainTimeout, rep)

		return drainErr
	}); err != nil {
		return err
	}

	defer func() {
		if uncordonErr := WithClientAndNodes(ctx, func(ctx context.Context, c *client.Client, _ []string) error {
			return uncordonNodes(ctx, c, nodeNames, rebootCmdFlags.timeout, rep)
		}); uncordonErr != nil {
			retErr = errors.Join(retErr, uncordonErr)
		}
	}()

	return rebootInternal(ctx, rebootCmdFlags.wait, rebootCmdFlags.debug, rebootCmdFlags.timeout, rep, opts...)
}

func rebootInternal(ctx context.Context, wait, debug bool, timeout time.Duration, rep *reporter.Reporter, opts ...client.RebootMode) error {
	if !wait {
		return WithClient(ctx, func(ctx context.Context, c *client.Client) error {
			if err := helpers.ClientVersionCheck(ctx, c); err != nil {
				return err
			}

			if err := c.Reboot(ctx, opts...); err != nil {
				return fmt.Errorf("error executing reboot: %s", err)
			}

			return nil
		})
	}

	return action.NewTracker(
		&GlobalArgs,
		action.MachineReadyEventFn,
		rebootGetActorID(opts...),
		action.WithPostCheck(action.BootIDChangedPostCheckFn),
		action.WithDebug(debug),
		action.WithTimeout(timeout),
		action.WithReporter(rep),
	).Run(ctx)
}

func rebootGetActorID(opts ...client.RebootMode) func(ctx context.Context, c *client.Client) (string, error) {
	return func(ctx context.Context, c *client.Client) (string, error) {
		resp, err := c.RebootWithResponse(ctx, opts...)
		if err != nil {
			return "", err
		}

		if len(resp.GetMessages()) == 0 {
			return "", errors.New("no messages returned from action run")
		}

		return resp.GetMessages()[0].GetActorId(), nil
	}
}

func init() {
	rebootCmd.Flags().Var(rebootCmdFlags.progress, "progress", fmt.Sprintf("output mode for upgrade progress. Values: %v", rebootCmdFlags.progress.Options()))
	rebootCmd.Flags().VarP(
		rebootCmdFlags.rebootMode, "mode", "m",
		fmt.Sprintf(
			"select the reboot mode during upgrade. Mode %q bypasses kexec. Values: %v",
			strings.ToLower(machine.UpgradeRequest_POWERCYCLE.String()),
			rebootCmdFlags.rebootMode.Options(),
		),
	)
	rebootCmd.Flags().BoolVar(&rebootCmdFlags.drain, "drain", false, "drain the Kubernetes node before rebooting (cordon + evict pods)")
	rebootCmd.Flags().DurationVar(&rebootCmdFlags.drainTimeout, "drain-timeout", nodedrain.DefaultDrainTimeout, "timeout for draining the Kubernetes node")
	rebootCmdFlags.addTrackActionFlags(rebootCmd)
	addCommand(rebootCmd)
}
