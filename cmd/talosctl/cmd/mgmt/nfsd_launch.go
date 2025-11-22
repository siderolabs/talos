// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build linux || darwin

package mgmt

import (
	"net"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"

	"github.com/siderolabs/talos/pkg/provision/providers/vm"
)

var nfsdLaunchCmdFlags struct {
	addr    string
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
		var ips []net.IP

		for ip := range strings.SplitSeq(dnsdLaunchCmdFlags.addr, ",") {
			ips = append(ips, net.ParseIP(ip))
		}

		var eg errgroup.Group

		eg.Go(func() error {
			return vm.NFSd(ips, dnsdLaunchCmdFlags.resolvConf)
		})

		return eg.Wait()
	},
}

func init() {
	nfsdLaunchCmd.Flags().StringVar(&nfsdLaunchCmdFlags.addr, "addr", "localhost:2049", `Address to bind nfsd to`)
	nfsdLaunchCmd.Flags().StringVar(&nfsdLaunchCmdFlags.workdir, "workdir", "", `Working directory for nfsd`)
	addCommand(nfsdLaunchCmd)
}
