// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package mgmt

import (
	"github.com/spf13/cobra"

	"github.com/talos-systems/talos/pkg/provision/providers/firecracker"
)

// firecrackerLaunchCmd represents the firecracker-launch command.
var firecrackerLaunchCmd = &cobra.Command{
	Use:    "firecracker-launch",
	Short:  "Internal command used by Firecracker provisioner",
	Long:   ``,
	Args:   cobra.NoArgs,
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return firecracker.Launch()
	},
}

func init() {
	addCommand(firecrackerLaunchCmd)
}
