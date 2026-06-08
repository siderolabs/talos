// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/siderolabs/gen/maps"
	"github.com/spf13/cobra"

	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/action"
	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/helpers"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/client/multiplex"
)

var wipeOptions = map[string]machineapi.ResetRequest_WipeMode{
	wipeModeAll:        machineapi.ResetRequest_ALL,
	wipeModeSystemDisk: machineapi.ResetRequest_SYSTEM_DISK,
	wipeModeUserDisks:  machineapi.ResetRequest_USER_DISKS,
}

// WipeMode apply, patch, edit config update mode.
type WipeMode machineapi.ResetRequest_WipeMode

const (
	wipeModeAll        = "all"
	wipeModeSystemDisk = "system-disk"
	wipeModeUserDisks  = "user-disks"
)

func (m WipeMode) String() string {
	switch machineapi.ResetRequest_WipeMode(m) {
	case machineapi.ResetRequest_ALL:
		return wipeModeAll
	case machineapi.ResetRequest_SYSTEM_DISK:
		return wipeModeSystemDisk
	case machineapi.ResetRequest_USER_DISKS:
		return wipeModeUserDisks
	}

	return wipeModeAll
}

// Set implements Flag interface.
func (m *WipeMode) Set(value string) error {
	mode, ok := wipeOptions[value]
	if !ok {
		return fmt.Errorf("possible options are: %s", m.Type())
	}

	*m = WipeMode(mode)

	return nil
}

// Type implements Flag interface.
func (m *WipeMode) Type() string {
	options := maps.Keys(wipeOptions)
	slices.Sort(options)

	return strings.Join(options, ", ")
}

var resetCmdFlags struct {
	trackableActionCmdFlags

	graceful           bool
	reboot             bool
	wipeMode           WipeMode
	userDisksToWipe    []string
	systemLabelsToWipe []string
}

// resetCmd represents the reset command.
var resetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Reset a node",
	Long:  ``,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if resetCmdFlags.debug {
			resetCmdFlags.wait = true
		}

		resetRequest := buildResetRequest()

		ctx := cmd.Context()

		clientFactory, err := NewClientFactory(ctx, &resetCmdFlags, action.GRPCDialOptions()...)
		if err != nil {
			return err
		}

		defer clientFactory.Close() //nolint:errcheck

		if !resetCmdFlags.wait {
			if err := helpers.ClientVersionCheck(ctx, clientFactory); err != nil {
				return err
			}

			responseChan := multiplex.UnaryViaFactory(
				ctx, clientFactory,
				func(ctx context.Context, c *client.Client) (struct{}, error) {
					return struct{}{}, c.ResetGeneric(ctx, resetRequest)
				},
			)

			var errs error

			for resp := range responseChan {
				if resp.Err != nil {
					errs = errors.Join(errs, fmt.Errorf("error executing reset on node %s: %w", resp.Node, resp.Err))
				}
			}

			return errs
		}

		actionFn := func(ctx context.Context, c *client.Client) (string, error) {
			return resetGetActorID(ctx, c, resetRequest)
		}

		var postCheckFn func(context.Context, *client.Client, string, string) error

		if resetCmdFlags.reboot {
			postCheckFn = func(ctx context.Context, c *client.Client, node, preActionBootID string) error {
				maintenanceCli, err := client.New(ctx,
					client.WithDefaultGRPCDialOptions(),
					client.WithMaintenanceMode(node, nil),
				)
				if err != nil {
					return err
				}

				defer maintenanceCli.Close() //nolint:errcheck

				_, err = maintenanceCli.Disks(ctx)

				// if we can get into maintenance mode, reset has succeeded
				if err == nil {
					return nil
				}

				// try to get the boot ID in the normal mode to see if the node has rebooted
				return action.BootIDChangedPostCheckFn(ctx, c, node, preActionBootID)
			}
		}

		return action.NewTracker(
			clientFactory,
			action.StopAllServicesEventFn,
			actionFn,
			action.WithPostCheck(postCheckFn),
			action.WithDebug(resetCmdFlags.debug),
			action.WithTimeout(resetCmdFlags.timeout),
		).Run(cmd.Context())
	},
}

func buildResetRequest() *machineapi.ResetRequest {
	systemPartitionsToWipe := make([]*machineapi.ResetPartitionSpec, 0, len(resetCmdFlags.systemLabelsToWipe))

	for _, label := range resetCmdFlags.systemLabelsToWipe {
		systemPartitionsToWipe = append(systemPartitionsToWipe, &machineapi.ResetPartitionSpec{
			Label: label,
			Wipe:  true,
		})
	}

	return &machineapi.ResetRequest{
		Graceful:               resetCmdFlags.graceful,
		Reboot:                 resetCmdFlags.reboot,
		UserDisksToWipe:        resetCmdFlags.userDisksToWipe,
		Mode:                   machineapi.ResetRequest_WipeMode(resetCmdFlags.wipeMode),
		SystemPartitionsToWipe: systemPartitionsToWipe,
	}
}

func resetGetActorID(ctx context.Context, c *client.Client, req *machineapi.ResetRequest) (string, error) {
	resp, err := c.ResetGenericWithResponse(ctx, req)
	if err != nil {
		return "", err
	}

	if len(resp.GetMessages()) == 0 {
		return "", errors.New("no messages returned from action run")
	}

	return resp.GetMessages()[0].GetActorId(), nil
}

func init() {
	resetCmd.Flags().BoolVar(&resetCmdFlags.graceful, "graceful", true, "if true, attempt to cordon/drain node and leave etcd (if applicable)")
	resetCmd.Flags().BoolVar(&resetCmdFlags.reboot, "reboot", false, "if true, reboot the node after resetting instead of shutting down")
	resetCmd.Flags().Var(&resetCmdFlags.wipeMode, "wipe-mode", "disk reset mode")
	resetCmd.Flags().StringSliceVar(&resetCmdFlags.userDisksToWipe, "user-disks-to-wipe", nil, "if set, wipes defined devices in the list")
	resetCmd.Flags().StringSliceVar(&resetCmdFlags.systemLabelsToWipe, "system-labels-to-wipe", nil, "if set, just wipe selected system disk partitions by label but keep other partitions intact")
	resetCmdFlags.addTrackActionFlags(resetCmd)
	addCommand(resetCmd)
}
