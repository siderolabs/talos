// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"context"
	"errors"

	"github.com/spf13/cobra"

	"github.com/siderolabs/talos/pkg/machinery/client"
)

// InterfacesCmd represents the net interfaces command.
var InterfacesCmd = &cobra.Command{
	Use:    "interfaces",
	Short:  "List network interfaces",
	Long:   ``,
	Args:   cobra.NoArgs,
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return WithClient(func(ctx context.Context, c *client.Client) error {
			return errors.New("`talosctl interfaces` is deprecated, please use `talosctl get addresses` and `talosctl get links` instead")
		})
	},
}

func init() {
	addCommand(InterfacesCmd)
}
