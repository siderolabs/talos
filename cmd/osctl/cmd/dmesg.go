// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cmd

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/talos-systems/talos/cmd/osctl/pkg/client"
	"github.com/talos-systems/talos/cmd/osctl/pkg/helpers"
)

var dmesgTail bool

// dmesgCmd represents the dmesg command
var dmesgCmd = &cobra.Command{
	Use:   "dmesg",
	Short: "Retrieve kernel logs",
	Long:  ``,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) != 0 {
			helpers.Should(cmd.Usage())
			os.Exit(1)
		}

		return setupClientE(func(c *client.Client) error {
			stream, err := c.Dmesg(globalCtx, follow, dmesgTail)
			if err != nil {
				return fmt.Errorf("error getting dmesg: %w", err)
			}

			defaultNode := remotePeer(stream.Context())

			for {
				resp, err := stream.Recv()
				if err != nil {
					if err == io.EOF || status.Code(err) == codes.Canceled {
						break
					}

					return fmt.Errorf("error reading from stream: %w", err)
				}

				node := defaultNode
				if resp.Metadata != nil {
					node = resp.Metadata.Hostname

					if resp.Metadata.Error != "" {
						fmt.Fprintf(os.Stderr, "%s: %s\n", node, resp.Metadata.Error)
					}
				}

				if resp.Bytes != nil {
					fmt.Printf("%s: %s", node, resp.Bytes)
				}
			}

			return nil
		})
	},
}

func init() {
	rootCmd.AddCommand(dmesgCmd)
	dmesgCmd.Flags().BoolVarP(&follow, "follow", "f", false, "specify if the kernel log should be streamed")
	dmesgCmd.Flags().BoolVarP(&dmesgTail, "tail", "", false, "specify if only new messages should be sent (makes sense only when combined with --follow)")
}
