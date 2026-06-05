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

	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/client/multiplex"
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
		ctx := cmd.Context()

		clientFactory, err := NewClientFactory(ctx, &eventsCmdFlags)
		if err != nil {
			return err
		}

		defer clientFactory.Close() //nolint:errcheck

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

		responseChan := multiplex.StreamingViaFactory(
			ctx, clientFactory,
			func(ctx context.Context, c *client.Client) (machine.MachineService_EventsClient, error) {
				return c.Events(ctx, opts...)
			},
		)

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "NODE\tID\tEVENT\tACTOR\tSOURCE\tMESSAGE")

		const format = "%s\t%s\t%s\n%s\t%s\t%s\n"

		var errs error

		for resp := range responseChan {
			if resp.Err != nil {
				errs = errors.Join(errs, fmt.Errorf("error from node %s: %w", resp.Node, resp.Err))

				continue
			}

			event, err := client.UnmarshalEvent(resp.Payload)
			if err != nil {
				if _, ok := errors.AsType[client.EventNotSupportedError](err); ok { //nolint:errcheck // wrong linter error
					continue
				}

				errs = errors.Join(errs, fmt.Errorf("error from node %s: %w", resp.Node, err))

				continue
			}

			var eventArgs []any

			switch msg := event.Payload.(type) {
			case *machine.SequenceEvent:
				eventArgs = []any{msg.GetSequence()}
				if msg.Error != nil {
					eventArgs = append(eventArgs, "error:"+" "+msg.GetError().GetMessage())
				} else {
					eventArgs = append(eventArgs, msg.GetAction().String())
				}
			case *machine.PhaseEvent:
				eventArgs = []any{msg.GetPhase(), msg.GetAction().String()}
			case *machine.TaskEvent:
				eventArgs = []any{msg.GetTask(), msg.GetAction().String()}
			case *machine.ServiceStateEvent:
				eventArgs = []any{msg.GetService(), fmt.Sprintf("%s: %s", msg.GetAction(), msg.GetMessage())}
			case *machine.ConfigLoadErrorEvent:
				eventArgs = []any{"error", msg.GetError()}
			case *machine.ConfigValidationErrorEvent:
				eventArgs = []any{"error", msg.GetError()}
			case *machine.AddressEvent:
				eventArgs = []any{msg.GetHostname(), fmt.Sprintf("ADDRESSES: %s", strings.Join(msg.GetAddresses(), ","))}
			case *machine.MachineStatusEvent:
				eventArgs = []any{
					msg.GetStage().String(),
					fmt.Sprintf(
						"ready: %v, unmet conditions: %v",
						msg.GetStatus().Ready,
						xslices.Map(
							msg.GetStatus().GetUnmetConditions(),
							func(c *machine.MachineStatusEvent_MachineStatus_UnmetCondition) string {
								return c.Name
							},
						),
					),
				}
			}

			eventArgs = append([]any{resp.Node, event.ID, event.TypeURL, event.ActorID}, eventArgs...)
			fmt.Fprintf(w, format, eventArgs...)

			if err := w.Flush(); err != nil {
				errs = errors.Join(errs, fmt.Errorf("error flushing output: %w", err))
			}
		}

		return errs
	},
}

func init() {
	addCommand(eventsCmd)
	eventsCmd.Flags().Int32Var(&eventsCmdFlags.tailEvents, "tail", 0, "show specified number of past events (use -1 to show full history, default is to show no history)")
	eventsCmd.Flags().DurationVar(&eventsCmdFlags.tailDuration, "duration", 0, "show events for the past duration interval (one second resolution, default is to show no history)")
	eventsCmd.Flags().StringVar(&eventsCmdFlags.tailID, "since", "", "show events after the specified event ID (default is to show no history)")
	eventsCmd.Flags().StringVar(&eventsCmdFlags.actorID, "actor-id", "", "filter events by the specified actor ID (default is no filter)")
}
