// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build linux || darwin

package mgmt

import (
	"github.com/spf13/cobra"

	"github.com/siderolabs/talos/pkg/provision/providers/qemu"
)

// qemuLaunchCmd represents the qemu-launch command.
var qemuLaunchCmd = &cobra.Command{
	Use:    "qemu-launch",
	Short:  "Internal command used by QEMU provisioner",
	Long:   ``,
	Args:   cobra.NoArgs,
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return qemu.Launch()
	},
}

func init() {
	addCommand(qemuLaunchCmd)
}
