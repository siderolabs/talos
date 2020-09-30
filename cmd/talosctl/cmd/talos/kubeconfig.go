// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"sync"

	"github.com/particledecay/kconf/pkg/kubeconfig"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/spf13/cobra"

	"github.com/talos-systems/talos/cmd/talosctl/pkg/talos/helpers"
	"github.com/talos-systems/talos/pkg/machinery/client"
)

var (
	force bool
	merge bool
)

// kubeconfigCmd represents the kubeconfig command.
var kubeconfigCmd = &cobra.Command{
	Use:   "kubeconfig [local-path]",
	Short: "Download the admin kubeconfig from the node",
	Long: `Download the admin kubeconfig from the node.
Kubeconfig will be written to PWD or [local-path] if specified.
If merge flag is defined, config will be merged with ~/.kube/config or [local-path] if specified.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return WithClient(func(ctx context.Context, c *client.Client) error {
			if err := helpers.FailIfMultiNodes(ctx, "kubeconfig"); err != nil {
				return err
			}

			var localPath string

			if len(args) == 0 {
				// no path given, use defaults
				var err error

				if merge {
					var usr *user.User
					usr, err = user.Current()

					if err != nil {
						return err
					}

					localPath = filepath.Join(usr.HomeDir, ".kube/config")
				} else {
					localPath, err = os.Getwd()
					if err != nil {
						return fmt.Errorf("error getting current working directory: %s", err)
					}
				}
			} else {
				localPath = args[0]
			}

			localPath = filepath.Clean(localPath)

			st, err := os.Stat(localPath)
			if err != nil {
				if !os.IsNotExist(err) {
					return fmt.Errorf("error checking path %q: %w", localPath, err)
				}

				err = os.MkdirAll(filepath.Dir(localPath), 0o755)
				if err != nil {
					return err
				}
			} else if st.IsDir() {
				// only dir name was given, append `kubeconfig` by default
				localPath = filepath.Join(localPath, "kubeconfig")
			}

			_, err = os.Stat(localPath)
			if err == nil && !(force || merge) {
				return fmt.Errorf("kubeconfig file already exists, use --force to overwrite: %q", localPath)
			} else if err != nil {
				if os.IsNotExist(err) {
					// merge doesn't make sense if target path doesn't exist
					merge = false
				} else {
					return fmt.Errorf("error checking path %q: %w", localPath, err)
				}
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
			defer r.Close() //nolint: errcheck

			data, err := helpers.ExtractFileFromTarGz("kubeconfig", r)
			if err != nil {
				return err
			}

			if merge {
				return extractAndMerge(data, localPath)
			}

			return ioutil.WriteFile(localPath, data, 0o640)
		})
	},
}

func extractAndMerge(data []byte, localPath string) error {
	config, err := clientcmd.Load(data)
	if err != nil {
		return err
	}

	baseConfig, err := clientcmd.LoadFromFile(localPath)
	if err != nil {
		return err
	}

	kconf := kubeconfig.KConf{Config: *baseConfig}
	err = kconf.Merge(config, Cmdcontext)

	if err != nil {
		return err
	}

	return clientcmd.WriteToFile(kconf.Config, localPath)
}

func init() {
	kubeconfigCmd.Flags().BoolVarP(&force, "force", "f", false, "Force overwrite of kubeconfig if already present")
	kubeconfigCmd.Flags().BoolVarP(&merge, "merge", "m", false, "Merge with existing kubeconfig")
	addCommand(kubeconfigCmd)
}
