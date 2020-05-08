// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"context"
	"fmt"
	"io"
	"log"

	"github.com/golang/protobuf/proto"
	"github.com/spf13/cobra"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/talos-systems/talos/api/machine"
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

			for {
				event, err := stream.Recv()
				if err != nil {
					if err == io.EOF || status.Code(err) == codes.Canceled {
						return nil
					}

					return err
				}

				msg := &machine.SequenceEvent{}
				proto.Unmarshal(event.Event.GetData().GetValue(), msg)
				log.Println(event.GetEvent().GetData().GetTypeUrl())
				log.Println(msg.GetType().String())
				log.Println(msg.GetSequence())
			}
		})
	},
}

func init() {
	addCommand(eventsCmd)
}
