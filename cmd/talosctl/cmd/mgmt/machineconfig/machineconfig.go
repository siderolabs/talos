// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package machineconfig

import "github.com/spf13/cobra"

// Cmd represents the `machineconfig` command.
var Cmd = &cobra.Command{
	Use:     "machineconfig",
	Short:   "Machine config related commands",
	Aliases: []string{"mc"},
}
