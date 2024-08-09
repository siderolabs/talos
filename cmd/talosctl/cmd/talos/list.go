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
	"strings"
	"text/tabwriter"
	"time"

	humanize "github.com/dustin/go-humanize"
	"github.com/spf13/cobra"

	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/helpers"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
)

const sixMonths = 6 * time.Hour * 24 * 30

var (
	long           bool
	recurse        bool
	recursionDepth int32
	humanizeFlag   bool
	types          []string
)

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

		return completePathFromNode(toComplete), cobra.ShellCompDirectiveNoFileComp
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		if recurse && recursionDepth != 1 {
			return errors.New("only one of flags --recurse and --depth can be specified at the same time")
		}

		return WithClient(func(ctx context.Context, c *client.Client) error {
			rootDir := "/"

			if len(args) > 0 {
				rootDir = args[0]
			}

			// handle all variants: --type=f,l; -tfl; etc
			var reqTypes []machineapi.ListRequest_Type
			for _, typ := range types {
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

			if recurse {
				recursionDepth = -1
			}

			stream, err := c.LS(ctx, &machineapi.ListRequest{
				Root:           rootDir,
				Recurse:        recursionDepth > 1 || recurse,
				RecursionDepth: recursionDepth,
				Types:          reqTypes,
				ReportXattrs:   long,
			})
			if err != nil {
				return fmt.Errorf("error fetching logs: %s", err)
			}

			if !long {
				w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
				fmt.Fprintln(w, "NODE\tNAME")

				defer w.Flush() //nolint:errcheck

				return helpers.ReadGRPCStream(stream, func(info *machineapi.FileInfo, node string, multipleNodes bool) error {
					if info.Error != "" {
						return helpers.NonFatalError(fmt.Errorf("%s: error reading file %s: %s", node, info.Name, info.Error))
					}

					if !multipleNodes {
						fmt.Println(info.RelativeName)
					} else {
						fmt.Fprintf(w, "%s\t%s\n",
							node,
							info.RelativeName,
						)
					}

					return nil
				})
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
			defer w.Flush() //nolint:errcheck

			fmt.Fprintln(w, "NODE\tMODE\tUID\tGID\tSIZE(B)\tLASTMOD\tLABEL\tNAME")

			return helpers.ReadGRPCStream(stream, func(info *machineapi.FileInfo, node string, multipleNodes bool) error {
				if info.Error != "" {
					return helpers.NonFatalError(fmt.Errorf("%s: error reading file %s: %s", node, info.Name, info.Error))
				}

				display := info.RelativeName
				if info.Link != "" {
					display += " -> " + info.Link
				}

				size := strconv.FormatInt(info.Size, 10)

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

				label := ""
				if info.Xattrs != nil {
					for _, l := range info.Xattrs {
						if l.Name == "security.selinux" {
							label = string(l.Data)

							break
						}
					}
				}

				fmt.Fprintf(w, "%s\t%s\t%d\t%d\t%s\t%s\t%s\t%s\n",
					node,
					os.FileMode(info.Mode).String(),
					info.Uid,
					info.Gid,
					size,
					timestampFormatted,
					label,
					display,
				)

				return nil
			})
		})
	},
}

func init() {
	typesHelp := strings.Join([]string{
		"filter by specified types:",
		"f" + "\t" + "regular file",
		"d" + "\t" + "directory",
		"l, L" + "\t" + "symbolic link",
	}, "\n")

	lsCmd.Flags().BoolVarP(&long, "long", "l", false, "display additional file details")
	lsCmd.Flags().BoolVarP(&recurse, "recurse", "r", false, "recurse into subdirectories")
	lsCmd.Flags().BoolVarP(&humanizeFlag, "humanize", "H", false, "humanize size and time in the output")
	lsCmd.Flags().Int32VarP(&recursionDepth, "depth", "d", 1, "maximum recursion depth")
	lsCmd.Flags().StringSliceVarP(&types, "type", "t", nil, typesHelp)
	addCommand(lsCmd)
}
