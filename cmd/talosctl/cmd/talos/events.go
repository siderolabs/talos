// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"text/tabwriter"

	"github.com/golang/protobuf/proto"
	"github.com/spf13/cobra"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/talos-systems/talos/api/machine"
	"github.com/talos-systems/talos/cmd/talosctl/pkg/talos/helpers"
	"github.com/talos-systems/talos/pkg/client"
)

// eventsCmd represents the events command
var eventsCmd = &cobra.Command{
	Use:   "events",
	Short: "Stream runtime events",
	Long:  ``,
	RunE: func(cmd *cobra.Command, args []string) error {
		return WithClient(func(ctx context.Context, c *client.Client) error {
			stream, err := c.Events(ctx)
			if err != nil {
				return fmt.Errorf("error fetching events: %s", err)
			}

			defaultNode := helpers.RemotePeer(stream.Context())

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
			fmt.Fprintln(w, "NODE\tEVENT\tMESSAGE")

			for {
				resp, err := stream.Recv()
				if err != nil {
					if err == io.EOF || status.Code(err) == codes.Canceled {
						return nil
					}

					return fmt.Errorf("failed to watch events: %w", err)
				}

				node := defaultNode

				for _, event := range resp.Messages {
					if event.Metadata != nil {
						node = event.Metadata.Hostname
					}

					typeURL := event.GetData().GetTypeUrl()

					format := "%s\t%s\t%s\n"

					var args []interface{}

					switch event.GetData().GetTypeUrl() {
					case "talos/runtime/" + proto.MessageName(&machine.SequenceEvent{}):
						msg := &machine.SequenceEvent{}

						if err = proto.Unmarshal(event.GetData().GetValue(), msg); err != nil {
							log.Printf("failed to unmarshal message: %v", err)
							continue
						}

						if msg.Error != nil {
							args = []interface{}{msg.GetSequence() + " error:" + " " + msg.GetError().GetMessage()}
						} else {
							args = []interface{}{msg.GetSequence() + " " + msg.GetAction().String()}
						}
					default:
						// We haven't implemented the handling of this event yet.
						continue
					}

					args = append([]interface{}{node, typeURL}, args...)
					fmt.Fprintf(w, format, args...)

					// nolint: errcheck
					w.Flush()
				}
			}
		})
	},
}

func init() {
	addCommand(eventsCmd)
}
