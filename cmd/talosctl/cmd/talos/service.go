// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"context"
	"errors"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/siderolabs/gen/xslices"
	"github.com/spf13/cobra"

	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/global"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/client/multiplex"
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
			return getServiceFromNode(cmd.Context(), nil), cobra.ShellCompDirectiveNoFileComp
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

		ctx := cmd.Context()

		clientFactory, err := NewClientFactory(ctx, nil)
		if err != nil {
			return err
		}

		defer clientFactory.Close() //nolint:errcheck

		switch action {
		case "status":
			if serviceID == "" {
				return serviceList(ctx, clientFactory)
			}

			return serviceInfo(ctx, clientFactory, serviceID)
		case "start":
			return serviceStart(ctx, clientFactory, serviceID)
		case "stop":
			return serviceStop(ctx, clientFactory, serviceID)
		case "restart":
			return serviceRestart(ctx, clientFactory, serviceID)
		default:
			return fmt.Errorf("unsupported service action: %q", action)
		}
	},
}

func serviceList(ctx context.Context, clientFactory *global.ClientFactory) error {
	responseChan := multiplex.UnaryViaFactory(
		ctx, clientFactory,
		func(ctx context.Context, c *client.Client) (*machineapi.ServiceListResponse, error) {
			return c.ServiceList(ctx)
		},
	)

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "NODE\tSERVICE\tSTATE\tHEALTH\tLAST CHANGE\tLAST EVENT")

	var errs error

	for resp := range responseChan {
		if resp.Err != nil {
			errs = errors.Join(errs, fmt.Errorf("error from node %s: %w", resp.Node, resp.Err))

			continue
		}

		for _, msg := range resp.Payload.Messages {
			for _, s := range msg.Services {
				svc := serviceInfoWrapper{s}

				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s ago\t%s\n", resp.Node, svc.Id, svc.State, svc.healthStatus(), svc.lastUpdated(), svc.lastEvent())
			}
		}
	}

	return errors.Join(errs, w.Flush())
}

func serviceInfo(ctx context.Context, clientFactory *global.ClientFactory, id string) error {
	responseChan := multiplex.UnaryViaFactory(
		ctx, clientFactory,
		func(ctx context.Context, c *client.Client) ([]client.ServiceInfo, error) {
			return c.ServiceInfo(ctx, id)
		},
	)

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)

	var (
		errs  error
		found bool
	)

	for resp := range responseChan {
		if resp.Err != nil {
			errs = errors.Join(errs, fmt.Errorf("error from node %s: %w", resp.Node, resp.Err))

			continue
		}

		for _, s := range resp.Payload {
			found = true

			renderServiceInfo(w, resp.Node, s.Service)
		}
	}

	if err := w.Flush(); err != nil {
		errs = errors.Join(errs, err)
	}

	if !found && errs == nil {
		return fmt.Errorf("service %q is not registered on any nodes", id)
	}

	return errs
}

// renderServiceInfo writes detailed human readable service information for a single node.
func renderServiceInfo(w *tabwriter.Writer, node string, s *machineapi.ServiceInfo) {
	svc := serviceInfoWrapper{s}

	fmt.Fprintf(w, "NODE\t%s\n", node)
	fmt.Fprintf(w, "ID\t%s\n", svc.Id)
	fmt.Fprintf(w, "STATE\t%s\n", svc.State)
	fmt.Fprintf(w, "HEALTH\t%s\n", svc.healthStatus())

	if svc.Health.LastMessage != "" {
		fmt.Fprintf(w, "LAST HEALTH MESSAGE\t%s\n", svc.Health.LastMessage)
	}

	label := "EVENTS"

	for i := range svc.Events.Events {
		event := svc.Events.Events[len(svc.Events.Events)-1-i]

		ts := event.Ts.AsTime()
		fmt.Fprintf(w, "%s\t[%s]: %s (%s ago)\n", label, event.State, event.Msg, time.Since(ts).Round(time.Second))
		label = ""
	}
}

func serviceStart(ctx context.Context, clientFactory *global.ClientFactory, id string) error {
	return serviceActionRun(
		ctx, clientFactory,
		func(ctx context.Context, c *client.Client) (*machineapi.ServiceStartResponse, error) {
			return c.ServiceStart(ctx, id)
		},
		func(resp *machineapi.ServiceStartResponse) []string {
			return xslices.Map(resp.Messages, (*machineapi.ServiceStart).GetResp)
		},
	)
}

func serviceStop(ctx context.Context, clientFactory *global.ClientFactory, id string) error {
	return serviceActionRun(
		ctx, clientFactory,
		func(ctx context.Context, c *client.Client) (*machineapi.ServiceStopResponse, error) {
			return c.ServiceStop(ctx, id)
		},
		func(resp *machineapi.ServiceStopResponse) []string {
			return xslices.Map(resp.Messages, (*machineapi.ServiceStop).GetResp)
		},
	)
}

func serviceRestart(ctx context.Context, clientFactory *global.ClientFactory, id string) error {
	return serviceActionRun(
		ctx, clientFactory,
		func(ctx context.Context, c *client.Client) (*machineapi.ServiceRestartResponse, error) {
			return c.ServiceRestart(ctx, id)
		},
		func(resp *machineapi.ServiceRestartResponse) []string {
			return xslices.Map(resp.Messages, (*machineapi.ServiceRestart).GetResp)
		},
	)
}

// serviceActionRun runs a service control action across all nodes and renders the per-node responses.
func serviceActionRun[RespT any](
	ctx context.Context,
	clientFactory *global.ClientFactory,
	call func(context.Context, *client.Client) (RespT, error),
	responses func(RespT) []string,
) error {
	responseChan := multiplex.UnaryViaFactory(ctx, clientFactory, call)

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "NODE\tRESPONSE")

	var errs error

	for resp := range responseChan {
		if resp.Err != nil {
			errs = errors.Join(errs, fmt.Errorf("error from node %s: %w", resp.Node, resp.Err))

			continue
		}

		for _, r := range responses(resp.Payload) {
			fmt.Fprintf(w, "%s\t%s\n", resp.Node, r)
		}
	}

	return errors.Join(errs, w.Flush())
}

// serviceInfoWrapper helper that allows generating rich service information.
type serviceInfoWrapper struct {
	*machineapi.ServiceInfo
}

// lastUpdated derives last updated time from the events stream.
func (svc serviceInfoWrapper) lastUpdated() string {
	if len(svc.Events.Events) == 0 {
		return ""
	}

	ts := svc.Events.Events[len(svc.Events.Events)-1].Ts.AsTime()

	return time.Since(ts).Round(time.Second).String()
}

// lastEvent returns the last service event.
func (svc serviceInfoWrapper) lastEvent() string {
	if len(svc.Events.Events) == 0 {
		return "<none>"
	}

	return svc.Events.Events[len(svc.Events.Events)-1].Msg
}

// healthStatus returns the service health status.
func (svc serviceInfoWrapper) healthStatus() string {
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
