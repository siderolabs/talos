/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

// nolint: dupl,golint
package cmd

import (
	"log"

	"github.com/spf13/cobra"
	"github.com/talos-systems/talos/internal/app/osinstall/internal/userdata"
)

// validateCmd reads in a userData file and attempts to parse it
var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate userdata",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		ud, err := userdata.UserData(userdataFile)
		if err != nil {
			log.Fatal(err)
		}
		log.Println(ud)
	},
}

func init() {
	validateCmd.Flags().StringVarP(&userdataFile, "userdata", "u", "", "path or url of userdata file")
	rootCmd.AddCommand(validateCmd)
}
