// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"errors"

	"github.com/spf13/cobra"
)

var disksCmd = &cobra.Command{
	Use:    "disks",
	Short:  "Get the list of disks from /sys/block on the machine",
	Long:   ``,
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return errors.New("`talosctl disks` is deprecated, please use `talosctl get disks`, `talosctl get systemdisk`, `talosctl get discoveredvolumes` instead")
	},
}

func init() {
	addCommand(disksCmd)
}
