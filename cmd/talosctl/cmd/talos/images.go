// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"github.com/spf13/cobra"
)

// imagesCmd represents the (deprecated) images command.
//
// TODO: remove in Talos 1.6, add 'images' as an alias to talosctl image.
var imagesCmd = &cobra.Command{
	Use:    "images",
	Short:  "List the default images used by Talos",
	Long:   ``,
	Hidden: true,
	RunE:   imageDefaultCmd.RunE,
}

func init() {
	addCommand(imagesCmd)
}
