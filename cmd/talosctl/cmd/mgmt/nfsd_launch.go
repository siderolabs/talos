// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build linux || darwin

package mgmt

import (
	"net"

	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"

	"github.com/siderolabs/talos/pkg/provision/providers/vm"
)

var nfsdLaunchCmdFlags struct {
	addr    []net.IP
	workdir string
}

// nfsdLaunchCmd represents the nfsd-launch command.
var nfsdLaunchCmd = &cobra.Command{
	Use:    "nfsd-launch",
	Short:  "Internal command used by VM provisioners",
	Long:   ``,
	Args:   cobra.NoArgs,
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		var eg errgroup.Group

		eg.Go(func() error {
			return vm.NFSd(nfsdLaunchCmdFlags.addr, nfsdLaunchCmdFlags.workdir)
		})

		return eg.Wait()
	},
}

func init() {
	nfsdLaunchCmd.Flags().IPSliceVar(&nfsdLaunchCmdFlags.addr, "addr", []net.IP{net.ParseIP("127.0.0.1")}, `Address to bind nfsd to`)
	nfsdLaunchCmd.Flags().StringVar(&nfsdLaunchCmdFlags.workdir, "workdir", "", `Working directory for nfsd`)
	addCommand(nfsdLaunchCmd)
}
