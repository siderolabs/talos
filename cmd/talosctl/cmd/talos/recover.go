// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"fmt"

	"github.com/spf13/cobra"
)

// recoverCmd represents the recover command
var recoverCmd = &cobra.Command{
	Use:   "recover",
	Short: "Recover a control plane",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("recover called")
	},
}

func init() {
	recoverCmd.Flags().StringVarP(recoverSource, "source", "s", "api", "The data source for restoring the control plane manifests from (valid options are 'api' and 'etcd')")
	addCommand(recoverCmd)
}
