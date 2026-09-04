// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package mgmt

import (
	"github.com/spf13/cobra"

	"github.com/siderolabs/talos/pkg/provision/providers/vm"
)

var nfsLaunchCmdFlags struct {
	bindAddress string
	port        int
}

var nfsLaunchCmd = &cobra.Command{
	Use:    "nfs-launch",
	Short:  "Internal command used by the QEMU provisioner",
	Args:   cobra.NoArgs,
	Hidden: true,
	RunE: func(cmd *cobra.Command, _ []string) error {
		return vm.NFSd(cmd.Context(), nfsLaunchCmdFlags.bindAddress, nfsLaunchCmdFlags.port)
	},
}

func init() {
	nfsLaunchCmd.Flags().StringVar(&nfsLaunchCmdFlags.bindAddress, "bind-address", "127.0.0.1", "NFS listen address")
	nfsLaunchCmd.Flags().IntVar(&nfsLaunchCmdFlags.port, "port", 2049, "NFS listen port")
	addCommand(nfsLaunchCmd)
}
