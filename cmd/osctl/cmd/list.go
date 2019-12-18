// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cmd

import (
	"fmt"
	"io"
	"os"
	"text/tabwriter"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	machineapi "github.com/talos-systems/talos/api/machine"
	"github.com/talos-systems/talos/cmd/osctl/pkg/client"
	"github.com/talos-systems/talos/cmd/osctl/pkg/helpers"
)

const sixMonths = 6 * time.Hour * 24 * 30

// lsCmd represents the ls command
var lsCmd = &cobra.Command{
	Use:     "list [path]",
	Aliases: []string{"ls"},
	Short:   "Retrieve a directory listing",
	Long:    ``,
	Run: func(cmd *cobra.Command, args []string) {
		setupClient(func(c *client.Client) {
			rootDir := "/"
			if len(args) > 0 {
				rootDir = args[0]
			}
			long, err := cmd.Flags().GetBool("long")
			if err != nil {
				helpers.Fatalf("failed to parse long flag: %w", err)
			}
			recurse, err := cmd.Flags().GetBool("recurse")
			if err != nil {
				helpers.Fatalf("failed to parse recurse flag: %w", err)
			}
			recursionDepth, err := cmd.Flags().GetInt32("depth")
			if err != nil {
				helpers.Fatalf("failed to parse depth flag: %w", err)
			}
			humanizeFlag, err := cmd.Flags().GetBool("humanize")
			if err != nil {
				helpers.Fatalf("failed to parse humanize flag: %w", err)
			}

			stream, err := c.LS(globalCtx, machineapi.ListRequest{
				Root:           rootDir,
				Recurse:        recurse,
				RecursionDepth: recursionDepth,
			})
			if err != nil {
				helpers.Fatalf("error fetching logs: %s", err)
			}

			defaultNode := remotePeer(stream.Context())

			if !long {
				w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
				fmt.Fprintln(w, "NODE\tNAME")

				multipleNodes := false
				node := defaultNode

				for {
					info, err := stream.Recv()
					if err != nil {
						if err == io.EOF || status.Code(err) == codes.Canceled {
							if multipleNodes {
								helpers.Should(w.Flush())
							}
							return
						}
						helpers.Fatalf("error streaming results: %s", err)
					}

					if info.Metadata != nil && info.Metadata.Hostname != "" {
						multipleNodes = true
						node = info.Metadata.Hostname
					}

					if info.Metadata != nil && info.Metadata.Error != "" {
						fmt.Fprintf(os.Stderr, "%s: %s\n", node, info.Metadata.Error)
						continue
					}

					if info.Error != "" {
						fmt.Fprintf(os.Stderr, "%s: error reading file %s: %s\n", node, info.Name, info.Error)
						continue
					}

					if !multipleNodes {
						fmt.Println(info.RelativeName)
					} else {
						fmt.Fprintf(w, "%s\t%s\n",
							node,
							info.RelativeName,
						)
					}

				}
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
			fmt.Fprintln(w, "NODE\tMODE\tSIZE(B)\tLASTMOD\tNAME")
			for {
				info, err := stream.Recv()
				if err != nil {
					if err == io.EOF || status.Code(err) == codes.Canceled {
						helpers.Should(w.Flush())
						return
					}
					helpers.Fatalf("error streaming results: %s", err)
				}

				node := defaultNode
				if info.Metadata != nil && info.Metadata.Hostname != "" {
					node = info.Metadata.Hostname
				}

				if info.Error != "" {
					fmt.Fprintf(os.Stderr, "%s: error reading file %s: %s\n", node, info.Name, info.Error)
					continue
				}

				if info.Metadata != nil && info.Metadata.Error != "" {
					fmt.Fprintf(os.Stderr, "%s: %s\n", node, info.Metadata.Error)
					continue
				}

				display := info.RelativeName
				if info.Link != "" {
					display += " -> " + info.Link
				}

				size := fmt.Sprintf("%d", info.Size)

				if humanizeFlag {
					size = humanize.Bytes(uint64(info.Size))
				}

				timestamp := time.Unix(info.Modified, 0)
				timestampFormatted := ""

				if humanizeFlag {
					timestampFormatted = humanize.Time(timestamp)
				} else {
					if time.Since(timestamp) < sixMonths {
						timestampFormatted = timestamp.Format("Jan _2 15:04:05")
					} else {
						timestampFormatted = timestamp.Format("Jan _2 2006 15:04")
					}
				}

				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
					node,
					os.FileMode(info.Mode).String(),
					size,
					timestampFormatted,
					display,
				)
			}
		})
	},
}

func init() {
	lsCmd.Flags().BoolP("long", "l", false, "display additional file details")
	lsCmd.Flags().BoolP("recurse", "r", false, "recurse into subdirectories")
	lsCmd.Flags().BoolP("humanize", "H", false, "humanize size and time in the output")
	lsCmd.Flags().Int32P("depth", "d", 0, "maximum recursion depth")
	rootCmd.AddCommand(lsCmd)
}
