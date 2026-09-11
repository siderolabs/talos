// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package mgmt

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
)

func init() {
	var bridge string

	cmd := &cobra.Command{
		Use:    "lldp-launch",
		Short:  "Launch the QEMU host LLDP receive-test advertiser",
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			return runLLDPAdvertiser(ctx, bridge)
		},
	}
	cmd.Flags().StringVar(&bridge, "bridge", "", "QEMU management bridge")

	if err := cmd.MarkFlagRequired("bridge"); err != nil {
		panic(err)
	}

	addCommand(cmd)
}
