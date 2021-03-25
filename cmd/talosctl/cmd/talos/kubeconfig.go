// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"bufio"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/talos-systems/talos/cmd/talosctl/pkg/talos/helpers"
	"github.com/talos-systems/talos/internal/pkg/kubeconfig"
	"github.com/talos-systems/talos/pkg/machinery/client"
)

var (
	force            bool
	forceContextName string
	merge            bool
)

// kubeconfigCmd represents the kubeconfig command.
var kubeconfigCmd = &cobra.Command{
	Use:   "kubeconfig [local-path]",
	Short: "Download the admin kubeconfig from the node",
	Long: `Download the admin kubeconfig from the node.
If merge flag is defined, config will be merged with ~/.kube/config or [local-path] if specified.
Otherwise kubeconfig will be written to PWD or [local-path] if specified.`,
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
					localPath, err = kubeconfig.DefaultPath()
					if err != nil {
						return err
					}
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
			defer r.Close() //nolint:errcheck

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

	merger, err := kubeconfig.Load(localPath)
	if err != nil {
		return err
	}

	interactive := isatty.IsTerminal(os.Stdout.Fd())

	err = merger.Merge(config, kubeconfig.MergeOptions{
		ActivateContext:  true,
		ForceContextName: forceContextName,
		OutputWriter:     os.Stdout,
		ConflictHandler: func(component kubeconfig.ConfigComponent, name string) (kubeconfig.ConflictDecision, error) {
			if force {
				return kubeconfig.OverwriteDecision, nil
			}

			if !interactive {
				return kubeconfig.RenameDecision, nil
			}

			return askOverwriteOrRename(fmt.Sprintf("%s %q already exists", component, name))
		},
	})

	if err != nil {
		return err
	}

	return merger.Write(localPath)
}

func askOverwriteOrRename(prompt string) (kubeconfig.ConflictDecision, error) {
	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Printf("%s [(r)ename/(o)verwrite]: ", prompt)

		response, err := reader.ReadString('\n')
		if err != nil {
			return "", err
		}

		switch strings.ToLower(strings.TrimSpace(response)) {
		case "overwrite", "o":
			return kubeconfig.OverwriteDecision, nil
		case "rename", "r":
			return kubeconfig.RenameDecision, nil
		}
	}
}

func init() {
	kubeconfigCmd.Flags().BoolVarP(&force, "force", "f", false, "Force overwrite of kubeconfig if already present, force overwrite on kubeconfig merge")
	kubeconfigCmd.Flags().StringVar(&forceContextName, "force-context-name", "", "Force context name for kubeconfig merge")
	kubeconfigCmd.Flags().BoolVarP(&merge, "merge", "m", true, "Merge with existing kubeconfig")
	addCommand(kubeconfigCmd)
}
