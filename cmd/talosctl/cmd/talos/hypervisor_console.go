// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build !windows

package talos

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/siderolabs/talos/internal/pkg/vmconsole"
)

var hypervisorConsoleCmd = &cobra.Command{
	Use:   "console <vm-name>",
	Short: "Attach to a virtual machine's live serial console",
	Long: "Attach to a VM's live serial console from a Unix client (Windows is not supported). " +
		"Requires Admin access and exactly one target node. " +
		"Press Ctrl-] to detach without stopping the VM. Ctrl-C is forwarded to the guest. " +
		"This does not replay previous console output.",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) (result error) {
		ctx := cmd.Context()

		factory, err := NewClientFactory(ctx, nil)
		if err != nil {
			return fmt.Errorf("create console client: %w", err)
		}

		defer func() { result = errors.Join(result, factory.Close()) }()

		ctx, c, _, err := factory.BuildClientEnforceSingleNode(ctx, "talosctl hypervisor console")
		if err != nil {
			return err
		}

		// Console output is an interactive raw stream, not escaped tabular output.
		return vmconsole.Run(ctx, c.HypervisorClient, args[0], os.Stdin, os.Stdout) //nolint:forbidigo
	},
}

func init() {
	hypervisorCmd.AddCommand(hypervisorConsoleCmd)
}
