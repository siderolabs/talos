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

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"

	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/client/multiplex"
	blockres "github.com/siderolabs/talos/pkg/machinery/resources/block"
)

var dfCmdFlags struct {
	bytes  bool
	inodes bool
}

// dfRow is a single line of output: the mount it describes, plus its statfs result.
type dfRow struct {
	volume string
	source string
	target string
	statfs *machine.StorageServiceStatfsResponse
}

// dfCmd represents the df command.
var dfCmd = &cobra.Command{
	Use:     "diskfree [volume ID]",
	Aliases: []string{"df"},
	Short:   "Retrieve disk usage information for mounted volumes",
	Long: `Reports storage and inode usage for the volumes Talos has mounted.

Only volumes backed by a filesystem of their own are listed. Directory, symlink and
overlay volumes are omitted, since their usage is already accounted for by the volume
they sit on top of.

This is a per-volume view rather than the node's full mount table: use
'talosctl mounts' to see every mount the kernel reports.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		clientFactory, err := NewClientFactory(ctx, &dfCmdFlags)
		if err != nil {
			return err
		}

		defer clientFactory.Close() //nolint:errcheck

		var volumeID string

		if len(args) > 0 {
			volumeID = args[0]
		}

		responseChan := multiplex.UnaryViaFactory(
			ctx, clientFactory,
			func(ctx context.Context, c *client.Client) ([]dfRow, error) {
				return collectDiskFree(ctx, c, volumeID)
			},
		)

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

			if !headerWritten {
				if dfCmdFlags.inodes {
					fmt.Fprintln(w, "NODE\tVOLUME\tFILESYSTEM\tINODES\tIUSED\tIFREE\t%IUSED\tMOUNT POINT")
				} else {
					fmt.Fprintln(w, "NODE\tVOLUME\tFILESYSTEM\tSIZE\tUSED\tAVAILABLE\tUSAGE\tMOUNT POINT")
				}

				headerWritten = true
			}

			for _, row := range resp.Payload {
				printDiskFreeRow(w, resp.Node, row)
			}
		}

		return errs
	},
}

// collectDiskFree resolves which filesystems to report on from the mount status resources,
// then asks the node for the statfs result of each one.
//
// When volumeID is set, only that volume is reported, and it is an error if it is not mounted.
//
//nolint:gocyclo
func collectDiskFree(ctx context.Context, c *client.Client, volumeID string) ([]dfRow, error) {
	if volumeID != "" {
		vs, err := safe.StateGetByID[*blockres.VolumeStatus](ctx, c.COSI, volumeID)
		if err != nil {
			if state.IsNotFoundError(err) {
				return nil, fmt.Errorf("volume %q not found", volumeID)
			}

			return nil, fmt.Errorf("error looking up volume %q: %w", volumeID, err)
		}

		if vs.TypedSpec().Phase != blockres.VolumePhaseReady {
			return nil, fmt.Errorf("volume %q is not ready (phase: %s)", volumeID, vs.TypedSpec().Phase)
		}

		switch vs.TypedSpec().Type {
		case blockres.VolumeTypeDirectory, blockres.VolumeTypeSymlink, blockres.VolumeTypeOverlay:
			return nil, fmt.Errorf("volume %q is a %s volume, which does not have its own filesystem", volumeID, vs.TypedSpec().Type)
		case blockres.VolumeTypePartition, blockres.VolumeTypeDisk, blockres.VolumeTypeTmpfs, blockres.VolumeTypeExternal:
			// these are all fine
		}

		if vs.TypedSpec().Filesystem == blockres.FilesystemTypeSwap {
			return nil, fmt.Errorf("volume %q is a swap volume, which does not have a filesystem", volumeID)
		}
	}

	mountStatuses, err := safe.StateListAll[*blockres.MountStatus](ctx, c.COSI)
	if err != nil {
		return nil, err
	}

	var rows []dfRow

	for ms := range mountStatuses.All() {
		spec := ms.TypedSpec()

		// mounts without a source device aren't backed by a filesystem of their own
		if spec.Source == "" {
			continue
		}

		if spec.Detached {
			continue
		}

		if volumeID != "" && spec.Spec.VolumeID != volumeID {
			continue
		}

		rows = append(rows, dfRow{
			volume: spec.Spec.VolumeID,
			source: spec.Source,
			target: spec.Target,
		})
	}

	if volumeID != "" && len(rows) == 0 {
		return nil, fmt.Errorf("volume %q not mounted", volumeID)
	}

	for i, row := range rows {
		statfs, err := c.MachineStorageClient.Statfs(ctx, &machine.StorageServiceStatfsRequest{
			Path: row.target,
		})
		if err != nil {
			return nil, fmt.Errorf("error calling statfs on %q: %w", row.target, err)
		}

		rows[i].statfs = statfs
	}

	return rows, nil
}

func printDiskFreeRow(w *tabwriter.Writer, node string, row dfRow) {
	info := row.statfs

	if dfCmdFlags.inodes {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			node, row.volume, row.source,
			stringifyCount(info.Inodes), stringifyCount(info.InodesUsed), stringifyCount(info.InodesFree),
			percent(info.InodesUsed, info.Inodes), row.target,
		)
	} else {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			node, row.volume, row.source,
			stringifySize(info.Size), stringifySize(info.Used), stringifySize(info.Available),
			percent(info.Used, info.Used+info.Available), row.target,
		)
	}
}

// stringifySize formats a byte count, humanizing (powers of 1024) unless --bytes is set.
func stringifySize(s uint64) string {
	if dfCmdFlags.bytes {
		return strconv.FormatUint(s, 10)
	}

	// humanize.IBytes returns e.g. "960 MiB"; drop the space to match df's compact form.
	return strings.ReplaceAll(humanize.IBytes(s), " ", "")
}

// stringifyCount formats an inode count, humanizing with metric (K/M/G) suffixes unless --bytes is set.
func stringifyCount(v uint64) string {
	if dfCmdFlags.bytes {
		return strconv.FormatUint(v, 10)
	}

	mantissa, prefix := humanize.ComputeSI(float64(v))
	if prefix == "" {
		return strconv.FormatUint(v, 10)
	}

	return humanize.FtoaWithDigits(mantissa, 1) + strings.ToUpper(prefix)
}

func percent(used, total uint64) string {
	if total == 0 {
		return "-"
	}

	pct := used * 100 / total
	if used*100%total != 0 {
		pct++
	}

	return fmt.Sprintf("%d%%", pct)
}

func init() {
	dfCmd.Flags().BoolVarP(&dfCmdFlags.bytes, "bytes", "b", false, "print sizes in bytes rather than in human-readable format")
	dfCmd.Flags().BoolVarP(&dfCmdFlags.inodes, "inodes", "i", false, "list inode information instead of block usage")
	addCommand(dfCmd)
}
