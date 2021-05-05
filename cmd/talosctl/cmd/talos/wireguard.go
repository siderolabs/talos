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
	"time"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"

	"github.com/talos-systems/talos/pkg/cli"
	networkapi "github.com/talos-systems/talos/pkg/machinery/api/network"
	"github.com/talos-systems/talos/pkg/machinery/client"
)

// wireguardCmd represents the wireguard command tree.
var wireguardCmd = &cobra.Command{
	Use:   "wireguard",
	Short: "wireguard operations",
	Long:  ``,
}

var wireguardDevicesCmd = &cobra.Command{
	Use:   "devices",
	Short: "list wireguard devices and peers",
	Long:  "Returns the status and connections of all Wireguard devices and the peers thereof.",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return WithClient(func(ctx context.Context, c *client.Client) error {
			var remotePeer peer.Peer

			resp, err := c.WireguardDevices(ctx, grpc.Peer(&remotePeer))
			if err != nil {
				if resp == nil {
					return fmt.Errorf("error getting wireguard devices: %w", err)
				}

				cli.Warning("%s", err.Error())
			}

			return wgDevicesRender(&remotePeer, resp)
		})
	},
}

func wgDevicesRender(remotePeer *peer.Peer, resp *networkapi.WireguardDevicesResponse) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "NODE\tNAME\tPUBLIC_KEY\tPORT\tPEERS")

	defaultNode := client.AddrFromPeer(remotePeer)

	for _, msg := range resp.Messages {
		node := defaultNode

		if msg.Metadata != nil {
			node = msg.Metadata.Hostname
		}

		for _, d := range msg.Devices {
			fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%d\n", node, d.Name, d.PublicKey, d.ListenPort, len(d.Peers))
		}
	}

	return w.Flush()
}

var wireguardPeersCmd = &cobra.Command{
	Use:   "peers [wg-device]",
	Short: "list the peers of a wireguard device",
	Long:  "Returns the details of the peers of a wireguard device",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return WithClient(func(ctx context.Context, c *client.Client) error {
			var remotePeer peer.Peer

			var devName string

			if len(args) > 0 {
				devName = args[0]
			}

			resp, err := c.WireguardDevices(ctx, grpc.Peer(&remotePeer))
			if err != nil {
				if resp == nil {
					return fmt.Errorf("error getting wireguard devices: %w", err)
				}

				cli.Warning("%s", err.Error())
			}

			return wgPeersRender(&remotePeer, resp, devName)
		})
	},
}

func wgPeersRender(remotePeer *peer.Peer, resp *networkapi.WireguardDevicesResponse, devName string) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "NODE\tDEVICE\tPUBLIC_KEY\tENDPOINT\tLAST_HANDSHAKE\tACCEPT_IPS")

	defaultNode := client.AddrFromPeer(remotePeer)

	for _, msg := range resp.Messages {
		node := defaultNode

		if msg.Metadata != nil {
			node = msg.Metadata.Hostname
		}

		for _, d := range msg.Devices {
			if devName != "" && d.Name != devName {
				continue
			}

			for _, p := range d.Peers {
				var lastHandshake string

				if p.LastHandshake.AsTime().IsZero() {
					lastHandshake = "never"
				} else {
					lastHandshake = time.Since(p.LastHandshake.AsTime()).Truncate(time.Second).String()
				}

				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", node, d.Name, p.PublicKey, p.Endpoint, lastHandshake, strings.Join(p.AllowedIps, ","))
			}
		}
	}

	return w.Flush()
}

func init() {
	wireguardCmd.AddCommand(wireguardDevicesCmd)
	wireguardCmd.AddCommand(wireguardPeersCmd)

	addCommand(wireguardCmd)
}
