/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package cmd

import (
	"os"
	"os/user"
	"path"

	"github.com/spf13/cobra"
	"github.com/talos-systems/talos/cmd/osctl/pkg/helpers"
	"github.com/talos-systems/talos/internal/pkg/constants"
)

var (
	ca           string
	crt          string
	key          string
	organization string
	rsa          bool
	name         string
	csr          string
	ip           string
	hours        int
	kubernetes   bool
	talosconfig  string
	target       string
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "osctl",
	Short: "A CLI for out-of-band management of Kubernetes nodes created by Talos",
	Long:  ``,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	var (
		defaultTalosConfig string
		ok                 bool
	)
	if defaultTalosConfig, ok = os.LookupEnv(constants.TalosConfigEnvVar); !ok {
		u, err := user.Current()
		if err != nil {
			return
		}
		defaultTalosConfig = path.Join(u.HomeDir, ".talos", "config")
	}
	rootCmd.PersistentFlags().StringVar(&talosconfig, "talosconfig", defaultTalosConfig, "The path to the Talos configuration file")
	if err := rootCmd.Execute(); err != nil {
		helpers.Fatalf("%s", err)
	}
}
