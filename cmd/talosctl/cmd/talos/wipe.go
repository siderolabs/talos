// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"context"
	"fmt"

	"github.com/siderolabs/gen/xslices"
	"github.com/spf13/cobra"

	"github.com/siderolabs/talos/pkg/machinery/api/storage"
	"github.com/siderolabs/talos/pkg/machinery/client"
)

// wipeCmd represents the wipe command.
var wipeCmd = &cobra.Command{
	Use:   "wipe",
	Short: "Wipe block device or volumes",
	Args:  cobra.NoArgs,
}

var wipeDiskCmdFlags struct {
	wipeMethod         string
	skipVolumeCheck    bool
	skipSecondaryCheck bool
	dropPartition      bool
	insecure           bool
}

// wipeDiskCmd represents the wipe disk command.
var wipeDiskCmd = &cobra.Command{
	Use:   "disk <device names>...",
	Short: "Wipe a block device (disk or partition) which is not used as a volume",
	Long: `Wipe a block device (disk or partition) which is not used as a volume.

Use device names as arguments, for example: vda or sda5.`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if wipeDiskCmdFlags.insecure {
			return WithClientMaintenance(nil, cmdWipe(args))
		}

		if GlobalArgs.SkipVerify {
			return WithClientSkipVerify(cmdWipe(args))
		}

		return WithClient(cmdWipe(args))
	},
}

func cmdWipe(args []string) func(ctx context.Context, c *client.Client) error {
	return func(ctx context.Context, c *client.Client) error {
		method, ok := storage.BlockDeviceWipeDescriptor_Method_value[wipeDiskCmdFlags.wipeMethod]
		if !ok {
			return fmt.Errorf("invalid wipe method %q", wipeDiskCmdFlags.wipeMethod)
		}

		return c.BlockDeviceWipe(ctx, &storage.BlockDeviceWipeRequest{
			Devices: xslices.Map(args, func(devName string) *storage.BlockDeviceWipeDescriptor {
				return &storage.BlockDeviceWipeDescriptor{
					Device:             devName,
					Method:             storage.BlockDeviceWipeDescriptor_Method(method),
					SkipVolumeCheck:    wipeDiskCmdFlags.skipVolumeCheck,
					SkipSecondaryCheck: wipeDiskCmdFlags.skipSecondaryCheck,
					DropPartition:      wipeDiskCmdFlags.dropPartition,
				}
			}),
		})
	}
}

func wipeMethodValues() []string {
	var method storage.BlockDeviceWipeDescriptor_Method

	values := make([]string, method.Descriptor().Values().Len())

	for idx := range method.Descriptor().Values().Len() {
		values[idx] = storage.BlockDeviceWipeDescriptor_Method_name[int32(idx)]
	}

	return values
}

func init() {
	addCommand(wipeCmd)

	wipeDiskCmd.Flags().StringVar(&wipeDiskCmdFlags.wipeMethod, "method", wipeMethodValues()[0], fmt.Sprintf("wipe method to use %s", wipeMethodValues()))
	wipeDiskCmd.Flags().BoolVar(&wipeDiskCmdFlags.skipVolumeCheck, "skip-volume-check", false, "skip volume check")
	wipeDiskCmd.Flags().BoolVar(&wipeDiskCmdFlags.skipSecondaryCheck, "skip-secondary-check", false, "skip secondary disk check (e.g. underlying disk for RAID or LVM), use with caution")
	wipeDiskCmd.Flags().BoolVar(&wipeDiskCmdFlags.dropPartition, "drop-partition", false, "drop partition after wipe (if applicable)")
	wipeDiskCmd.Flags().MarkHidden("skip-volume-check")    //nolint:errcheck
	wipeDiskCmd.Flags().MarkHidden("skip-secondary-check") //nolint:errcheck
	wipeDiskCmd.Flags().BoolVarP(&wipeDiskCmdFlags.insecure, "insecure", "i", false, "use Talos maintenance mode API")

	wipeCmd.AddCommand(wipeDiskCmd)
}
