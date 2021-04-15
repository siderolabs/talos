// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"go.etcd.io/etcd/etcdctl/v3/snapshot"

	"github.com/talos-systems/talos/cmd/talosctl/pkg/talos/helpers"
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

var etcdMemberRemoveCmd = &cobra.Command{
	Use:   "remove-member <hostname>",
	Short: "Remove the node from etcd cluster",
	Long: `Use this command only if you want to remove a member which is in broken state.
If there is no access to the node, or the node can't access etcd to call etcd leave.
Always prefer etcd leave over this command.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return WithClient(func(ctx context.Context, c *client.Client) error {
			return c.EtcdRemoveMember(ctx, &machine.EtcdRemoveMemberRequest{
				Member: args[0],
			})
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
			pattern := "%s\t%s\t%s\t%s\n"

			for i, message := range response.Messages {
				if message.Metadata != nil && message.Metadata.Hostname != "" {
					node = message.Metadata.Hostname
				}

				if len(message.Members) == 0 {
					continue
				}

				for _, member := range message.Members {
					if i == 0 {
						if node != "" {
							fmt.Fprintln(w, "NODE\tID\tHOSTNAME\tPEER URLS\tCLIENT URLS")
							pattern = "%s\t" + pattern
						} else {
							fmt.Fprintln(w, "ID\tHOSTNAME\tPEER URLS\tCLIENT URLS")
						}
					}

					args := []interface{}{
						strconv.FormatUint(member.Id, 16),
						member.Hostname,
						strings.Join(member.PeerUrls, ","),
						strings.Join(member.ClientUrls, ","),
					}
					if node != "" {
						args = append([]interface{}{node}, args...)
					}

					fmt.Fprintf(w, pattern, args...)
				}
			}

			return w.Flush()
		})
	},
}

var etcdSnapshotCmd = &cobra.Command{
	Use:   "snapshot <path>",
	Short: "Stream snapshot of the etcd node to the path.",
	Long:  ``,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return WithClient(func(ctx context.Context, c *client.Client) error {
			if err := helpers.FailIfMultiNodes(ctx, "etcd snapshot"); err != nil {
				return err
			}

			dbPath := args[0]
			partPath := dbPath + ".part"

			defer os.RemoveAll(partPath) //nolint:errcheck

			dest, err := os.OpenFile(partPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
			if err != nil {
				return fmt.Errorf("error creating temporary file: %w", err)
			}

			defer dest.Close() //nolint:errcheck

			r, errCh, err := c.EtcdSnapshot(ctx, &machine.EtcdSnapshotRequest{})
			if err != nil {
				return fmt.Errorf("error reading file: %w", err)
			}

			defer r.Close() //nolint:errcheck

			var wg sync.WaitGroup

			wg.Add(1)
			go func() {
				defer wg.Done()
				for err := range errCh {
					fmt.Fprintln(os.Stderr, err.Error())
				}
			}()

			defer wg.Wait()

			size, err := io.Copy(dest, r)
			if err != nil {
				return fmt.Errorf("error reading: %w", err)
			}

			if err = dest.Sync(); err != nil {
				return fmt.Errorf("failed to fsync: %w", err)
			}

			// this check is from https://github.com/etcd-io/etcd/blob/client/v3.5.0-alpha.0/client/v3/snapshot/v3_snapshot.go#L46
			if (size % 512) != sha256.Size {
				return fmt.Errorf("sha256 checksum not found (size %d)", size)
			}

			if err = os.Rename(partPath, dbPath); err != nil {
				return fmt.Errorf("error renaming to final location: %w", err)
			}

			fmt.Printf("etcd snapshot saved to %q (%d bytes)\n", dbPath, size)

			manager := snapshot.NewV3(nil)

			status, err := manager.Status(dbPath)
			if err != nil {
				return err
			}

			fmt.Printf("snapshot info: hash %08x, revision %d, total keys %d, total size %d\n",
				status.Hash, status.Revision, status.TotalKey, status.TotalSize)

			return nil
		})
	},
}

func init() {
	etcdCmd.AddCommand(etcdLeaveCmd, etcdForfeitLeadershipCmd, etcdMemberListCmd, etcdMemberRemoveCmd, etcdSnapshotCmd)
	addCommand(etcdCmd)
}
