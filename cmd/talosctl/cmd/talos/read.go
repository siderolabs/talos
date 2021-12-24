// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/spf13/cobra"

	"github.com/talos-systems/talos/cmd/talosctl/pkg/talos/helpers"
	"github.com/talos-systems/talos/pkg/machinery/client"
)

// readCmd represents the read command.
var readCmd = &cobra.Command{
	Use:     "read <path>",
	Short:   "Read a file on the machine",
	Long:    ``,
	Args:    cobra.ExactArgs(1),
	Aliases: []string{"cat"},
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) != 0 {
			return nil, cobra.ShellCompDirectiveError | cobra.ShellCompDirectiveNoFileComp
		}

		return completePathFromNode(toComplete), cobra.ShellCompDirectiveNoFileComp
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		return WithClient(func(ctx context.Context, c *client.Client) error {
			if err := helpers.FailIfMultiNodes(ctx, "read"); err != nil {
				return err
			}

			r, errCh, err := c.Read(ctx, args[0])
			if err != nil {
				return fmt.Errorf("error reading file: %w", err)
			}

			defer r.Close() //nolint:errcheck

			var wg sync.WaitGroup

			wg.Add(1)
			go func() {
				defer wg.Done()
				for err := range errCh {
					fmt.Fprintln(os.Stderr, err.Error())
				}
			}()

			defer wg.Wait()

			_, err = io.Copy(os.Stdout, r)
			if err != nil {
				return fmt.Errorf("error reading: %w", err)
			}

			return r.Close()
		})
	},
}

func init() {
	addCommand(readCmd)
}
