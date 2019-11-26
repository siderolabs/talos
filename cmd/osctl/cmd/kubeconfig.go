// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// nolint: dupl,golint
package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/spf13/cobra"

	"github.com/talos-systems/talos/cmd/osctl/pkg/client"
	"github.com/talos-systems/talos/cmd/osctl/pkg/helpers"
)

var force bool

// kubeconfigCmd represents the kubeconfig command
var kubeconfigCmd = &cobra.Command{
	Use:   "kubeconfig [local-path]",
	Short: "Download the admin kubeconfig from the node",
	Long: `Download the admin kubeconfig from the node.
Kubeconfig will be written to PWD/kubeconfig or [local-path]/kubeconfig if specified.`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) != 1 {
			helpers.Should(cmd.Usage())
			os.Exit(1)
		}

		setupClient(func(c *client.Client) {
			r, errCh, err := c.KubeconfigRaw(globalCtx)
			if err != nil {
				helpers.Fatalf("error copying: %s", err)
			}

			var wg sync.WaitGroup

			wg.Add(1)
			go func() {
				defer wg.Done()
				for err := range errCh {
					fmt.Fprintln(os.Stderr, err.Error())
				}
			}()

			defer wg.Wait()

			localPath, err := os.Getwd()
			if err != nil {
				helpers.Fatalf("error getting current working directory: %s", err)
			}

			if len(args) == 1 {
				localPath = args[0]
			}
			localPath = filepath.Clean(localPath)

			// Drop the existing kubeconfig before writing the new one if force flag is specified.
			if force {
				err = os.Remove(filepath.Join(localPath, "kubeconfig"))
				if err != nil && !os.IsNotExist(err) {
					helpers.Fatalf("error deleting existing kubeconfig: %s", err)
				}
			}
			extractTarGz(localPath, r)
		})
	},
}

func init() {
	kubeconfigCmd.Flags().BoolVarP(&force, "force", "f", false, "Force overwrite of kubeconfig if already present")
	rootCmd.AddCommand(kubeconfigCmd)
}
