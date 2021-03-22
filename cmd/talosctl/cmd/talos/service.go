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

	"github.com/talos-systems/talos/pkg/cli"
	machineapi "github.com/talos-systems/talos/pkg/machinery/api/machine"
	"github.com/talos-systems/talos/pkg/machinery/client"
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
			svc := serviceInfoWrapper{s}

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

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)

	defaultNode := client.AddrFromPeer(&remotePeer)

	for _, s := range services {
		node := defaultNode

		if s.Metadata != nil {
			node = s.Metadata.Hostname
		}

		fmt.Fprintf(w, "NODE\t%s\n", node)

		svc := serviceInfoWrapper{s.Service}
		fmt.Fprintf(w, "ID\t%s\n", svc.Id)
		fmt.Fprintf(w, "STATE\t%s\n", svc.State)
		fmt.Fprintf(w, "HEALTH\t%s\n", svc.HealthStatus())

		if svc.Health.LastMessage != "" {
			fmt.Fprintf(w, "LAST HEALTH MESSAGE\t%s\n", svc.Health.LastMessage)
		}

		label := "EVENTS"

		for i := range svc.Events.Events {
			event := svc.Events.Events[len(svc.Events.Events)-1-i]

			ts := event.Ts.AsTime()
			fmt.Fprintf(w, "%s\t[%s]: %s (%s ago)\n", label, event.State, event.Msg, time.Since(ts).Round(time.Second))
			label = "" //nolint:wastedassign
		}
	}

	if len(services) == 0 {
		return fmt.Errorf("service %q is not registered on any nodes", id)
	}

	return w.Flush()
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

type serviceInfoWrapper struct {
	*machineapi.ServiceInfo
}

func (svc serviceInfoWrapper) LastUpdated() string {
	if len(svc.Events.Events) == 0 {
		return ""
	}

	ts := svc.Events.Events[len(svc.Events.Events)-1].Ts.AsTime()

	return time.Since(ts).Round(time.Second).String()
}

func (svc serviceInfoWrapper) LastEvent() string {
	if len(svc.Events.Events) == 0 {
		return "<none>"
	}

	return svc.Events.Events[len(svc.Events.Events)-1].Msg
}

func (svc serviceInfoWrapper) HealthStatus() string {
	if svc.Health.Unknown {
		return "?"
	}

	if svc.Health.Healthy {
		return "OK"
	}

	return "Fail"
}

func init() {
	addCommand(serviceCmd)
}
