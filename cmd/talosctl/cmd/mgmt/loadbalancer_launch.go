// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package mgmt

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/talos-systems/go-loadbalancer/loadbalancer"

	"github.com/talos-systems/talos/pkg/machinery/constants"
	"github.com/talos-systems/talos/pkg/machinery/generic/slices"
)

var loadbalancerLaunchCmdFlags struct {
	addr             string
	upstreams        []string
	apidOnlyInitNode bool
}

// loadbalancerLaunchCmd represents the loadbalancer-launch command.
var loadbalancerLaunchCmd = &cobra.Command{
	Use:    "loadbalancer-launch",
	Short:  "Internal command used by QEMU provisioner",
	Long:   ``,
	Args:   cobra.NoArgs,
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		var lb loadbalancer.TCP

		for _, port := range []int{constants.DefaultControlPlanePort} {
			upstreams := slices.Map(loadbalancerLaunchCmdFlags.upstreams, func(upstream string) string {
				return fmt.Sprintf("%s:%d", upstream, port)
			})

			if err := lb.AddRoute(fmt.Sprintf("%s:%d", loadbalancerLaunchCmdFlags.addr, port), upstreams); err != nil {
				return err
			}
		}

		return lb.Run()
	},
}

func init() {
	loadbalancerLaunchCmd.Flags().StringVar(&loadbalancerLaunchCmdFlags.addr, "loadbalancer-addr", "localhost", "load balancer listen address (IP or host)")
	loadbalancerLaunchCmd.Flags().StringSliceVar(&loadbalancerLaunchCmdFlags.upstreams, "loadbalancer-upstreams", []string{}, "load balancer upstreams (nodes to proxy to)")
	addCommand(loadbalancerLaunchCmd)
}
