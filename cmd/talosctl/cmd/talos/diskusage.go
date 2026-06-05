// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strconv"
	"text/tabwriter"

	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"

	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/client/multiplex"
)

var duCmdFlags struct {
	humanize  bool
	all       bool
	threshold int64
	depth     int32
}

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

		for _, path := range completePathFromNode(cmd.Context(), &duCmdFlags, toComplete) {
			if path[len(path)-1:] == "/" {
				completeOnlyPaths = append(completeOnlyPaths, path)
			}
		}

		return completeOnlyPaths, cobra.ShellCompDirectiveNoFileComp
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		clientFactory, err := NewClientFactory(ctx, &duCmdFlags)
		if err != nil {
			return err
		}

		defer clientFactory.Close() //nolint:errcheck

		var paths []string

		if len(args) == 0 {
			paths = []string{"/"}
		} else {
			paths = args
		}

		multipleNodes := len(clientFactory.Nodes()) > 1

		responseChan := multiplex.StreamingViaFactory(
			ctx, clientFactory,
			func(ctx context.Context, c *client.Client) (machineapi.MachineService_DiskUsageClient, error) {
				return c.DiskUsage(ctx, &machineapi.DiskUsageRequest{
					RecursionDepth: duCmdFlags.depth + 1,
					All:            duCmdFlags.all,
					Threshold:      duCmdFlags.threshold,
					Paths:          paths,
				})
			},
		)

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		defer w.Flush() //nolint:errcheck

		stringifySize := func(s int64) string {
			if duCmdFlags.humanize {
				return humanize.Bytes(uint64(s))
			}

			return strconv.FormatInt(s, 10)
		}

		var (
			errs          error
			headerWritten bool
		)

		for resp := range responseChan {
			if resp.Err != nil {
				errs = errors.Join(errs, fmt.Errorf("error from node %s: %w", resp.Node, resp.Err))

				continue
			}

			info := resp.Payload

			if info.Error != "" {
				errs = errors.Join(errs, fmt.Errorf("error from node %s: %s", resp.Node, info.Error))

				continue
			}

			if !headerWritten {
				if multipleNodes {
					fmt.Fprintln(w, "NODE\tSIZE\tNAME")
				} else {
					fmt.Fprintln(w, "SIZE\tNAME")
				}

				headerWritten = true
			}

			pattern := "%s\t%s\n"

			outputArgs := []any{
				stringifySize(info.Size), info.RelativeName,
			}

			if multipleNodes {
				pattern = "%s\t%s\t%s\n"
				outputArgs = slices.Insert(outputArgs, 0, any(resp.Node))
			}

			fmt.Fprintf(w, pattern, outputArgs...)
		}

		return errs
	},
}

func init() {
	duCmd.Flags().BoolVarP(&duCmdFlags.humanize, "humanize", "H", false, "humanize size and time in the output")
	duCmd.Flags().BoolVarP(&duCmdFlags.all, "all", "a", false, "write counts for all files, not just directories")
	duCmd.Flags().Int64VarP(&duCmdFlags.threshold, "threshold", "t", 0, "threshold exclude entries smaller than SIZE if positive, or entries greater than SIZE if negative")
	duCmd.Flags().Int32VarP(&duCmdFlags.depth, "depth", "d", 0, "maximum recursion depth")
	addCommand(duCmd)
}
