// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	humanize "github.com/dustin/go-humanize"
	"github.com/spf13/cobra"

	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/client/multiplex"
)

const sixMonths = 6 * time.Hour * 24 * 30

var lsCmdFlags struct {
	long     bool
	recurse  bool
	depth    int32
	humanize bool
	types    []string
}

// lsCmd represents the ls command.
var lsCmd = &cobra.Command{
	Use:     "list [path]",
	Aliases: []string{"ls"},
	Short:   "Retrieve a directory listing",
	Long:    ``,
	Args:    cobra.MaximumNArgs(1),
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) != 0 {
			return nil, cobra.ShellCompDirectiveError | cobra.ShellCompDirectiveNoFileComp
		}

		return completePathFromNode(cmd.Context(), &lsCmdFlags, toComplete), cobra.ShellCompDirectiveNoFileComp
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		if lsCmdFlags.recurse && lsCmdFlags.depth != 1 {
			return errors.New("only one of flags --recurse and --depth can be specified at the same time")
		}

		ctx := cmd.Context()

		clientFactory, err := NewClientFactory(ctx, &lsCmdFlags)
		if err != nil {
			return err
		}

		defer clientFactory.Close() //nolint:errcheck

		rootDir := "/"

		if len(args) > 0 {
			rootDir = args[0]
		}

		// handle all variants: --type=f,l; -tfl; etc
		var reqTypes []machineapi.ListRequest_Type

		for _, typ := range lsCmdFlags.types {
			for _, t := range typ {
				// handle both `find -type X` and os.FileMode.String() designations
				switch t {
				case 'f':
					reqTypes = append(reqTypes, machineapi.ListRequest_REGULAR)
				case 'd':
					reqTypes = append(reqTypes, machineapi.ListRequest_DIRECTORY)
				case 'l', 'L':
					reqTypes = append(reqTypes, machineapi.ListRequest_SYMLINK)
				default:
					return fmt.Errorf("invalid file type: %s", string(t))
				}
			}
		}

		recursionDepth := lsCmdFlags.depth

		if lsCmdFlags.recurse {
			recursionDepth = -1
		}

		multipleNodes := len(clientFactory.Nodes()) > 1

		responseChan := multiplex.StreamingViaFactory(
			ctx, clientFactory,
			func(ctx context.Context, c *client.Client) (machineapi.MachineService_ListClient, error) {
				return c.LS(ctx, &machineapi.ListRequest{
					Root:           rootDir,
					Recurse:        recursionDepth > 1 || lsCmdFlags.recurse,
					RecursionDepth: recursionDepth,
					Types:          reqTypes,
					ReportXattrs:   lsCmdFlags.long,
				})
			},
		)

		if !lsCmdFlags.long {
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
			defer w.Flush() //nolint:errcheck

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
					errs = errors.Join(errs, fmt.Errorf("%s: error reading file %s: %s", resp.Node, info.Name, info.Error))

					continue
				}

				if !multipleNodes {
					fmt.Println(info.RelativeName)

					continue
				}

				if !headerWritten {
					fmt.Fprintln(w, "NODE\tNAME")

					headerWritten = true
				}

				fmt.Fprintf(
					w, "%s\t%s\n",
					resp.Node,
					info.RelativeName,
				)
			}

			return errs
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		defer w.Flush() //nolint:errcheck

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
				errs = errors.Join(errs, fmt.Errorf("%s: error reading file %s: %s", resp.Node, info.Name, info.Error))

				continue
			}

			if !headerWritten {
				fmt.Fprintln(w, "NODE\tMODE\tUID\tGID\tSIZE(B)\tLASTMOD\tLABEL\tNAME")

				headerWritten = true
			}

			display := info.RelativeName
			if info.Link != "" {
				display += " -> " + info.Link
			}

			size := strconv.FormatInt(info.Size, 10)

			if lsCmdFlags.humanize {
				size = humanize.Bytes(uint64(info.Size))
			}

			timestamp := time.Unix(info.Modified, 0)
			timestampFormatted := ""

			if lsCmdFlags.humanize {
				timestampFormatted = humanize.Time(timestamp)
			} else {
				if time.Since(timestamp) < sixMonths {
					timestampFormatted = timestamp.Format("Jan _2 15:04:05")
				} else {
					timestampFormatted = timestamp.Format("Jan _2 2006 15:04")
				}
			}

			label := ""

			if info.Xattrs != nil {
				for _, l := range info.Xattrs {
					if l.Name == "security.selinux" {
						label = string(bytes.Trim(l.Data, "\x00\n"))

						break
					}
				}
			}

			fmt.Fprintf(
				w, "%s\t%s\t%d\t%d\t%s\t%s\t%s\t%s\n",
				resp.Node,
				os.FileMode(info.Mode).String(),
				info.Uid,
				info.Gid,
				size,
				timestampFormatted,
				label,
				display,
			)
		}

		return errs
	},
}

func init() {
	typesHelp := strings.Join([]string{
		"filter by specified types:",
		"f" + "\t" + "regular file",
		"d" + "\t" + "directory",
		"l, L" + "\t" + "symbolic link",
	}, "\n")

	lsCmd.Flags().BoolVarP(&lsCmdFlags.long, "long", "l", false, "display additional file details")
	lsCmd.Flags().BoolVarP(&lsCmdFlags.recurse, "recurse", "r", false, "recurse into subdirectories")
	lsCmd.Flags().BoolVarP(&lsCmdFlags.humanize, "humanize", "H", false, "humanize size and time in the output")
	lsCmd.Flags().Int32VarP(&lsCmdFlags.depth, "depth", "d", 1, "maximum recursion depth")
	lsCmd.Flags().StringSliceVarP(&lsCmdFlags.types, "type", "t", nil, typesHelp)
	addCommand(lsCmd)
}
