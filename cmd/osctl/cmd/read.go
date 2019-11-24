// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cmd

import (
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/spf13/cobra"

	"github.com/talos-systems/talos/cmd/osctl/pkg/client"
	"github.com/talos-systems/talos/cmd/osctl/pkg/helpers"
)

// readCmd represents the read command
var readCmd = &cobra.Command{
	Use:   "read <path>",
	Short: "Read a file on the machine",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) != 1 {
			helpers.Should(cmd.Usage())
			os.Exit(1)
		}

		setupClient(func(c *client.Client) {
			r, errCh, err := c.Read(globalCtx, args[0])
			if err != nil {
				helpers.Fatalf("error reading file: %s", err)
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

			// nolint: errcheck
			_, err = io.Copy(os.Stdout, r)
			if err != nil {
				helpers.Fatalf("error reading: %s", err)
			}
		})
	},
}

func init() {
	rootCmd.AddCommand(readCmd)
}
