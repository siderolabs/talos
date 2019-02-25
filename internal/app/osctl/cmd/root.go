/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package cmd

import (
	"fmt"
	"os"
	"os/user"
	"path"

	"github.com/autonomy/talos/internal/pkg/constants"
	"github.com/spf13/cobra"
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
		fmt.Println(err)
		os.Exit(1)
	}
}
