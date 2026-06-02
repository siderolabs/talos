// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"
	snapshot "go.etcd.io/etcd/etcdutl/v3/snapshot"

	"github.com/siderolabs/talos/pkg/logging"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/client/multiplex"
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

// etcdAlarmListCmd represents the etcd alarm list command.
var etcdAlarmListCmd = &cobra.Command{
	Use:   "list",
	Short: "List the etcd alarms for the node.",
	Long:  ``,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		clientFactory, err := NewClientFactory(ctx, nil)
		if err != nil {
			return err
		}

		defer clientFactory.Close() //nolint:errcheck

		responseChan := multiplex.UnaryViaFactory(
			ctx, clientFactory,
			func(ctx context.Context, c *client.Client) (*machine.EtcdAlarmListResponse, error) {
				return c.EtcdAlarmList(ctx)
			},
		)

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)

		flushTimer := time.NewTimer(outputFlushInterval)
		defer flushTimer.Stop()

		flushTimer.Stop()

		var (
			errs          error
			headerPrinted bool
		)

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
						for _, alarm := range msg.GetMemberAlarms() {
							if !headerPrinted {
								fmt.Fprintln(w, "NODE\tMEMBER\tALARM")

								headerPrinted = true
							}

							fmt.Fprintf(w, "%s\t%s\t%s\n", resp.Node, etcdresource.FormatMemberID(alarm.GetMemberId()), alarm.GetAlarm().String())
						}
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

// etcdAlarmDisarmCmd represents the etcd alarm disarm command.
var etcdAlarmDisarmCmd = &cobra.Command{
	Use:   "disarm",
	Short: "Disarm the etcd alarms for the node.",
	Long:  ``,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		clientFactory, err := NewClientFactory(ctx, nil)
		if err != nil {
			return err
		}

		defer clientFactory.Close() //nolint:errcheck

		responseChan := multiplex.UnaryViaFactory(
			ctx, clientFactory,
			func(ctx context.Context, c *client.Client) (*machine.EtcdAlarmDisarmResponse, error) {
				return c.EtcdAlarmDisarm(ctx)
			},
		)

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)

		flushTimer := time.NewTimer(outputFlushInterval)
		defer flushTimer.Stop()

		flushTimer.Stop()

		var (
			errs          error
			headerPrinted bool
		)

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
						for _, alarm := range msg.GetMemberAlarms() {
							if !headerPrinted {
								fmt.Fprintln(w, "NODE\tMEMBER\tALARM")

								headerPrinted = true
							}

							fmt.Fprintf(w, "%s\t%s\t%s\n", resp.Node, etcdresource.FormatMemberID(alarm.GetMemberId()), alarm.GetAlarm().String())
						}
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

// etcdDefragCmd represents the etcd defrag command.
var etcdDefragCmd = &cobra.Command{
	Use:   "defrag",
	Short: "Defragment etcd database on the node",
	Long: `Defragmentation is a maintenance operation that releases unused space from the etcd database file.
Defragmentation is a resource heavy operation and should be performed only when necessary on a single node at a time.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		clientFactory, err := NewClientFactory(ctx, nil)
		if err != nil {
			return err
		}

		defer clientFactory.Close() //nolint:errcheck

		ctx, c, _, err := clientFactory.BuildClientEnforceSingleNode(ctx, "etcd defrag")
		if err != nil {
			return err
		}

		_, err = c.EtcdDefragment(ctx)

		return err
	},
}

var etcdLeaveCmd = &cobra.Command{
	Use:   "leave",
	Short: "Tell nodes to leave etcd cluster",
	Long:  ``,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		clientFactory, err := NewClientFactory(ctx, nil)
		if err != nil {
			return err
		}

		defer clientFactory.Close() //nolint:errcheck

		ctx, c, _, err := clientFactory.BuildClientEnforceSingleNode(ctx, "etcd leave")
		if err != nil {
			return err
		}

		return c.EtcdLeaveCluster(ctx, &machine.EtcdLeaveClusterRequest{})
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
		ctx := cmd.Context()

		clientFactory, err := NewClientFactory(ctx, nil)
		if err != nil {
			return err
		}

		defer clientFactory.Close() //nolint:errcheck

		memberID, err := etcdresource.ParseMemberID(args[0])
		if err != nil {
			return fmt.Errorf("error parsing member ID: %w", err)
		}

		responseChan := multiplex.UnaryViaFactory(
			ctx, clientFactory,
			func(ctx context.Context, c *client.Client) (struct{}, error) {
				return struct{}{}, c.EtcdRemoveMemberByID(ctx, &machine.EtcdRemoveMemberByIDRequest{
					MemberId: memberID,
				})
			},
		)

		var errs error

		for resp := range responseChan {
			if resp.Err != nil {
				errs = errors.Join(errs, fmt.Errorf("error from node %s: %w", resp.Node, resp.Err))
			}
		}

		return errs
	},
}

var etcdForfeitLeadershipCmd = &cobra.Command{
	Use:   "forfeit-leadership",
	Short: "Tell node to forfeit etcd cluster leadership",
	Long:  ``,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		clientFactory, err := NewClientFactory(ctx, nil)
		if err != nil {
			return err
		}

		defer clientFactory.Close() //nolint:errcheck

		responseChan := multiplex.UnaryViaFactory(
			ctx, clientFactory,
			func(ctx context.Context, c *client.Client) (struct{}, error) {
				_, err := c.EtcdForfeitLeadership(ctx, &machine.EtcdForfeitLeadershipRequest{})

				return struct{}{}, err
			},
		)

		var errs error

		for resp := range responseChan {
			if resp.Err != nil {
				errs = errors.Join(errs, fmt.Errorf("error from node %s: %w", resp.Node, resp.Err))
			}
		}

		return errs
	},
}

var etcdMemberListCmd = &cobra.Command{
	Use:   "members",
	Short: "Get the list of etcd cluster members",
	Long:  ``,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		clientFactory, err := NewClientFactory(ctx, nil)
		if err != nil {
			return err
		}

		defer clientFactory.Close() //nolint:errcheck

		responseChan := multiplex.UnaryViaFactory(
			ctx, clientFactory,
			func(ctx context.Context, c *client.Client) (*machine.EtcdMemberListResponse, error) {
				return c.EtcdMemberList(ctx, &machine.EtcdMemberListRequest{
					QueryLocal: true,
				})
			},
		)

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "NODE\tID\tHOSTNAME\tPEER URLS\tCLIENT URLS\tLEARNER")

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
					for _, message := range resp.Payload.Messages {
						for _, member := range message.Members {
							fmt.Fprintf(
								w, "%s\t%s\t%s\t%s\t%s\t%v\n",
								resp.Node,
								etcdresource.FormatMemberID(member.Id),
								member.Hostname,
								strings.Join(member.PeerUrls, ","),
								strings.Join(member.ClientUrls, ","),
								member.IsLearner,
							)
						}
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

var etcdStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Get the status of etcd cluster member",
	Long:  `Returns the status of etcd member on the node, use multiple nodes to get status of all members.`,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		clientFactory, err := NewClientFactory(ctx, nil)
		if err != nil {
			return err
		}

		defer clientFactory.Close() //nolint:errcheck

		responseChan := multiplex.UnaryViaFactory(
			ctx, clientFactory,
			func(ctx context.Context, c *client.Client) (*machine.EtcdStatusResponse, error) {
				return c.EtcdStatus(ctx)
			},
		)

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "NODE\tMEMBER\tDB SIZE\tIN USE\tLEADER\tRAFT INDEX\tRAFT TERM\tRAFT APPLIED INDEX\tLEARNER\tPROTOCOL\tSTORAGE\tERRORS")

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
					for _, message := range resp.Payload.Messages {
						var ratio float64

						if message.GetMemberStatus().GetDbSize() > 0 {
							ratio = float64(message.GetMemberStatus().GetDbSizeInUse()) / float64(message.GetMemberStatus().GetDbSize()) * 100.0
						}

						fmt.Fprintf(
							w, "%s\t%s\t%s\t%s (%.2f%%)\t%s\t%d\t%d\t%d\t%v\t%s\t%s\t%s\n",
							resp.Node,
							etcdresource.FormatMemberID(message.GetMemberStatus().GetMemberId()),
							humanize.Bytes(uint64(message.GetMemberStatus().GetDbSize())),
							humanize.Bytes(uint64(message.GetMemberStatus().GetDbSizeInUse())),
							ratio,
							etcdresource.FormatMemberID(message.GetMemberStatus().GetLeader()),
							message.GetMemberStatus().GetRaftIndex(),
							message.GetMemberStatus().GetRaftTerm(),
							message.GetMemberStatus().GetRaftAppliedIndex(),
							message.GetMemberStatus().GetIsLearner(),
							message.GetMemberStatus().GetProtocolVersion(),
							message.GetMemberStatus().GetStorageVersion(),
							strings.Join(message.GetMemberStatus().GetErrors(), ", "),
						)
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

var etcdSnapshotCmd = &cobra.Command{
	Use:   "snapshot <path>",
	Short: "Stream snapshot of the etcd node to the path.",
	Long:  ``,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		clientFactory, err := NewClientFactory(ctx, nil)
		if err != nil {
			return err
		}

		defer clientFactory.Close() //nolint:errcheck

		ctx, c, _, err := clientFactory.BuildClientEnforceSingleNode(ctx, "etcd snapshot")
		if err != nil {
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
	},
}

var etcdDowngradeCmd = &cobra.Command{
	Use:   "downgrade",
	Short: "Manage etcd storage system downgrades",
	Long:  ``,
}

const (
	etcdDowngradePattern = "%s\t%s\n"
	etcdDowngradeHeader  = "NODE\tMESSAGE"
)

var etcdDowngradeValidateCmd = &cobra.Command{
	Use:   "validate <version>",
	Short: "Validate if the etcd storage system can be downgraded to the specified version.",
	Long:  ``,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		clientFactory, err := NewClientFactory(ctx, nil)
		if err != nil {
			return err
		}

		defer clientFactory.Close() //nolint:errcheck

		ctx, c, node, err := clientFactory.BuildClientEnforceSingleNode(ctx, "etcd downgrade validate")
		if err != nil {
			return err
		}

		version := args[0]

		r, err := c.EtcdDowngradeValidate(ctx, &machine.EtcdDowngradeValidateRequest{Version: version})
		if err != nil {
			return err
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		pattern := etcdDowngradePattern
		header := etcdDowngradeHeader

		for i, message := range r.Messages {
			if i == 0 {
				fmt.Fprintln(w, header)
			}

			fmt.Fprintf(
				w, pattern, node,
				fmt.Sprintf(
					"downgrade validate success, cluster version %s",
					message.GetClusterDowngrade().GetClusterVersion(),
				),
			)
		}

		return w.Flush()
	},
}

var etcdDowngradeEnableCmd = &cobra.Command{
	Use:   "enable <version>",
	Short: "Enable etcd storage system downgrade to the specified version.",
	Long:  ``,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		clientFactory, err := NewClientFactory(ctx, nil)
		if err != nil {
			return err
		}

		defer clientFactory.Close() //nolint:errcheck

		ctx, c, node, err := clientFactory.BuildClientEnforceSingleNode(ctx, "etcd downgrade enable")
		if err != nil {
			return err
		}

		version := args[0]

		r, err := c.EtcdDowngradeEnable(ctx, &machine.EtcdDowngradeEnableRequest{Version: version})
		if err != nil {
			return err
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		pattern := etcdDowngradePattern
		header := etcdDowngradeHeader

		for i, message := range r.Messages {
			if i == 0 {
				fmt.Fprintln(w, header)
			}

			fmt.Fprintf(
				w, pattern,
				node,
				fmt.Sprintf(
					"downgrade enable success, cluster version %s",
					message.GetClusterDowngrade().GetClusterVersion(),
				),
			)
		}

		return w.Flush()
	},
}

var etcdDowngradeCancelCmd = &cobra.Command{
	Use:   "cancel",
	Short: "Cancel etcd storage system downgrade.",
	Long:  ``,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		clientFactory, err := NewClientFactory(ctx, nil)
		if err != nil {
			return err
		}

		defer clientFactory.Close() //nolint:errcheck

		ctx, c, node, err := clientFactory.BuildClientEnforceSingleNode(ctx, "etcd downgrade cancel")
		if err != nil {
			return err
		}

		r, err := c.EtcdDowngradeCancel(ctx)
		if err != nil {
			return err
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		pattern := etcdDowngradePattern
		header := etcdDowngradeHeader

		for i, message := range r.Messages {
			if i == 0 {
				fmt.Fprintln(w, header)
			}

			fmt.Fprintf(
				w, pattern, node,
				fmt.Sprintf(
					"downgrade cancel success, cluster version %s",
					message.GetClusterDowngrade().GetClusterVersion(),
				),
			)
		}

		return w.Flush()
	},
}

func init() {
	etcdAlarmCmd.AddCommand(
		etcdAlarmListCmd,
		etcdAlarmDisarmCmd,
	)

	etcdDowngradeCmd.AddCommand(
		etcdDowngradeValidateCmd,
		etcdDowngradeEnableCmd,
		etcdDowngradeCancelCmd,
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
		etcdDowngradeCmd,
	)

	addCommand(etcdCmd)
}
