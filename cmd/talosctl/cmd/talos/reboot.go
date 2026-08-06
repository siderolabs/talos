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
	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/global"
	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/helpers"
	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/nodedrain"
	"github.com/siderolabs/talos/pkg/flags"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/client/multiplex"
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
	clientFactory, err := NewClientFactory(ctx, &rebootCmdFlags, action.GRPCDialOptions()...)
	if err != nil {
		return err
	}

	defer clientFactory.Close() //nolint:errcheck

	rep := reporter.New(
		reporter.WithOutputMode(rebootCmdFlags.progress.Value()),
	)

	if !rebootCmdFlags.drain {
		return rebootInternal(ctx, clientFactory, rebootCmdFlags.wait, rebootCmdFlags.debug, rebootCmdFlags.timeout, rep, opts...)
	}

	nodeNames, err := drainNodes(ctx, clientFactory, rebootCmdFlags.drainTimeout, rep)

	// Register the uncordon before checking the error: on a partial drain failure
	// drainNodes still returns the nodes it managed to cordon. Unlike upgrade there
	// is no staged image forcing us onward, so aborting the reboot is fine - but the
	// nodes that were cordoned must still be restored to schedulable rather than left
	// SchedulingDisabled. On the abort path the nodes never rebooted, so the deferred
	// WaitForNodeReady returns immediately and the uncordon proceeds.
	defer func() {
		if len(nodeNames) > 0 {
			if uncordonErr := uncordonNodes(ctx, clientFactory, nodeNames, rebootCmdFlags.timeout, rep); uncordonErr != nil {
				retErr = errors.Join(retErr, uncordonErr)
			}
		}
	}()

	if err != nil {
		return fmt.Errorf("error draining nodes: %w", err)
	}

	return rebootInternal(ctx, clientFactory, rebootCmdFlags.wait, rebootCmdFlags.debug, rebootCmdFlags.timeout, rep, opts...)
}

func rebootInternal(
	ctx context.Context, clientFactory *global.ClientFactory,
	wait, debug bool, timeout time.Duration, rep *reporter.Reporter, opts ...client.RebootMode,
) error {
	if !wait {
		if err := helpers.ClientVersionCheck(ctx, clientFactory); err != nil {
			return err
		}

		responseChan := multiplex.UnaryViaFactory(
			ctx, clientFactory,
			func(ctx context.Context, c *client.Client) (struct{}, error) {
				return struct{}{}, c.Reboot(ctx, opts...)
			},
		)

		var errs error

		for resp := range responseChan {
			if resp.Err != nil {
				errs = errors.Join(errs, fmt.Errorf("error executing reboot on node %s: %w", resp.Node, resp.Err))
			}
		}

		return errs
	}

	return action.NewTracker(
		clientFactory,
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
