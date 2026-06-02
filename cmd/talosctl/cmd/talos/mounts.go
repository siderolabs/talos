// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/client/multiplex"
)

// mountsCmd represents the mounts command.
var mountsCmd = &cobra.Command{
	Use:     "mounts",
	Aliases: []string{"mount"},
	Short:   "List mounts",
	Long:    ``,
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		clientFactory, err := NewClientFactory(ctx, nil)
		if err != nil {
			return err
		}

		defer clientFactory.Close() //nolint:errcheck

		responseChan := multiplex.UnaryViaFactory(
			ctx, clientFactory,
			func(ctx context.Context, c *client.Client) (*machineapi.MountsResponse, error) {
				return c.Mounts(ctx)
			},
		)

		return renderMounts(os.Stdout, responseChan)
	},
}

// renderMounts renders the mounts output for a stream of multiplexed responses.
func renderMounts(output io.Writer, responseChan <-chan multiplex.Response[*machineapi.MountsResponse]) error {
	w := tabwriter.NewWriter(output, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "NODE\tFILESYSTEM\tSIZE(GB)\tUSED(GB)\tAVAILABLE(GB)\tPERCENT USED\tMOUNTED ON")

	flushTimer := time.NewTimer(outputFlushInterval)
	defer flushTimer.Stop()

	flushTimer.Stop()

	var errs error

	for {
		select {
		case resp, ok := <-responseChan:
			if !ok {
				return errors.Join(errs, w.Flush())
			}

			if resp.Err != nil {
				errs = errors.Join(errs, fmt.Errorf("error from node %s: %w", resp.Node, resp.Err))
			} else {
				for _, msg := range resp.Payload.Messages {
					for _, r := range msg.Stats {
						percentAvailable := 100.0 - 100.0*(float64(r.Available)/float64(r.Size))

						if math.IsNaN(percentAvailable) {
							continue
						}

						fmt.Fprintf(
							w, "%s\t%s\t%.02f\t%.02f\t%.02f\t%.02f%%\t%s\n",
							resp.Node,
							r.Filesystem,
							float64(r.Size)*1e-9,
							float64(r.Size-r.Available)*1e-9,
							float64(r.Available)*1e-9,
							percentAvailable,
							r.MountedOn,
						)
					}
				}
			}

			flushTimer.Reset(outputFlushInterval)
		case <-flushTimer.C:
			if err := w.Flush(); err != nil {
				errs = errors.Join(errs, fmt.Errorf("error flushing output: %w", err))
			}
		}
	}
}

func init() {
	addCommand(mountsCmd)
}
