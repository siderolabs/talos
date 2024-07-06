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
	"strings"
	"text/tabwriter"

	"github.com/dustin/go-humanize"
	"github.com/siderolabs/gen/xslices"
	"github.com/spf13/cobra"
	snapshot "go.etcd.io/etcd/etcdutl/v3/snapshot"

	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/helpers"
	"github.com/siderolabs/talos/pkg/cli"
	"github.com/siderolabs/talos/pkg/logging"
	"github.com/siderolabs/talos/pkg/machinery/api/common"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	etcdresource "github.com/siderolabs/talos/pkg/machinery/resources/etcd"
)

// etcdCmd represents the etcd command.
var etcdCmd = &cobra.Command{
	Use:   "etcd",
	Short: "Manage etcd",
	Long:  ``,
}

// etcdAlarmCmd represents the etcd alarm command.
var etcdAlarmCmd = &cobra.Command{
	Use:   "alarm",
	Short: "Manage etcd alarms",
	Long:  ``,
}

type alarmMessage interface {
	GetMetadata() *common.Metadata
	GetMemberAlarms() []*machine.EtcdMemberAlarm
}

func displayAlarms(messages []alarmMessage) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	node := ""
	pattern := "%s\t%s\n"
	header := "MEMBER\tALARM"

	for i, message := range messages {
		if message.GetMetadata() != nil && message.GetMetadata().GetHostname() != "" {
			node = message.GetMetadata().GetHostname()
		}

		for j, alarm := range message.GetMemberAlarms() {
			if i == 0 && j == 0 {
				if node != "" {
					header = "NODE\t" + header
					pattern = "%s\t" + pattern
				}

				fmt.Fprintln(w, header)
			}

			args := []any{
				etcdresource.FormatMemberID(alarm.GetMemberId()),
				alarm.GetAlarm().String(),
			}
			if node != "" {
				args = append([]any{node}, args...)
			}

			fmt.Fprintf(w, pattern, args...)
		}
	}

	return w.Flush()
}

// etcdAlarmListCmd represents the etcd alarm list command.
var etcdAlarmListCmd = &cobra.Command{
	Use:   "list",
	Short: "List the etcd alarms for the node.",
	Long:  ``,
	RunE: func(cmd *cobra.Command, args []string) error {
		return WithClient(func(ctx context.Context, c *client.Client) error {
			response, err := c.EtcdAlarmList(ctx)
			if err != nil {
				if response == nil {
					return fmt.Errorf("error getting alarms: %w", err)
				}
				cli.Warning("%s", err)
			}

			return displayAlarms(xslices.Map(response.Messages, func(v *machine.EtcdAlarm) alarmMessage {
				return v
			}))
		})
	},
}

// etcdAlarmDisarmCmd represents the etcd alarm disarm command.
var etcdAlarmDisarmCmd = &cobra.Command{
	Use:   "disarm",
	Short: "Disarm the etcd alarms for the node.",
	Long:  ``,
	RunE: func(cmd *cobra.Command, args []string) error {
		return WithClient(func(ctx context.Context, c *client.Client) error {
			response, err := c.EtcdAlarmDisarm(ctx)
			if err != nil {
				if response == nil {
					return fmt.Errorf("error disarming alarms: %w", err)
				}
				cli.Warning("%s", err)
			}

			return displayAlarms(xslices.Map(response.Messages, func(v *machine.EtcdAlarmDisarm) alarmMessage {
				return v
			}))
		})
	},
}

// etcdDefragCmd represents the etcd defrag command.
var etcdDefragCmd = &cobra.Command{
	Use:   "defrag",
	Short: "Defragment etcd database on the node",
	Long: `Defragmentation is a maintenance operation that releases unused space from the etcd database file.
Defragmentation is a resource heavy operation and should be performed only when necessary on a single node at a time.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return WithClient(func(ctx context.Context, c *client.Client) error {
			if err := helpers.FailIfMultiNodes(ctx, "etcd defrag"); err != nil {
				return err
			}

			_, err := c.EtcdDefragment(ctx)

			return err
		})
	},
}

var etcdLeaveCmd = &cobra.Command{
	Use:   "leave",
	Short: "Tell nodes to leave etcd cluster",
	Long:  ``,
	RunE: func(cmd *cobra.Command, args []string) error {
		return WithClient(func(ctx context.Context, c *client.Client) error {
			if err := helpers.FailIfMultiNodes(ctx, "etcd leave"); err != nil {
				return err
			}

			return c.EtcdLeaveCluster(ctx, &machine.EtcdLeaveClusterRequest{})
		})
	},
}

var etcdMemberRemoveCmd = &cobra.Command{
	Use:   "remove-member <member ID>",
	Short: "Remove the node from etcd cluster",
	Long: `Use this command only if you want to remove a member which is in broken state.
If there is no access to the node, or the node can't access etcd to call etcd leave.
Always prefer etcd leave over this command.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return WithClient(func(ctx context.Context, c *client.Client) error {
			memberID, err := etcdresource.ParseMemberID(args[0])
			if err != nil {
				return fmt.Errorf("error parsing member ID: %w", err)
			}

			return c.EtcdRemoveMemberByID(ctx, &machine.EtcdRemoveMemberByIDRequest{
				MemberId: memberID,
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
			pattern := "%s\t%s\t%s\t%s\t%v\n"

			for i, message := range response.Messages {
				if message.Metadata != nil && message.Metadata.Hostname != "" {
					node = message.Metadata.Hostname
				}

				if len(message.Members) == 0 {
					continue
				}

				for j, member := range message.Members {
					if i == 0 && j == 0 {
						if node != "" {
							fmt.Fprintln(w, "NODE\tID\tHOSTNAME\tPEER URLS\tCLIENT URLS\tLEARNER")
							pattern = "%s\t" + pattern
						} else {
							fmt.Fprintln(w, "ID\tHOSTNAME\tPEER URLS\tCLIENT URLS\tLEARNER")
						}
					}

					args := []any{
						etcdresource.FormatMemberID(member.Id),
						member.Hostname,
						strings.Join(member.PeerUrls, ","),
						strings.Join(member.ClientUrls, ","),
						member.IsLearner,
					}
					if node != "" {
						args = append([]any{node}, args...)
					}

					fmt.Fprintf(w, pattern, args...)
				}
			}

			return w.Flush()
		})
	},
}

var etcdStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Get the status of etcd cluster member",
	Long:  `Returns the status of etcd member on the node, use multiple nodes to get status of all members.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return WithClient(func(ctx context.Context, c *client.Client) error {
			response, err := c.EtcdStatus(ctx)
			if err != nil {
				if response == nil {
					return fmt.Errorf("error getting status: %w", err)
				}
				cli.Warning("%s", err)
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
			node := ""
			pattern := "%s\t%s\t%s (%.2f%%)\t%s\t%d\t%d\t%d\t%v\t%s\n"
			header := "MEMBER\tDB SIZE\tIN USE\tLEADER\tRAFT INDEX\tRAFT TERM\tRAFT APPLIED INDEX\tLEARNER\tERRORS"

			for i, message := range response.Messages {
				if message.Metadata != nil && message.Metadata.Hostname != "" {
					node = message.Metadata.Hostname
				}

				if i == 0 {
					if node != "" {
						header = "NODE\t" + header
						pattern = "%s\t" + pattern
					}

					fmt.Fprintln(w, header)
				}

				var ratio float64

				if message.GetMemberStatus().GetDbSize() > 0 {
					ratio = float64(message.GetMemberStatus().GetDbSizeInUse()) / float64(message.GetMemberStatus().GetDbSize()) * 100.0
				}

				args := []any{
					etcdresource.FormatMemberID(message.GetMemberStatus().GetMemberId()),
					humanize.Bytes(uint64(message.GetMemberStatus().GetDbSize())),
					humanize.Bytes(uint64(message.GetMemberStatus().GetDbSizeInUse())),
					ratio,
					etcdresource.FormatMemberID(message.GetMemberStatus().GetLeader()),
					message.GetMemberStatus().GetRaftIndex(),
					message.GetMemberStatus().GetRaftTerm(),
					message.GetMemberStatus().GetRaftAppliedIndex(),
					message.GetMemberStatus().GetIsLearner(),
					strings.Join(message.GetMemberStatus().GetErrors(), ", "),
				}
				if node != "" {
					args = append([]any{node}, args...)
				}

				fmt.Fprintf(w, pattern, args...)
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

			r, err := c.EtcdSnapshot(ctx, &machine.EtcdSnapshotRequest{})
			if err != nil {
				return fmt.Errorf("error reading file: %w", err)
			}

			defer r.Close() //nolint:errcheck

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

			if err = dest.Close(); err != nil {
				return fmt.Errorf("failed to close: %w", err)
			}

			if err = os.Rename(partPath, dbPath); err != nil {
				return fmt.Errorf("error renaming to final location: %w", err)
			}

			fmt.Printf("etcd snapshot saved to %q (%d bytes)\n", dbPath, size)

			manager := snapshot.NewV3(logging.Wrap(os.Stderr))

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
	etcdAlarmCmd.AddCommand(
		etcdAlarmListCmd,
		etcdAlarmDisarmCmd,
	)

	etcdCmd.AddCommand(
		etcdAlarmCmd,
		etcdDefragCmd,
		etcdForfeitLeadershipCmd,
		etcdLeaveCmd,
		etcdMemberListCmd,
		etcdMemberRemoveCmd,
		etcdSnapshotCmd,
		etcdStatusCmd,
	)

	addCommand(etcdCmd)
}
