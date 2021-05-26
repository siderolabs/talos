// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package gen

import (
	"github.com/spf13/cobra"
)

// Cmd represents the `gen` command.
var Cmd = &cobra.Command{
	Use:   "gen",
	Short: "Generate CAs, certificates, and private keys",
	Long:  ``,
}
