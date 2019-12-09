// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/golang/protobuf/ptypes"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"

	machineapi "github.com/talos-systems/talos/api/machine"
	"github.com/talos-systems/talos/cmd/osctl/pkg/client"
	"github.com/talos-systems/talos/cmd/osctl/pkg/helpers"
)

// serviceCmd represents the service command
var serviceCmd = &cobra.Command{
	Use:     "service [<id> [start|stop|restart|status]]",
	Aliases: []string{"services"},
	Short:   "Retrieve the state of a service (or all services), control service state",
	Long: `Service control command. If run without arguments, lists all the services and their state.
If service ID is specified, default action 'status' is executed which shows status of a single list service.
With actions 'start', 'stop', 'restart', service state is updated respectively.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) > 2 {
			helpers.Should(cmd.Usage())
			os.Exit(1)
		}

		action := "status"
		serviceID := ""
		if len(args) >= 1 {
			serviceID = args[0]
		}
		if len(args) == 2 {
			action = args[1]
		}

		return setupClientE(func(c *client.Client) error {
			switch action {
			case "status":
				if serviceID == "" {
					return serviceList(c)
				}

				return serviceInfo(c, serviceID)
			case "start":
				return serviceStart(c, serviceID)
			case "stop":
				return serviceStop(c, serviceID)
			case "restart":
				return serviceRestart(c, serviceID)
			default:
				return fmt.Errorf("unsupported service action: %q", action)
			}
		})
	},
}

func serviceList(c *client.Client) error {
	var remotePeer peer.Peer

	reply, err := c.ServiceList(globalCtx, grpc.Peer(&remotePeer))
	if err != nil {
		if reply == nil {
			return fmt.Errorf("error listing services: %w", err)
		}

		helpers.Warning("%s", err)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "NODE\tSERVICE\tSTATE\tHEALTH\tLAST CHANGE\tLAST EVENT")

	defaultNode := addrFromPeer(&remotePeer)

	for _, resp := range reply.Response {
		for _, s := range resp.Services {
			svc := serviceInfoWrapper{s}

			node := defaultNode

			if resp.Metadata != nil {
				node = resp.Metadata.Hostname
			}

			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s ago\t%s\n", node, svc.Id, svc.State, svc.HealthStatus(), svc.LastUpdated(), svc.LastEvent())
		}
	}

	return w.Flush()
}

func serviceInfo(c *client.Client, id string) error {
	var remotePeer peer.Peer

	services, err := c.ServiceInfo(globalCtx, id, grpc.Peer(&remotePeer))
	if err != nil {
		if services == nil {
			return fmt.Errorf("error listing services: %w", err)
		}

		helpers.Warning("%s", err)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)

	defaultNode := addrFromPeer(&remotePeer)

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

			// nolint: errcheck
			ts, _ := ptypes.Timestamp(event.Ts)
			fmt.Fprintf(w, "%s\t[%s]: %s (%s ago)\n", label, event.State, event.Msg, time.Since(ts).Round(time.Second))
			label = ""
		}
	}

	if len(services) == 0 {
		return fmt.Errorf("service %q is not registered on any nodes", id)
	}

	return w.Flush()
}

func serviceStart(c *client.Client, id string) error {
	var remotePeer peer.Peer

	reply, err := c.ServiceStart(globalCtx, id, grpc.Peer(&remotePeer))
	if err != nil {
		if reply == nil {
			return fmt.Errorf("error starting service: %w", err)
		}

		helpers.Warning("%s", err)
	}

	defaultNode := addrFromPeer(&remotePeer)

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "NODE\tRESPONSE")

	for _, resp := range reply.Response {
		node := defaultNode

		if resp.Metadata != nil {
			node = resp.Metadata.Hostname
		}

		fmt.Fprintf(w, "%s\t%s\n", node, resp.Resp)
	}

	return w.Flush()
}

func serviceStop(c *client.Client, id string) error {
	var remotePeer peer.Peer

	reply, err := c.ServiceStop(globalCtx, id, grpc.Peer(&remotePeer))
	if err != nil {
		if reply == nil {
			return fmt.Errorf("error starting service: %w", err)
		}

		helpers.Warning("%s", err)
	}

	defaultNode := addrFromPeer(&remotePeer)

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "NODE\tRESPONSE")

	for _, resp := range reply.Response {
		node := defaultNode

		if resp.Metadata != nil {
			node = resp.Metadata.Hostname
		}

		fmt.Fprintf(w, "%s\t%s\n", node, resp.Resp)
	}

	return w.Flush()
}

func serviceRestart(c *client.Client, id string) error {
	var remotePeer peer.Peer

	reply, err := c.ServiceRestart(globalCtx, id, grpc.Peer(&remotePeer))
	if err != nil {
		if reply == nil {
			return fmt.Errorf("error starting service: %w", err)
		}

		helpers.Warning("%s", err)
	}

	defaultNode := addrFromPeer(&remotePeer)

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "NODE\tRESPONSE")

	for _, resp := range reply.Response {
		node := defaultNode

		if resp.Metadata != nil {
			node = resp.Metadata.Hostname
		}

		fmt.Fprintf(w, "%s\t%s\n", node, resp.Resp)
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

	// nolint: errcheck
	ts, _ := ptypes.Timestamp(svc.Events.Events[len(svc.Events.Events)-1].Ts)

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
	rootCmd.AddCommand(serviceCmd)
}
