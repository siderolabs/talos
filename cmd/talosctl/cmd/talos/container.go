// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"github.com/spf13/cobra"
)

// containerCmd represents the container command group.
var containerCmd = &cobra.Command{
	Use:   "container",
	Short: "Manage containers declared with ContainerConfig documents",
	Long:  ``,
	Args:  cobra.NoArgs,
}

func init() {
	addCommand(containerCmd)
}
