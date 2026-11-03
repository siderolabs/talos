// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/global"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
)

// mdCmd represents the md command.
var mdCmd = &cobra.Command{
	Use:   "md",
	Short: "Manage MD (software RAID) arrays",
	Args:  cobra.NoArgs,
}

var mdAddCmdFlags struct {
	global.InsecureFlags
}

// mdAddCmd adds a disk to a degraded MD array.
var mdAddCmd = &cobra.Command{
	Use:   "add <array device> <disk>",
	Short: "Add a disk to a degraded MD (software RAID) array",
	Long: `Add a disk to a degraded raid1 MD (software RAID) array, which then rebuilds onto it.

Use this after replacing a failed member: it restores the redundancy the array
already declares, it never adds a spare or grows the number of RAID devices.
The array must be degraded, with no sync action in progress. Arrays managed by
a RAIDArrayConfig are refused: update its volume selector instead.

The disk must be a whole disk which is blank (no partition table or filesystem
signature, see "talosctl wipe disk"), not the system disk, not in use, at least
as large as the array members and with the same logical sector size.

Both arguments are full /dev paths, e.g.
/dev/disk/by-id/md-name-talos:boot and /dev/nvme1n1 (or a /dev/disk/by-path link).
As disk paths are specific to a machine, exactly one node must be targeted.`,
	Example: `  talosctl md add /dev/disk/by-id/md-name-talos:boot /dev/disk/by-path/pci-0000:02:00.0-nvme-1`,
	Args:    cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		device, member := args[0], args[1]

		for _, path := range args {
			if !strings.HasPrefix(path, "/dev/") {
				return fmt.Errorf("%q must be a full /dev path", path)
			}
		}

		ctx := cmd.Context()

		clientFactory, err := NewClientFactory(ctx, &mdAddCmdFlags)
		if err != nil {
			return err
		}

		defer clientFactory.Close() //nolint:errcheck

		// disk paths are specific to one machine: never fan the same request out to several nodes
		ctx, c, _, err := clientFactory.BuildClientEnforceSingleNode(ctx, "md add")
		if err != nil {
			return err
		}

		_, err = c.MDService().Add(ctx, &machine.MDAddRequest{
			Device: device,
			Member: member,
		})

		return err
	},
}

func init() {
	addCommand(mdCmd)

	mdAddCmdFlags.InsecureFlags.AddFlags(mdAddCmd)

	mdCmd.AddCommand(mdAddCmd)
}
