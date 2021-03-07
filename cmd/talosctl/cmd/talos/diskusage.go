// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"context"
	"fmt"
	"io"
	"os"
	"text/tabwriter"

	humanize "github.com/dustin/go-humanize"
	"github.com/spf13/cobra"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	machineapi "github.com/talos-systems/talos/pkg/machinery/api/machine"
	"github.com/talos-systems/talos/pkg/machinery/client"
)

var (
	all       bool
	threshold int64
)

// duCmd represents the du command.
var duCmd = &cobra.Command{
	Use:     "usage [path1] [path2] ... [pathN]",
	Aliases: []string{"du"},
	Short:   "Retrieve a disk usage",
	Long:    ``,
	RunE: func(cmd *cobra.Command, args []string) error {
		return WithClient(func(ctx context.Context, c *client.Client) error {
			var paths []string

			if len(args) == 0 {
				paths = []string{"/"}
			} else {
				paths = args
			}

			stream, err := c.DiskUsage(ctx, &machineapi.DiskUsageRequest{
				RecursionDepth: recursionDepth,
				All:            all,
				Threshold:      threshold,
				Paths:          paths,
			})
			if err != nil {
				return fmt.Errorf("error fetching logs: %s", err)
			}

			addedHeader := false
			defaultNode := client.RemotePeer(stream.Context())

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)

			multipleNodes := false
			node := defaultNode

			stringifySize := func(s int64) string {
				if humanizeFlag {
					return humanize.Bytes(uint64(s))
				}

				return fmt.Sprintf("%d", s)
			}

			for {
				info, err := stream.Recv()
				if err != nil {
					if err == io.EOF || status.Code(err) == codes.Canceled {
						return w.Flush()
					}

					return fmt.Errorf("error streaming results: %s", err)
				}

				if info.Error != "" {
					fmt.Fprintf(os.Stderr, "%s: error reading file %s: %s\n", node, info.Name, info.Error)

					continue
				}

				pattern := "%s\t%s\n"

				size := stringifySize(info.Size)

				args := []interface{}{
					size, info.RelativeName,
				}

				if info.Metadata != nil && info.Metadata.Hostname != "" {
					multipleNodes = true
					node = info.Metadata.Hostname
				}

				if !addedHeader {
					if multipleNodes {
						fmt.Fprintln(w, "NODE\tSIZE\tNAME")
					} else {
						fmt.Fprintln(w, "SIZE\tNAME")
					}
					addedHeader = true
				}

				if info.Metadata != nil && info.Metadata.Error != "" {
					fmt.Fprintf(os.Stderr, "%s: %s\n", node, info.Metadata.Error)

					continue
				}

				if multipleNodes {
					pattern = "%s\t%s\t%s\n"
					args = append([]interface{}{node}, args...)
				}

				fmt.Fprintf(w, pattern, args...)
			}
		})
	},
}

func init() {
	duCmd.Flags().BoolVarP(&humanizeFlag, "humanize", "H", false, "humanize size and time in the output")
	duCmd.Flags().BoolVarP(&all, "all", "a", false, "write counts for all files, not just directories")
	duCmd.Flags().Int64VarP(&threshold, "threshold", "t", 0, "threshold exclude entries smaller than SIZE if positive, or entries greater than SIZE if negative")
	duCmd.Flags().Int32VarP(&recursionDepth, "depth", "d", 0, "maximum recursion depth")
	addCommand(duCmd)
}
