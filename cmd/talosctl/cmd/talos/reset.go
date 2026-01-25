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
	insecure           bool
	noConfirm          bool
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
		if !resetCmdFlags.noConfirm {
			prompt := "Are you sure you want to reset the node(s)"

			if len(GlobalArgs.Nodes) > 0 {
				nodes := strings.Join(GlobalArgs.Nodes, ", ")
				prompt = fmt.Sprintf("%s: %s?", prompt, nodes)
			} else {
				prompt += " from the current context?"
			}

			if !helpers.Confirm(prompt) {
				fmt.Println("Abort.")
				return nil
			}
		}

		if resetCmdFlags.debug {
			resetCmdFlags.wait = true
		}

		resetRequest := buildResetRequest()

		if resetCmdFlags.wait && resetCmdFlags.insecure {
			return errors.New("cannot use --wait and --insecure together")
		}

		if !resetCmdFlags.wait {
			resetNoWait := func(ctx context.Context, c *client.Client) error {
				if err := helpers.ClientVersionCheck(ctx, c); err != nil {
					return err
				}

				if err := c.ResetGeneric(ctx, resetRequest); err != nil {
					return fmt.Errorf("error executing reset: %s", err)
				}

				return nil
			}

			if resetCmdFlags.insecure {
				return WithClientMaintenance(nil, resetNoWait)
			}

			return WithClient(resetNoWait)
		}

		actionFn := func(ctx context.Context, c *client.Client) (string, error) {
			return resetGetActorID(ctx, c, resetRequest)
		}

		var postCheckFn func(context.Context, *client.Client, string) error

		if resetCmdFlags.reboot {
			postCheckFn = func(ctx context.Context, c *client.Client, preActionBootID string) error {
				err := WithClientMaintenance(nil,
					func(ctx context.Context, cli *client.Client) error {
						_, err := cli.Disks(ctx)

						return err
					})

				// if we can get into maintenance mode, reset has succeeded
				if err == nil {
					return nil
				}

				// try to get the boot ID in the normal mode to see if the node has rebooted
				return action.BootIDChangedPostCheckFn(ctx, c, preActionBootID)
			}
		}

		return action.NewTracker(
			&GlobalArgs,
			action.StopAllServicesEventFn,
			actionFn,
			action.WithPostCheck(postCheckFn),
			action.WithDebug(resetCmdFlags.debug),
			action.WithTimeout(resetCmdFlags.timeout),
		).Run()
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
	resetCmd.Flags().BoolVar(&resetCmdFlags.insecure, "insecure", false, "reset using the insecure (encrypted with no auth) maintenance service")
	resetCmd.Flags().BoolVarP(&resetCmdFlags.noConfirm, "noconfirm", "y", false, "if set, do not ask for confirmation")
	resetCmd.Flags().Var(&resetCmdFlags.wipeMode, "wipe-mode", "disk reset mode")
	resetCmd.Flags().StringSliceVar(&resetCmdFlags.userDisksToWipe, "user-disks-to-wipe", nil, "if set, wipes defined devices in the list")
	resetCmd.Flags().StringSliceVar(&resetCmdFlags.systemLabelsToWipe, "system-labels-to-wipe", nil, "if set, just wipe selected system disk partitions by label but keep other partitions intact")
	resetCmdFlags.addTrackActionFlags(resetCmd)
	addCommand(resetCmd)
}
