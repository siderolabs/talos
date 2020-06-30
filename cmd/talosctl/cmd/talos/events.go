// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/talos-systems/talos/api/machine"
	"github.com/talos-systems/talos/pkg/client"
)

// eventsCmd represents the events command.
var eventsCmd = &cobra.Command{
	Use:   "events",
	Short: "Stream runtime events",
	Long:  ``,
	RunE: func(cmd *cobra.Command, args []string) error {
		return WithClient(func(ctx context.Context, c *client.Client) error {
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
			fmt.Fprintln(w, "NODE\tEVENT\tMESSAGE")

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

					format := "%s\t%s\t%s\n"

					var args []interface{}

					switch msg := event.Payload.(type) {
					case *machine.SequenceEvent:
						if msg.Error != nil {
							args = []interface{}{msg.GetSequence() + " error:" + " " + msg.GetError().GetMessage()}
						} else {
							args = []interface{}{msg.GetSequence() + " " + msg.GetAction().String()}
						}
					default:
						// We haven't implemented the handling of this event yet.
						continue
					}

					args = append([]interface{}{event.Node, event.TypeURL}, args...)
					fmt.Fprintf(w, format, args...)

					// nolint: errcheck
					w.Flush()
				}
			})
		})
	},
}

func init() {
	addCommand(eventsCmd)
}
