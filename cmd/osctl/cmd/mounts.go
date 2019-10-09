/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

// nolint: dupl,golint
package cmd

import (
	"fmt"
	"math"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	machineapi "github.com/talos-systems/talos/api/machine"
	"github.com/talos-systems/talos/cmd/osctl/pkg/client"
	"github.com/talos-systems/talos/cmd/osctl/pkg/helpers"
)

// mountsCmd represents the mounts command.
var mountsCmd = &cobra.Command{
	Use:     "mounts",
	Aliases: []string{"m"},
	Short:   "List mounts",
	Long:    ``,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) != 0 {
			helpers.Should(cmd.Usage())
			os.Exit(1)
		}

		setupClient(func(c *client.Client) {
			mountsRender(c.Mounts(globalCtx))
		})
	},
}

func mountsRender(reply *machineapi.MountsReply, err error) {
	if reply == nil {
		if err != nil {
			helpers.Fatalf("error getting mounts: %s", err)
		}

		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "FILESYSTEM\tSIZE(GB)\tUSED(GB)\tAVAILABLE(GB)\tPERCENT USED\tMOUNTED ON")

	for _, r := range reply.Stats {
		percentAvailable := 100.0 - 100.0*(float64(r.Available)/float64(r.Size))

		if math.IsNaN(percentAvailable) {
			continue
		}

		fmt.Fprintf(w, "%s\t%.02f\t%.02f\t%.02f\t%.02f%%\t%s\n", r.Filesystem, float64(r.Size)*1e-9, float64(r.Size-r.Available)*1e-9, float64(r.Available)*1e-9, percentAvailable, r.MountedOn)
	}

	helpers.Should(w.Flush())
}

func init() {
	rootCmd.AddCommand(mountsCmd)
}
