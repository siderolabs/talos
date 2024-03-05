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
	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"

	"github.com/siderolabs/talos/pkg/cli"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/formatters"
)

// serviceCmd represents the service command.
var serviceCmd = &cobra.Command{
	Use:     "service [<id> [start|stop|restart|status]]",
	Aliases: []string{"services"},
	Short:   "Retrieve the state of a service (or all services), control service state",
	Long: `Service control command. If run without arguments, lists all the services and their state.
If service ID is specified, default action 'status' is executed which shows status of a single list service.
With actions 'start', 'stop', 'restart', service state is updated respectively.`,
	Args: cobra.MaximumNArgs(2),
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		switch len(args) {
		case 0:
			return getServiceFromNode(), cobra.ShellCompDirectiveNoFileComp
		case 1:
			return []string{"start", "stop", "restart", "status"}, cobra.ShellCompDirectiveNoFileComp
		}

		return nil, cobra.ShellCompDirectiveError | cobra.ShellCompDirectiveNoFileComp
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		action := "status"
		serviceID := ""

		if len(args) >= 1 {
			serviceID = args[0]
		}

		if len(args) == 2 {
			action = args[1]
		}

		return WithClient(func(ctx context.Context, c *client.Client) error {
			switch action {
			case "status":
				if serviceID == "" {
					return serviceList(ctx, c)
				}

				return serviceInfo(ctx, c, serviceID)
			case "start":
				return serviceStart(ctx, c, serviceID)
			case "stop":
				return serviceStop(ctx, c, serviceID)
			case "restart":
				return serviceRestart(ctx, c, serviceID)
			default:
				return fmt.Errorf("unsupported service action: %q", action)
			}
		})
	},
}

func serviceList(ctx context.Context, c *client.Client) error {
	var remotePeer peer.Peer

	resp, err := c.ServiceList(ctx, grpc.Peer(&remotePeer))
	if err != nil {
		if resp == nil {
			return fmt.Errorf("error listing services: %w", err)
		}

		cli.Warning("%s", err)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "NODE\tSERVICE\tSTATE\tHEALTH\tLAST CHANGE\tLAST EVENT")

	defaultNode := client.AddrFromPeer(&remotePeer)

	for _, msg := range resp.Messages {
		for _, s := range msg.Services {
			svc := formatters.ServiceInfoWrapper{ServiceInfo: s}

			node := defaultNode

			if msg.Metadata != nil {
				node = msg.Metadata.Hostname
			}

			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s ago\t%s\n", node, svc.Id, svc.State, svc.HealthStatus(), svc.LastUpdated(), svc.LastEvent())
		}
	}

	return w.Flush()
}

func serviceInfo(ctx context.Context, c *client.Client, id string) error {
	var remotePeer peer.Peer

	services, err := c.ServiceInfo(ctx, id, grpc.Peer(&remotePeer))
	if err != nil {
		if services == nil {
			return fmt.Errorf("error listing services: %w", err)
		}

		cli.Warning("%s", err)
	}

	defaultNode := client.AddrFromPeer(&remotePeer)

	if len(services) == 0 {
		return fmt.Errorf("service %q is not registered on any nodes", id)
	}

	return formatters.RenderServicesInfo(services, os.Stdout, defaultNode, true)
}

func serviceStart(ctx context.Context, c *client.Client, id string) error {
	var remotePeer peer.Peer

	resp, err := c.ServiceStart(ctx, id, grpc.Peer(&remotePeer))
	if err != nil {
		if resp == nil {
			return fmt.Errorf("error starting service: %w", err)
		}

		cli.Warning("%s", err)
	}

	defaultNode := client.AddrFromPeer(&remotePeer)

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "NODE\tRESPONSE")

	for _, msg := range resp.Messages {
		node := defaultNode

		if msg.Metadata != nil {
			node = msg.Metadata.Hostname
		}

		fmt.Fprintf(w, "%s\t%s\n", node, msg.Resp)
	}

	return w.Flush()
}

func serviceStop(ctx context.Context, c *client.Client, id string) error {
	var remotePeer peer.Peer

	resp, err := c.ServiceStop(ctx, id, grpc.Peer(&remotePeer))
	if err != nil {
		if resp == nil {
			return fmt.Errorf("error starting service: %w", err)
		}

		cli.Warning("%s", err)
	}

	defaultNode := client.AddrFromPeer(&remotePeer)

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "NODE\tRESPONSE")

	for _, msg := range resp.Messages {
		node := defaultNode

		if msg.Metadata != nil {
			node = msg.Metadata.Hostname
		}

		fmt.Fprintf(w, "%s\t%s\n", node, msg.Resp)
	}

	return w.Flush()
}

func serviceRestart(ctx context.Context, c *client.Client, id string) error {
	var remotePeer peer.Peer

	resp, err := c.ServiceRestart(ctx, id, grpc.Peer(&remotePeer))
	if err != nil {
		if resp == nil {
			return fmt.Errorf("error starting service: %w", err)
		}

		cli.Warning("%s", err)
	}

	defaultNode := client.AddrFromPeer(&remotePeer)

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "NODE\tRESPONSE")

	for _, msg := range resp.Messages {
		node := defaultNode

		if msg.Metadata != nil {
			node = msg.Metadata.Hostname
		}

		fmt.Fprintf(w, "%s\t%s\n", node, msg.Resp)
	}

	return w.Flush()
}

func init() {
	addCommand(serviceCmd)
}
