// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package mgmt

import "github.com/spf13/cobra"

// Commands is a list of commands published by the package.
var Commands []*cobra.Command

func addCommand(cmd *cobra.Command) {
	Commands = append(Commands, cmd)
}
