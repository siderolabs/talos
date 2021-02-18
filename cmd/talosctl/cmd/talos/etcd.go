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

	"github.com/spf13/cobra"

	"github.com/talos-systems/talos/pkg/cli"
	"github.com/talos-systems/talos/pkg/machinery/api/machine"
	"github.com/talos-systems/talos/pkg/machinery/client"
)

// etcdCmd represents the etcd command.
var etcdCmd = &cobra.Command{
	Use:   "etcd",
	Short: "Manage etcd",
	Long:  ``,
}

var etcdLeaveCmd = &cobra.Command{
	Use:   "leave",
	Short: "Tell nodes to leave etcd cluster",
	Long:  ``,
	RunE: func(cmd *cobra.Command, args []string) error {
		return WithClient(func(ctx context.Context, c *client.Client) error {
			return c.EtcdLeaveCluster(ctx, &machine.EtcdLeaveClusterRequest{})
		})
	},
}

var etcdForfeitLeadershipCmd = &cobra.Command{
	Use:   "forfeit-leadership",
	Short: "Tell node to forfeit etcd cluster leadership",
	Long:  ``,
	RunE: func(cmd *cobra.Command, args []string) error {
		return WithClient(func(ctx context.Context, c *client.Client) error {
			_, err := c.EtcdForfeitLeadership(ctx, &machine.EtcdForfeitLeadershipRequest{})

			return err
		})
	},
}

var etcdMemberListCmd = &cobra.Command{
	Use:   "members",
	Short: "Get the list of etcd cluster members",
	Long:  ``,
	RunE: func(cmd *cobra.Command, args []string) error {
		return WithClient(func(ctx context.Context, c *client.Client) error {
			response, err := c.EtcdMemberList(ctx, &machine.EtcdMemberListRequest{
				QueryLocal: true,
			})
			if err != nil {
				if response == nil {
					return fmt.Errorf("error getting members: %w", err)
				}
				cli.Warning("%s", err)
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
			node := ""
			pattern := "%s\t"

			for i, message := range response.Messages {
				if message.Metadata != nil && message.Metadata.Hostname != "" {
					node = message.Metadata.Hostname
				}

				if len(message.Members) == 0 {
					continue
				}

				if i == 0 {
					if node != "" {
						fmt.Fprintln(w, "NODE\tMEMBERS")
						pattern = "%s\t%s\n"
					} else {
						fmt.Fprintln(w, "MEMBERS")
					}
				}

				args := []interface{}{strings.Join(message.Members, ",")}
				if node != "" {
					args = append([]interface{}{node}, args...)
				}

				fmt.Fprintf(w, pattern, args...)

			}

			return w.Flush()
		})
	},
}

func init() {
	etcdCmd.AddCommand(etcdLeaveCmd, etcdForfeitLeadershipCmd, etcdMemberListCmd)
	addCommand(etcdCmd)
}
