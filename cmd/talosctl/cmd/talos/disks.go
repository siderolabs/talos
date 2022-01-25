// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"context"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	humanize "github.com/dustin/go-humanize"
	"github.com/spf13/cobra"

	"github.com/talos-systems/talos/pkg/cli"
	"github.com/talos-systems/talos/pkg/machinery/client"
)

var disksCmdFlags struct {
	insecure bool
}

var disksCmd = &cobra.Command{
	Use:   "disks",
	Short: "Get the list of disks from /sys/block on the machine",
	Long:  ``,
	RunE: func(cmd *cobra.Command, args []string) error {
		if disksCmdFlags.insecure {
			return WithClientMaintenance(nil, printDisks)
		}

		return WithClient(printDisks)
	},
}

//nolint:gocyclo
func printDisks(ctx context.Context, c *client.Client) error {
	response, err := c.Disks(ctx)
	if err != nil {
		if response == nil {
			return fmt.Errorf("error getting disks: %w", err)
		}

		cli.Warning("%s", err)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	node := ""

	labels := strings.Join(
		[]string{
			"DEV",
			"MODEL",
			"SERIAL",
			"TYPE",
			"UUID",
			"WWID",
			"MODALIAS",
			"NAME",
			"SIZE",
			"BUS_PATH",
		}, "\t")

	getWithPlaceholder := func(in string) string {
		if in == "" {
			return "-"
		}

		return in
	}

	for i, message := range response.Messages {
		if message.Metadata != nil && message.Metadata.Hostname != "" {
			node = message.Metadata.Hostname
		}

		if len(message.Disks) == 0 {
			continue
		}

		for j, disk := range message.Disks {
			if i == 0 && j == 0 {
				if node != "" {
					fmt.Fprintln(w, "NODE\t"+labels)
				} else {
					fmt.Fprintln(w, labels)
				}
			}

			args := []interface{}{}

			if node != "" {
				args = append(args, node)
			}

			args = append(args, []interface{}{
				getWithPlaceholder(disk.DeviceName),
				getWithPlaceholder(disk.Model),
				getWithPlaceholder(disk.Serial),
				disk.Type.String(),
				getWithPlaceholder(disk.Uuid),
				getWithPlaceholder(disk.Wwid),
				getWithPlaceholder(disk.Modalias),
				getWithPlaceholder(disk.Name),
				humanize.Bytes(disk.Size),
				getWithPlaceholder(disk.BusPath),
			}...)

			pattern := strings.Repeat("%s\t", len(args))
			pattern = strings.TrimSpace(pattern) + "\n"

			fmt.Fprintf(w, pattern, args...)
		}
	}

	return w.Flush()
}

func init() {
	disksCmd.Flags().BoolVarP(&disksCmdFlags.insecure, "insecure", "i", false, "get disks using the insecure (encrypted with no auth) maintenance service")
	addCommand(disksCmd)
}
