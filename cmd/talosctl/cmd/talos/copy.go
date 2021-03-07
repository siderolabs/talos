// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/spf13/cobra"

	"github.com/talos-systems/talos/cmd/talosctl/pkg/talos/helpers"
	"github.com/talos-systems/talos/pkg/machinery/client"
)

// cpCmd represents the cp command.
var cpCmd = &cobra.Command{
	Use:     "copy <src-path> -|<local-path>",
	Aliases: []string{"cp"},
	Short:   "Copy data out from the node",
	Long: `Creates an .tar.gz archive at the node starting at <src-path> and
streams it back to the client.

If '-' is given for <local-path>, archive is written to stdout.
Otherwise archive is extracted to <local-path> which should be an empty directory or
talosctl creates a directory if <local-path> doesn't exist. Command doesn't preserve
ownership and access mode for the files in extract mode, while  streamed .tar archive
captures ownership and permission bits.`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		return WithClient(func(ctx context.Context, c *client.Client) error {
			if err := helpers.FailIfMultiNodes(ctx, "copy"); err != nil {
				return err
			}

			r, errCh, err := c.Copy(ctx, args[0])
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

			localPath := args[1]

			if localPath == "-" {
				_, err = io.Copy(os.Stdout, r)

				return err
			}

			localPath = filepath.Clean(localPath)

			fi, err := os.Stat(localPath)
			if err == nil && !fi.IsDir() {
				return fmt.Errorf("local path %q should be a directory", args[1])
			}
			if err != nil {
				if !os.IsNotExist(err) {
					return fmt.Errorf("failed to stat local path: %w", err)
				}
				if err = os.MkdirAll(localPath, 0o777); err != nil {
					return fmt.Errorf("error creating local path %q: %w", localPath, err)
				}
			}

			return helpers.ExtractTarGz(localPath, r)
		})
	},
}

func init() {
	addCommand(cpCmd)
}
