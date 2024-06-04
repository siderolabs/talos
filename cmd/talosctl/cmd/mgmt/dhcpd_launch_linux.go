// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package mgmt

import (
	"net"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"

	"github.com/siderolabs/talos/pkg/provision/providers/vm"
)

var dhcpdLaunchCmdFlags struct {
	addr            string
	ifName          string
	statePath       string
	ipxeNextHandler string
}

// DhcpdLaunchCmd represents the dhcpd-launch command.
var DhcpdLaunchCmd = &cobra.Command{
	Use:    "dhcpd-launch",
	Short:  "Internal command used by VM provisioners",
	Long:   ``,
	Args:   cobra.NoArgs,
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		ips := []net.IP{}

		for _, ip := range strings.Split(dhcpdLaunchCmdFlags.addr, ",") {
			ips = append(ips, net.ParseIP(ip))
		}

		var eg errgroup.Group

		eg.Go(func() error {
			return vm.DHCPd(dhcpdLaunchCmdFlags.ifName, ips, dhcpdLaunchCmdFlags.statePath)
		})

		if dhcpdLaunchCmdFlags.ipxeNextHandler != "" {
			eg.Go(func() error {
				return vm.TFTPd(ips, dhcpdLaunchCmdFlags.ipxeNextHandler)
			})
		}

		return eg.Wait()
	},
}

func init() {
	DhcpdLaunchCmd.Flags().StringVar(&dhcpdLaunchCmdFlags.addr, "addr", "localhost", "IP addresses to listen on")
	DhcpdLaunchCmd.Flags().StringVar(&dhcpdLaunchCmdFlags.ifName, "interface", "", "interface to listen on")
	DhcpdLaunchCmd.Flags().StringVar(&dhcpdLaunchCmdFlags.statePath, "state-path", "", "path to state directory")
	DhcpdLaunchCmd.Flags().StringVar(&dhcpdLaunchCmdFlags.ipxeNextHandler, "ipxe-next-handler", "", "iPXE script to chain load")
	addCommand(DhcpdLaunchCmd)
}
