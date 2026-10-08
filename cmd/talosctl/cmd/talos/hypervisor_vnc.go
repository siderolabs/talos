// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/safeout"
	"github.com/siderolabs/talos/internal/pkg/vmvnc"
)

var hypervisorVNCPort int

var hypervisorVNCCmd = &cobra.Command{
	Use:   "vnc <vm-name>",
	Short: "Expose a virtual machine's VNC console on a local TCP port",
	Long: "Expose a VM's VNC console on IPv4 loopback. Requires Admin access and exactly one target node. " +
		"Connect your VNC viewer to the printed TCP endpoint (not a VNC display number). " +
		"Local processes that can connect to this port gain access to the guest console. " +
		"Only one viewer may connect at a time. The listener stays open for reconnects until Ctrl-C; no viewer is launched automatically.",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) (result error) {
		ctx := cmd.Context()

		factory, err := NewClientFactory(ctx, nil)
		if err != nil {
			return fmt.Errorf("create VNC client: %w", err)
		}

		defer func() { result = errors.Join(result, factory.Close()) }()

		ctx, c, _, err := factory.BuildClientEnforceSingleNode(ctx, "talosctl hypervisor vnc")
		if err != nil {
			return err
		}

		listener, err := vmvnc.Listen(ctx, hypervisorVNCPort)
		if err != nil {
			return err
		}

		safeout.Fprintf(safeout.Stderr(), "VNC listener ready at TCP %s (VM not yet attached). Connect a viewer; press Ctrl-C to stop.\n", listener.Addr()) //nolint:errcheck
		safeout.Flush()                                                                                                                                     //nolint:errcheck

		return vmvnc.Serve(ctx, c.HypervisorClient, args[0], listener, func(err error) {
			safeout.Fprintf(safeout.Stderr(), "%s\n", err) //nolint:errcheck
			safeout.Flush()                                //nolint:errcheck
		})
	},
}

func init() {
	hypervisorVNCCmd.Flags().IntVar(&hypervisorVNCPort, "port", 0, "Local loopback TCP port (0 selects an available port)")
	hypervisorCmd.AddCommand(hypervisorVNCCmd)
}
