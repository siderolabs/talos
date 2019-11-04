// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/spf13/cobra"

	"github.com/talos-systems/talos/cmd/osctl/pkg/client"
	"github.com/talos-systems/talos/cmd/osctl/pkg/helpers"
)

// cpCmd represents the cp command
var cpCmd = &cobra.Command{
	Use:   "cp <src-path> -|<local-path>",
	Short: "Copy data out from the node",
	Long: `Creates an .tar.gz archive at the node starting at <src-path> and
streams it back to the client.

If '-' is given for <local-path>, archive is written to stdout.
Otherwise archive is extracted to <local-path> which should be an empty directory or
osctl creates a directory if <local-path> doesn't exist. Command doesn't preserve
ownership and access mode for the files in extract mode, while  streamed .tar archive
captures ownership and permission bits.`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) != 2 {
			helpers.Should(cmd.Usage())
			os.Exit(1)
		}

		setupClient(func(c *client.Client) {
			r, errCh, err := c.CopyOut(globalCtx, args[0])
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

			localPath := args[1]

			if localPath == "-" {
				// nolint: errcheck
				_, err = io.Copy(os.Stdout, r)
				if err != nil {
					helpers.Fatalf("error copying: %s", err)
				}
				return
			}

			localPath = filepath.Clean(localPath)

			fi, err := os.Stat(localPath)
			if err == nil && !fi.IsDir() {
				helpers.Fatalf("local path %q should be a directory", args[1])
			}
			if err != nil {
				if !os.IsNotExist(err) {
					helpers.Fatalf("failed to stat local path: %s", err)
				}
				if err = os.MkdirAll(localPath, 0777); err != nil {
					helpers.Fatalf("error creating local path %q: %s", localPath, err)
				}
			}

			extractTarGz(localPath, r)
		})
	},
}

func init() {
	rootCmd.AddCommand(cpCmd)
}
