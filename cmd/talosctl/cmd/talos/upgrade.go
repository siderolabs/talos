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
	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"

	"github.com/talos-systems/talos/cmd/talosctl/pkg/talos/action"
	"github.com/talos-systems/talos/cmd/talosctl/pkg/talos/helpers"
	"github.com/talos-systems/talos/pkg/cli"
	"github.com/talos-systems/talos/pkg/machinery/client"
)

var upgradeCmdFlags struct {
	upgradeImage string
	preserve     bool
	stage        bool
	force        bool
	wait         bool
	debug        bool
}

// upgradeCmd represents the processes command.
var upgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Upgrade Talos on the target node",
	Long:  ``,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if upgradeCmdFlags.debug {
			upgradeCmdFlags.wait = true
		}

		if !upgradeCmdFlags.wait {
			return runUpgradeNoWait()
		}

		cmd.SilenceErrors = true

		postCheckFn := func(ctx context.Context, c *client.Client) error {
			_, err := c.Disks(ctx)

			return err
		}

		return action.NewTracker(&GlobalArgs, action.MachineReadyEventFn, upgradeGetActorID, postCheckFn, upgradeCmdFlags.debug).Run()
	},
}

func runUpgradeNoWait() error {
	return WithClient(func(ctx context.Context, c *client.Client) error {
		if err := helpers.ClientVersionCheck(ctx, c); err != nil {
			return err
		}

		var remotePeer peer.Peer

		// TODO: See if we can validate version and prevent starting upgrades to an unknown version
		resp, err := c.Upgrade(
			ctx,
			upgradeCmdFlags.upgradeImage,
			upgradeCmdFlags.preserve,
			upgradeCmdFlags.stage,
			force,
			grpc.Peer(&remotePeer),
		)
		if err != nil {
			if resp == nil {
				return fmt.Errorf("error performing upgrade: %s", err)
			}

			cli.Warning("%s", err)
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "NODE\tACK\tSTARTED")

		defaultNode := client.AddrFromPeer(&remotePeer)

		for _, msg := range resp.Messages {
			node := defaultNode

			if msg.Metadata != nil {
				node = msg.Metadata.Hostname
			}

			fmt.Fprintf(w, "%s\t%s\t%s\t\n", node, msg.Ack, time.Now())
		}

		return w.Flush()
	})
}

func upgradeGetActorID(ctx context.Context, c *client.Client) (string, error) {
	resp, err := c.Upgrade(
		ctx,
		upgradeCmdFlags.upgradeImage,
		upgradeCmdFlags.preserve,
		upgradeCmdFlags.stage,
		force,
	)
	if err != nil {
		return "", err
	}

	if len(resp.GetMessages()) == 0 {
		return "", fmt.Errorf("no messages returned from action run")
	}

	return resp.GetMessages()[0].GetActorId(), nil
}

func init() {
	upgradeCmd.Flags().StringVarP(&upgradeCmdFlags.upgradeImage, "image", "i", "", "the container image to use for performing the install")
	upgradeCmd.Flags().BoolVarP(&upgradeCmdFlags.preserve, "preserve", "p", false, "preserve data")
	upgradeCmd.Flags().BoolVarP(&upgradeCmdFlags.stage, "stage", "s", false, "stage the upgrade to perform it after a reboot")
	upgradeCmd.Flags().BoolVarP(&upgradeCmdFlags.force, "force", "f", false, "force the upgrade (skip checks on etcd health and members, might lead to data loss)")
	upgradeCmd.Flags().BoolVar(&upgradeCmdFlags.wait, "wait", false, "wait for the operation to complete, tracking its progress. always set to true when --debug is set")
	upgradeCmd.Flags().BoolVar(&upgradeCmdFlags.debug, "debug", false, "debug operation from kernel logs. --no-wait is set to false when this flag is set")
	addCommand(upgradeCmd)
}
