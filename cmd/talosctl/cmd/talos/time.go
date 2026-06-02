// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"context"
	"errors"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	timeapi "github.com/siderolabs/talos/pkg/machinery/api/time"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/client/multiplex"
)

var timeCmdFlags struct {
	ntpServer string
}

// timeCmd represents the time command.
var timeCmd = &cobra.Command{
	Use:   "time [--check server]",
	Short: "Gets current server time",
	Long:  ``,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		clientFactory, err := NewClientFactory(ctx, &timeCmdFlags)
		if err != nil {
			return err
		}

		defer clientFactory.Close() //nolint:errcheck

		responseChan := multiplex.UnaryViaFactory(
			ctx, clientFactory,
			func(ctx context.Context, c *client.Client) (*timeapi.TimeResponse, error) {
				if timeCmdFlags.ntpServer == "" {
					return c.Time(ctx)
				}

				return c.TimeCheck(ctx, timeCmdFlags.ntpServer)
			},
		)

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "NODE\tNTP-SERVER\tNODE-TIME\tNTP-SERVER-TIME")

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
						if !msg.Localtime.IsValid() {
							errs = errors.Join(errs, fmt.Errorf("node %s: error parsing local time", resp.Node))

							continue
						}

						if !msg.Remotetime.IsValid() {
							errs = errors.Join(errs, fmt.Errorf("node %s: error parsing remote time", resp.Node))

							continue
						}

						fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", resp.Node, msg.Server, msg.Localtime.AsTime().String(), msg.Remotetime.AsTime().String())
					}
				}

				flushTimer.Reset(outputFlushInterval)
			case <-flushTimer.C:
				if err := w.Flush(); err != nil {
					errs = errors.Join(errs, fmt.Errorf("error flushing output: %w", err))
				}
			}
		}
	},
}

func init() {
	timeCmd.Flags().StringVar(&timeCmdFlags.ntpServer, "check", "", "checks server time against specified ntp server")
	addCommand(timeCmd)
}
