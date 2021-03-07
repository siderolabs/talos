// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/talos-systems/talos/pkg/machinery/api/machine"
	"github.com/talos-systems/talos/pkg/machinery/client"
)

var eventsCmdFlags struct {
	tailEvents   int32
	tailDuration time.Duration
	tailID       string
}

// eventsCmd represents the events command.
var eventsCmd = &cobra.Command{
	Use:   "events",
	Short: "Stream runtime events",
	Long:  ``,
	RunE: func(cmd *cobra.Command, args []string) error {
		return WithClient(func(ctx context.Context, c *client.Client) error {
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
			fmt.Fprintln(w, "NODE\tID\tEVENT\tSOURCE\tMESSAGE")

			opts := []client.EventsOptionFunc{}

			if eventsCmdFlags.tailEvents != 0 {
				opts = append(opts, client.WithTailEvents(eventsCmdFlags.tailEvents))
			}

			if eventsCmdFlags.tailDuration != 0 {
				opts = append(opts, client.WithTailDuration(eventsCmdFlags.tailDuration))
			}

			if eventsCmdFlags.tailID != "" {
				opts = append(opts, client.WithTailID(eventsCmdFlags.tailID))
			}

			return c.EventsWatch(ctx, func(ch <-chan client.Event) {
				for {
					var (
						event client.Event
						ok    bool
					)

					select {
					case event, ok = <-ch:
						if !ok {
							return
						}
					case <-ctx.Done():
						return
					}

					format := "%s\t%s\t%s\t%s\t%s\n"

					var args []interface{}

					switch msg := event.Payload.(type) {
					case *machine.SequenceEvent:
						args = []interface{}{msg.GetSequence()}
						if msg.Error != nil {
							args = append(args, "error:"+" "+msg.GetError().GetMessage())
						} else {
							args = append(args, msg.GetAction().String())
						}
					case *machine.PhaseEvent:
						args = []interface{}{msg.GetPhase(), msg.GetAction().String()}
					case *machine.TaskEvent:
						args = []interface{}{msg.GetTask(), msg.GetAction().String()}
					case *machine.ServiceStateEvent:
						args = []interface{}{msg.GetService(), fmt.Sprintf("%s: %s", msg.GetAction(), msg.GetMessage())}
					default:
						// We haven't implemented the handling of this event yet.
						continue
					}

					args = append([]interface{}{event.Node, event.ID, event.TypeURL}, args...)
					fmt.Fprintf(w, format, args...)

					//nolint:errcheck
					w.Flush()
				}
			}, opts...)
		})
	},
}

func init() {
	addCommand(eventsCmd)
	eventsCmd.Flags().Int32Var(&eventsCmdFlags.tailEvents, "tail", 0, "show specified number of past events (use -1 to show full history, default is to show no history)")
	eventsCmd.Flags().DurationVar(&eventsCmdFlags.tailDuration, "duration", 0, "show events for the past duration interval (one second resolution, default is to show no history)")
	eventsCmd.Flags().StringVar(&eventsCmdFlags.tailID, "since", "", "show events after the specified event ID (default is to show no history)")
}
