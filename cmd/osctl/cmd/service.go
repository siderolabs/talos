/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/golang/protobuf/ptypes"
	"github.com/spf13/cobra"

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
	Run: func(cmd *cobra.Command, args []string) {
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

		setupClient(func(c *client.Client) {
			switch action {
			case "status":
				if serviceID == "" {
					serviceList(c)
				} else {
					serviceInfo(c, serviceID)
				}
			case "start":
				serviceStart(c, serviceID)
			case "stop":
				serviceStop(c, serviceID)
			case "restart":
				serviceRestart(c, serviceID)
			default:
				helpers.Fatalf("unsupported service action: %q", action)
			}
		})
	},
}

func serviceList(c *client.Client) {
	reply, err := c.ServiceList(globalCtx)
	if err != nil {
		helpers.Fatalf("error listing services: %s", err)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "SERVICE\tSTATE\tHEALTH\tLAST CHANGE\tLAST EVENT")
	for _, s := range reply.Services {
		svc := serviceInfoWrapper{s}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s ago\t%s\n", svc.Id, svc.State, svc.HealthStatus(), svc.LastUpdated(), svc.LastEvent())
	}
	if err := w.Flush(); err != nil {
		helpers.Fatalf("error writing response: %s", err)
	}
}

func serviceInfo(c *client.Client, id string) {
	s, err := c.ServiceInfo(globalCtx, id)
	if err != nil {
		helpers.Fatalf("error listing services: %s", err)
	}
	if s == nil {
		helpers.Fatalf("service %q is not registered", id)
	}

	svc := serviceInfoWrapper{s}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintf(w, "ID\t%s\n", svc.Id)
	fmt.Fprintf(w, "STATE\t%s\n", svc.State)
	fmt.Fprintf(w, "HEALTH\t%s\n", svc.HealthStatus())
	if svc.Health.LastMessage != "" {
		fmt.Fprintf(w, "LAST HEALTH MESSAGE\t%s\n", svc.Health.LastMessage)
	}
	label := "EVENTS"
	for _, event := range svc.Events.Events {
		// nolint: errcheck
		ts, _ := ptypes.Timestamp(event.Ts)
		fmt.Fprintf(w, "%s\t[%s]: %s (%s ago)\n", label, event.State, event.Msg, time.Since(ts).Round(time.Second))
		label = ""
	}

	if err := w.Flush(); err != nil {
		helpers.Fatalf("error writing response: %s", err)
	}
}

func serviceStart(c *client.Client, id string) {
	resp, err := c.ServiceStart(globalCtx, id)
	if err != nil {
		helpers.Fatalf("error starting service: %s", err)
	}

	fmt.Fprintln(os.Stderr, resp)
}

func serviceStop(c *client.Client, id string) {
	resp, err := c.ServiceStop(globalCtx, id)
	if err != nil {
		helpers.Fatalf("error starting service: %s", err)
	}

	fmt.Fprintln(os.Stderr, resp)
}

func serviceRestart(c *client.Client, id string) {
	resp, err := c.ServiceRestart(globalCtx, id)
	if err != nil {
		helpers.Fatalf("error starting service: %s", err)
	}

	fmt.Fprintln(os.Stderr, resp)
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
