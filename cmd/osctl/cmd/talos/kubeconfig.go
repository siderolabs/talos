// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/spf13/cobra"

	"github.com/talos-systems/talos/cmd/osctl/pkg/client"
	"github.com/talos-systems/talos/cmd/osctl/pkg/talos/helpers"
)

var force bool

// kubeconfigCmd represents the kubeconfig command
var kubeconfigCmd = &cobra.Command{
	Use:   "kubeconfig [local-path]",
	Short: "Download the admin kubeconfig from the node",
	Long: `Download the admin kubeconfig from the node.
Kubeconfig will be written to PWD/kubeconfig or [local-path]/kubeconfig if specified.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return WithClient(func(ctx context.Context, c *client.Client) error {
			if err := helpers.FailIfMultiNodes(ctx, "kubeconfig"); err != nil {
				return err
			}

			r, errCh, err := c.KubeconfigRaw(ctx)
			if err != nil {
				return fmt.Errorf("error copying: %w", err)
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
				return fmt.Errorf("error getting current working directory: %s", err)
			}

			if len(args) == 1 {
				localPath = args[0]
			}
			localPath = filepath.Clean(localPath)

			// Drop the existing kubeconfig before writing the new one if force flag is specified.
			if force {
				err = os.Remove(filepath.Join(localPath, "kubeconfig"))
				if err != nil && !os.IsNotExist(err) {
					return fmt.Errorf("error deleting existing kubeconfig: %s", err)
				}
			}

			return helpers.ExtractTarGz(localPath, r)
		})
	},
}

func init() {
	kubeconfigCmd.Flags().BoolVarP(&force, "force", "f", false, "Force overwrite of kubeconfig if already present")
	addCommand(kubeconfigCmd)
}
