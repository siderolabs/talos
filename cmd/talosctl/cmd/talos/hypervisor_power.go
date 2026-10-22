// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
)

// hypervisorStartCmd represents the hypervisor start command.
var hypervisorStartCmd = &cobra.Command{
	Use:   "start <vm-name>",
	Short: "Start a virtual machine",
	Long:  `Overrides the power state of the virtual machine to running until the node reboots. The machine configuration is not affected.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return withHypervisorNode(cmd.Context(), "talosctl hypervisor start", func(ctx context.Context, c *client.Client) error {
			_, err := c.HypervisorClient.Start(ctx, &machine.VirtualMachineStartRequest{Name: args[0]})

			return err
		})
	},
}

var hypervisorStopCmdFlags struct {
	graceful bool
}

// hypervisorStopCmd represents the hypervisor stop command.
var hypervisorStopCmd = &cobra.Command{
	Use:   "stop <vm-name>",
	Short: "Stop a virtual machine",
	Long:  `Overrides the power state of the virtual machine to stopped until the node reboots. The machine configuration is not affected.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return withHypervisorNode(cmd.Context(), "talosctl hypervisor stop", func(ctx context.Context, c *client.Client) error {
			_, err := c.HypervisorClient.Stop(ctx, &machine.VirtualMachineStopRequest{
				Name:  args[0],
				Force: !hypervisorStopCmdFlags.graceful,
			})

			return err
		})
	},
}

// withHypervisorNode runs f against exactly one node.
//
// Virtual machines are host-local, and the hypervisor API is not proxied to many nodes at once, so
// a command which names several of them is refused rather than silently applied to one.
func withHypervisorNode(ctx context.Context, command string, f func(context.Context, *client.Client) error) (result error) {
	factory, err := NewClientFactory(ctx, nil)
	if err != nil {
		return fmt.Errorf("create hypervisor client: %w", err)
	}

	defer func() { result = errors.Join(result, factory.Close()) }()

	ctx, c, _, err := factory.BuildClientEnforceSingleNode(ctx, command)
	if err != nil {
		return err
	}

	return f(ctx, c)
}

func init() {
	hypervisorStopCmd.Flags().BoolVar(&hypervisorStopCmdFlags.graceful, "graceful", true,
		"ask the guest to power itself off; --graceful=false destroys the domain instead")

	hypervisorCmd.AddCommand(hypervisorStartCmd)
	hypervisorCmd.AddCommand(hypervisorStopCmd)
}
