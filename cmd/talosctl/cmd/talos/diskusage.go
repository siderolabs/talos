// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"text/tabwriter"

	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"

	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/helpers"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
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
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) != 0 {
			return nil, cobra.ShellCompDirectiveError | cobra.ShellCompDirectiveNoFileComp
		}

		var completeOnlyPaths []string

		for _, path := range completePathFromNode(toComplete) {
			if path[len(path)-1:] == "/" {
				completeOnlyPaths = append(completeOnlyPaths, path)
			}
		}

		return completeOnlyPaths, cobra.ShellCompDirectiveNoFileComp
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		return WithClient(func(ctx context.Context, c *client.Client) error {
			var paths []string

			if len(args) == 0 {
				paths = []string{"/"}
			} else {
				paths = args
			}

			stream, err := c.DiskUsage(ctx, &machineapi.DiskUsageRequest{
				RecursionDepth: recursionDepth + 1,
				All:            all,
				Threshold:      threshold,
				Paths:          paths,
			})
			if err != nil {
				return fmt.Errorf("error fetching disk usage: %s", err)
			}

			addedHeader := false

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)

			stringifySize := func(s int64) string {
				if humanizeFlag {
					return humanize.Bytes(uint64(s))
				}

				return strconv.FormatInt(s, 10)
			}

			defer w.Flush() //nolint:errcheck

			return helpers.ReadGRPCStream(stream, func(info *machineapi.DiskUsageInfo, node string, multipleNodes bool) error {
				if info.Error != "" {
					return helpers.NonFatalError(errors.New(info.Error))
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

				if multipleNodes {
					pattern = "%s\t%s\t%s\n"
					args = append([]interface{}{node}, args...)
				}

				fmt.Fprintf(w, pattern, args...)

				return nil
			})
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
