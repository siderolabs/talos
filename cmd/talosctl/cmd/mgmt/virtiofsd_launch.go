// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build linux || darwin

package mgmt

import (
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"

	"github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/cluster/create/flags"
	"github.com/siderolabs/talos/pkg/provision/providers/vm"
)

var virtiofsdLaunchCmdFlags struct {
	virtiofsdBin string
	virtiofs     flags.Virtiofs
}

// virtiofsdLaunchCmd represents the virtiofsd-launch command.
var virtiofsdLaunchCmd = &cobra.Command{
	Use:    "virtiofsd-launch",
	Short:  "Internal command used by VM provisioners",
	Long:   ``,
	Args:   cobra.NoArgs,
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		eg, ctx := errgroup.WithContext(cmd.Context())

		for _, vfs := range virtiofsdLaunchCmdFlags.virtiofs.Requests() {
			eg.Go(func() error {
				return vm.Virtiofsd(ctx, virtiofsdLaunchCmdFlags.virtiofsdBin, vfs.SharedDir, vfs.SocketPath)
			})
		}

		return eg.Wait()
	},
}

func init() {
	virtiofsdLaunchCmd.Flags().StringVar(&virtiofsdLaunchCmdFlags.virtiofsdBin, "bin",
		"/usr/libexec/virtiofsd", `path to the virtiofsd binary`)
	virtiofsdLaunchCmd.Flags().Var(&virtiofsdLaunchCmdFlags.virtiofs, "virtiofs",
		`list of virtiofs shares to create in format "<share>:<socket>"`)
	addCommand(virtiofsdLaunchCmd)
}
