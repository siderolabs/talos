/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package cmd

import (
	"fmt"
	"io"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	machineapi "github.com/talos-systems/talos/api/machine"
	"github.com/talos-systems/talos/cmd/osctl/pkg/client"
	"github.com/talos-systems/talos/cmd/osctl/pkg/helpers"
)

// lsCmd represents the ls command
var lsCmd = &cobra.Command{
	Use:   "ls [path]",
	Short: "Retrieve a directory listing",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		setupClient(func(c *client.Client) {
			rootDir := "/"
			if len(args) > 0 {
				rootDir = args[0]
			}
			long, err := cmd.Flags().GetBool("long")
			if err != nil {
				helpers.Fatalf("failed to parse long flag: %v", err)
			}
			recurse, err := cmd.Flags().GetBool("recurse")
			if err != nil {
				helpers.Fatalf("failed to parse recurse flag: %v", err)
			}
			recursionDepth, err := cmd.Flags().GetInt32("depth")
			if err != nil {
				helpers.Fatalf("failed to parse depth flag: %v", err)
			}

			stream, err := c.LS(globalCtx, machineapi.LSRequest{
				Root:           rootDir,
				Recurse:        recurse,
				RecursionDepth: recursionDepth,
			})
			if err != nil {
				helpers.Fatalf("error fetching logs: %s", err)
			}

			if !long {
				for {
					info, err := stream.Recv()
					if err != nil {
						if err == io.EOF || status.Code(err) == codes.Canceled {
							return
						}
						helpers.Fatalf("error streaming results: %s", err)
					}
					if info.Error != "" {
						fmt.Fprintf(os.Stderr, "error reading file %s: %s\n", info.Name, info.Error)
					} else {
						fmt.Println(info.RelativeName)
					}
				}
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
			fmt.Fprintln(w, "MODE\tSIZE(B)\tLASTMOD\tNAME")
			for {
				info, err := stream.Recv()
				if err != nil {
					if err == io.EOF || status.Code(err) == codes.Canceled {
						helpers.Should(w.Flush())
						return
					}
					helpers.Fatalf("error streaming results: %s", err)
				}

				if info.Error != "" {
					fmt.Fprintf(os.Stderr, "error reading file %s: %s\n", info.Name, info.Error)
				} else {
					display := info.RelativeName
					if info.Link != "" {
						display += " -> " + info.Link
					}
					fmt.Fprintf(w, "%s\t%d\t%s\t%s\n",
						os.FileMode(info.Mode).String(),
						info.Size,
						time.Unix(info.Modified, 0).Format("Jan 2 2006"),
						display,
					)
				}
			}
		})
	},
}

func init() {
	lsCmd.Flags().BoolP("long", "l", false, "display additional file details")
	lsCmd.Flags().BoolP("recurse", "r", false, "recurse into subdirectories")
	lsCmd.Flags().Int32P("depth", "d", 0, "maximum recursion depth")
	rootCmd.AddCommand(lsCmd)
}
