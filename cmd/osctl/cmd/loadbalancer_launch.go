// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/talos-systems/talos/internal/pkg/loadbalancer"
	"github.com/talos-systems/talos/pkg/constants"
)

var loadbalancerLaunchCmdFlags struct {
	addr             string
	upstreams        []string
	apidOnlyInitNode bool
}

// loadbalancerLaunchCmd represents the loadbalancer-launch command
var loadbalancerLaunchCmd = &cobra.Command{
	Use:    "loadbalancer-launch",
	Short:  "Intneral command used by Firecracker provisioner",
	Long:   ``,
	Args:   cobra.NoArgs,
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		var lb loadbalancer.TCP

		for _, port := range []int{constants.ApidPort, 6443} { // TODO: need to put 6443 as constant or use config?
			upstreams := make([]string, len(loadbalancerLaunchCmdFlags.upstreams))
			for i := range upstreams {
				upstreams[i] = fmt.Sprintf("%s:%d", loadbalancerLaunchCmdFlags.upstreams[i], port)
			}

			if loadbalancerLaunchCmdFlags.apidOnlyInitNode {
				// for apid, add only init node for now (first item)
				if port == constants.ApidPort && len(upstreams) > 1 {
					upstreams = upstreams[:1]
				}
			}

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
	loadbalancerLaunchCmd.Flags().BoolVar(&loadbalancerLaunchCmdFlags.apidOnlyInitNode, "apid-only-init-node", false, "use only apid init node for load balancing")
	rootCmd.AddCommand(loadbalancerLaunchCmd)
}
