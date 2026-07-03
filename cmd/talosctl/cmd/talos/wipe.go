// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/siderolabs/gen/xslices"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/global"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/api/storage"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/client/multiplex"
)

// wipeCmd represents the wipe command.
var wipeCmd = &cobra.Command{
	Use:   "wipe",
	Short: "Wipe block device or volumes",
	Args:  cobra.NoArgs,
}

var wipeDiskCmdFlags struct {
	global.InsecureFlags

	wipeMethod         string
	skipVolumeCheck    bool
	skipSecondaryCheck bool
	dropPartition      bool
}

// wipeDiskCmd represents the wipe disk command.
var wipeDiskCmd = &cobra.Command{
	Use:   "disk <device names>...",
	Short: "Wipe a block device (disk or partition) which is not used as a volume",
	Long: `Wipe a block device (disk or partition) which is not used as a volume.

Use device names as arguments, for example: vda or sda5.`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmdWipe(cmd.Context(), args)
	},
}

func cmdWipe(ctx context.Context, args []string) error {
	clientFactory, err := NewClientFactory(ctx, &wipeDiskCmdFlags)
	if err != nil {
		return err
	}

	defer clientFactory.Close() //nolint:errcheck

	method, ok := storage.BlockDeviceWipeDescriptor_Method_value[wipeDiskCmdFlags.wipeMethod]
	if !ok {
		return fmt.Errorf("invalid wipe method %q", wipeDiskCmdFlags.wipeMethod)
	}

	respCh := multiplex.UnaryViaFactory(
		ctx, clientFactory,
		func(ctx context.Context, c *client.Client) (struct{}, error) {
			return struct{}{}, c.BlockDeviceWipe(
				ctx, &storage.BlockDeviceWipeRequest{
					Devices: xslices.Map(
						args, func(devName string) *storage.BlockDeviceWipeDescriptor {
							return &storage.BlockDeviceWipeDescriptor{
								Device:             devName,
								Method:             storage.BlockDeviceWipeDescriptor_Method(method),
								SkipVolumeCheck:    wipeDiskCmdFlags.skipVolumeCheck,
								SkipSecondaryCheck: wipeDiskCmdFlags.skipSecondaryCheck,
								DropPartition:      wipeDiskCmdFlags.dropPartition,
							}
						},
					),
				},
			)
		},
	)

	var errs error

	for resp := range respCh {
		if resp.Err != nil {
			errs = errors.Join(errs, fmt.Errorf("error wiping device on node %s: %w", resp.Node, resp.Err))
		}
	}

	return errs
}

func wipeMethodValues() []string {
	var method storage.BlockDeviceWipeDescriptor_Method

	values := make([]string, method.Descriptor().Values().Len())

	for idx := range method.Descriptor().Values().Len() {
		values[idx] = storage.BlockDeviceWipeDescriptor_Method_name[int32(idx)]
	}

	return values
}

var wipeLVMCmdFlags struct {
	global.InsecureFlags
}

// wipeLVCmd removes a single LVM logical volume.
var wipeLVCmd = &cobra.Command{
	Use:   "lv <vg/lv>",
	Short: "Remove an LVM logical volume",
	Long: `Remove an LVM logical volume.

The argument is the qualified logical-volume name, e.g. vg0/lv0.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		vg, lv, ok := strings.Cut(args[0], "/")
		if !ok || vg == "" || lv == "" {
			return fmt.Errorf("invalid logical volume %q, expected <vg>/<lv>", args[0])
		}

		ctx := cmd.Context()

		clientFactory, err := NewClientFactory(ctx, &wipeLVMCmdFlags)
		if err != nil {
			return err
		}

		defer clientFactory.Close() //nolint:errcheck

		responseChan := multiplex.UnaryViaFactory(
			ctx, clientFactory,
			func(ctx context.Context, c *client.Client) (*emptypb.Empty, error) {
				return c.LVMClient.LogicalVolumeRemove(ctx, &machine.LVMServiceLogicalVolumeRemoveRequest{
					VolumeGroup:   vg,
					LogicalVolume: lv,
				})
			},
		)

		var errs error

		for resp := range responseChan {
			if resp.Err != nil {
				errs = errors.Join(errs, fmt.Errorf("error from node %s: %w", resp.Node, resp.Err))
			}
		}

		return errs
	},
}

// wipeVGCmd removes a single LVM volume group.
var wipeVGCmd = &cobra.Command{
	Use:   "vg <name>",
	Short: "Remove an LVM volume group (cascades to its LVs)",
	Long: `Remove an LVM volume group.

WARNING: this is destructive. Every logical volume inside the group is
removed first, then the volume group itself. There is no separate
confirmation per LV. The underlying physical volumes keep their LVM
labels and remain claimable until you run "talosctl wipe pv".`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		vg := args[0]

		ctx := cmd.Context()

		clientFactory, err := NewClientFactory(ctx, &wipeLVMCmdFlags)
		if err != nil {
			return err
		}

		defer clientFactory.Close() //nolint:errcheck

		responseChan := multiplex.UnaryViaFactory(
			ctx, clientFactory,
			func(ctx context.Context, c *client.Client) (*emptypb.Empty, error) {
				return c.LVMClient.VolumeGroupRemove(ctx, &machine.LVMServiceVolumeGroupRemoveRequest{
					VolumeGroup: vg,
				})
			},
		)

		var errs error

		for resp := range responseChan {
			if resp.Err != nil {
				errs = errors.Join(errs, fmt.Errorf("error from node %s: %w", resp.Node, resp.Err))
			}
		}

		return errs
	},
}

// wipePVCmd removes the LVM label on a single physical volume.
var wipePVCmd = &cobra.Command{
	Use:   "pv <device>",
	Short: "Remove an LVM physical volume label",
	Long: `Wipe LVM metadata from a block device.

The PV must not be part of an active volume group; remove the VG first with "talosctl wipe vg".

The argument is a full device path, e.g. /dev/sda1.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		device := args[0]

		ctx := cmd.Context()

		clientFactory, err := NewClientFactory(ctx, &wipeLVMCmdFlags)
		if err != nil {
			return err
		}

		defer clientFactory.Close() //nolint:errcheck

		responseChan := multiplex.UnaryViaFactory(
			ctx, clientFactory,
			func(ctx context.Context, c *client.Client) (*emptypb.Empty, error) {
				return c.LVMClient.PhysicalVolumeRemove(ctx, &machine.LVMServicePhysicalVolumeRemoveRequest{
					Device: device,
				})
			},
		)

		var errs error

		for resp := range responseChan {
			if resp.Err != nil {
				errs = errors.Join(errs, fmt.Errorf("error from node %s: %w", resp.Node, resp.Err))
			}
		}

		return errs
	},
}

// wipeMDCmd stops an MD (software RAID) array and clears member superblocks.
var wipeMDCmd = &cobra.Command{
	Use:   "md <device>",
	Short: "Destroy an MD (software RAID) array",
	Long: `Stop an MD (software RAID) array and clear the superblock on every member device.

WARNING: this is destructive. The array must not be in use (mounted or claimed
by another device). The argument is the full array device path, e.g.
/dev/disk/by-id/md-name-data.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		device := args[0]
		if !strings.HasPrefix(device, "/dev/") {
			return fmt.Errorf("md device must be a full /dev path")
		}

		ctx := cmd.Context()

		clientFactory, err := NewClientFactory(ctx, &wipeLVMCmdFlags)
		if err != nil {
			return err
		}

		defer clientFactory.Close() //nolint:errcheck

		responseChan := multiplex.UnaryViaFactory(
			ctx, clientFactory,
			func(ctx context.Context, c *client.Client) (*emptypb.Empty, error) {
				return c.MDClient.Destroy(ctx, &machine.MDDestroyRequest{
					Device: device,
				})
			},
		)

		var errs error

		for resp := range responseChan {
			if resp.Err != nil {
				errs = errors.Join(errs, fmt.Errorf("error from node %s: %w", resp.Node, resp.Err))
			}
		}

		return errs
	},
}

func init() {
	addCommand(wipeCmd)

	wipeDiskCmd.Flags().StringVar(&wipeDiskCmdFlags.wipeMethod, "method", wipeMethodValues()[0], fmt.Sprintf("wipe method to use %s", wipeMethodValues()))
	wipeDiskCmd.Flags().BoolVar(&wipeDiskCmdFlags.skipVolumeCheck, "skip-volume-check", false, "skip volume check")
	wipeDiskCmd.Flags().BoolVar(&wipeDiskCmdFlags.skipSecondaryCheck, "skip-secondary-check", false, "skip secondary disk check (e.g. underlying disk for RAID or LVM), use with caution")
	wipeDiskCmd.Flags().BoolVar(&wipeDiskCmdFlags.dropPartition, "drop-partition", false, "drop partition after wipe (if applicable)")
	wipeDiskCmd.Flags().MarkHidden("skip-volume-check")    //nolint:errcheck
	wipeDiskCmd.Flags().MarkHidden("skip-secondary-check") //nolint:errcheck
	wipeDiskCmdFlags.InsecureFlags.AddFlags(wipeDiskCmd)

	for _, c := range []*cobra.Command{wipeLVCmd, wipeVGCmd, wipePVCmd, wipeMDCmd} {
		wipeLVMCmdFlags.InsecureFlags.AddFlags(c)
	}

	wipeCmd.AddCommand(wipeDiskCmd, wipeLVCmd, wipeVGCmd, wipePVCmd, wipeMDCmd)
}
