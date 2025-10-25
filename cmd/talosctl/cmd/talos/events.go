// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/siderolabs/gen/xslices"
	"github.com/spf13/cobra"

	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/helpers"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
)

var eventsCmdFlags struct {
	tailEvents   int32
	tailDuration time.Duration
	tailID       string
	actorID      string
}

// eventsCmd represents the events command.
var eventsCmd = &cobra.Command{
	Use:   "events",
	Short: "Stream runtime events",
	Long:  ``,
	RunE: func(cmd *cobra.Command, args []string) error {
		return WithClient(func(ctx context.Context, c *client.Client) error {
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
			fmt.Fprintln(w, "NODE\tID\tEVENT\tACTOR\tSOURCE\tMESSAGE")

			var opts []client.EventsOptionFunc

			if eventsCmdFlags.tailEvents != 0 {
				opts = append(opts, client.WithTailEvents(eventsCmdFlags.tailEvents))
			}

			if eventsCmdFlags.tailDuration != 0 {
				opts = append(opts, client.WithTailDuration(eventsCmdFlags.tailDuration))
			}

			if eventsCmdFlags.tailID != "" {
				opts = append(opts, client.WithTailID(eventsCmdFlags.tailID))
			}

			if eventsCmdFlags.actorID != "" {
				opts = append(opts, client.WithActorID(eventsCmdFlags.actorID))
			}

			events, err := c.Events(ctx, opts...)
			if err != nil {
				return err
			}

			return helpers.ReadGRPCStream(events, func(ev *machine.Event, node string, multipleNodes bool) error {
				format := "%s\t%s\t%s\n%s\t%s\t%s\n"

				event, err := client.UnmarshalEvent(ev)
				if err != nil {
					var errBadEvent client.EventNotSupportedError
					if errors.As(err, &errBadEvent) {
						return nil
					}

					return err
				}

				var args []any

				switch msg := event.Payload.(type) {
				case *machine.SequenceEvent:
					args = []any{msg.GetSequence()}
					if msg.Error != nil {
						args = append(args, "error:"+" "+msg.GetError().GetMessage())
					} else {
						args = append(args, msg.GetAction().String())
					}
				case *machine.PhaseEvent:
					args = []any{msg.GetPhase(), msg.GetAction().String()}
				case *machine.TaskEvent:
					args = []any{msg.GetTask(), msg.GetAction().String()}
				case *machine.ServiceStateEvent:
					args = []any{msg.GetService(), fmt.Sprintf("%s: %s", msg.GetAction(), msg.GetMessage())}
				case *machine.ConfigLoadErrorEvent:
					args = []any{"error", msg.GetError()}
				case *machine.ConfigValidationErrorEvent:
					args = []any{"error", msg.GetError()}
				case *machine.AddressEvent:
					args = []any{msg.GetHostname(), fmt.Sprintf("ADDRESSES: %s", strings.Join(msg.GetAddresses(), ","))}
				case *machine.MachineStatusEvent:
					args = []any{
						msg.GetStage().String(),
						fmt.Sprintf("ready: %v, unmet conditions: %v",
							msg.GetStatus().Ready,
							xslices.Map(msg.GetStatus().GetUnmetConditions(),
								func(c *machine.MachineStatusEvent_MachineStatus_UnmetCondition) string {
									return c.Name
								},
							),
						),
					}
				}

				args = append([]any{event.Node, event.ID, event.TypeURL, event.ActorID}, args...)
				fmt.Fprintf(w, format, args...)

				return w.Flush()
			})
		})
	},
}

func init() {
	addCommand(eventsCmd)
	eventsCmd.Flags().Int32Var(&eventsCmdFlags.tailEvents, "tail", 0, "show specified number of past events (use -1 to show full history, default is to show no history)")
	eventsCmd.Flags().DurationVar(&eventsCmdFlags.tailDuration, "duration", 0, "show events for the past duration interval (one second resolution, default is to show no history)")
	eventsCmd.Flags().StringVar(&eventsCmdFlags.tailID, "since", "", "show events after the specified event ID (default is to show no history)")
	eventsCmd.Flags().StringVar(&eventsCmdFlags.actorID, "actor-id", "", "filter events by the specified actor ID (default is no filter)")
}
