// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/siderolabs/talos/pkg/cli"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/version"
)

// versionCmdFlags represents the `talosctl version` command's flags.
var versionCmdFlags struct {
	clientOnly   bool
	shortVersion bool
	json         bool
	insecure     bool
}

// versionCmd represents the `talosctl version` command.
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Prints the version",
	Long:  ``,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if !versionCmdFlags.json {
			fmt.Println("Client:")

			if versionCmdFlags.shortVersion {
				version.PrintShortVersion()
			} else {
				version.PrintLongVersion()
			}

			// Exit early if we're only looking for client version
			if versionCmdFlags.clientOnly {
				return nil
			}

			fmt.Println("Server:")
		}

		if versionCmdFlags.insecure {
			return WithClientMaintenance(nil, cmdVersion)
		}

		return WithClient(cmdVersion)
	},
}

func cmdVersion(ctx context.Context, c *client.Client) error {
	var remotePeer peer.Peer

	resp, err := c.Version(ctx, grpc.Peer(&remotePeer))
	if err != nil {
		if resp == nil {
			return fmt.Errorf("error getting version: %s", err)
		}

		cli.Warning("%s", err)
	}

	defaultNode := client.AddrFromPeer(&remotePeer)

	for _, msg := range resp.Messages {
		node := defaultNode

		if msg.Metadata != nil {
			node = msg.Metadata.Hostname
		}

		if !versionCmdFlags.json {
			fmt.Printf("\t%s:        %s\n", "NODE", node)

			version.PrintLongVersionFromExisting(msg.Version)

			var enabledFeatures []string
			if msg.Features.GetRbac() {
				enabledFeatures = append(enabledFeatures, "RBAC")
			}

			fmt.Printf("\tEnabled:     %s\n", strings.Join(enabledFeatures, ", "))

			continue
		}

		b, err := protojson.Marshal(msg)
		if err != nil {
			return err
		}

		fmt.Printf("%s\n", b)
	}

	return nil
}

func init() {
	versionCmd.Flags().BoolVar(&versionCmdFlags.shortVersion, "short", false, "Print the short version")
	versionCmd.Flags().BoolVar(&versionCmdFlags.clientOnly, "client", false, "Print client version only")
	versionCmd.Flags().BoolVarP(&versionCmdFlags.insecure, "insecure", "i", false, "use Talos maintenance mode API")

	// TODO remove when https://github.com/siderolabs/talos/issues/907 is implemented
	versionCmd.Flags().BoolVar(&versionCmdFlags.json, "json", false, "")
	cli.Should(versionCmd.Flags().MarkHidden("json"))

	addCommand(versionCmd)
}
